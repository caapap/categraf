package spark_streaming

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"flashcat.cloud/categraf/config"
	"flashcat.cloud/categraf/types"
)

type Instance struct {
	config.InstanceConfig

	// YARN Configuration
	YARNAddress    string   `toml:"yarn_address"`
	YARNAppAPIPath string   `toml:"yarn_app_api_path"`
	AppPrefixes    []string `toml:"app_prefixes"`

	// HTTP Configuration
	Timeout        config.Duration `toml:"timeout"`
	MaxConcurrency int             `toml:"max_concurrency"` // Max concurrent app scraping
	SkipVerify     bool            `toml:"skip_verify"`     // Skip TLS verification

	// Kerberos Configuration (for future implementation)
	KerberosEnabled bool   `toml:"kerberos_enabled"`
	Principal       string `toml:"kerberos_principal"`
	KeytabPath      string `toml:"kerberos_keytab_path"`

	// Internal state
	client      *http.Client
	ctx         context.Context
	cancel      context.CancelFunc
	initialized bool
	timeout     time.Duration
	mu          sync.RWMutex
}

func (ins *Instance) Init() error {
	// Validate required configuration
	if ins.YARNAddress == "" {
		return fmt.Errorf("yarn_address is required")
	}

	// Set defaults
	if ins.YARNAppAPIPath == "" {
		ins.YARNAppAPIPath = "/ws/v1/cluster/apps"
	}

	if ins.Timeout <= 0 {
		ins.Timeout = config.Duration(time.Second * 10)
	}
	ins.timeout = time.Duration(ins.Timeout)

	if ins.MaxConcurrency <= 0 {
		ins.MaxConcurrency = 10 // Default: 10 concurrent app scraping
	}

	// Create context
	ins.ctx, ins.cancel = context.WithCancel(context.Background())

	// Create HTTP client
	ins.client = &http.Client{
		Timeout: ins.timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Verify YARN connection
	if err := ins.verifyYARNConnection(); err != nil {
		log.Printf("W! [spark_streaming] Failed to connect to YARN at %s: %v", ins.YARNAddress, err)
		// Don't return error, allow delayed connection
	}

	// Handle Kerberos (future implementation)
	if ins.KerberosEnabled {
		log.Printf("W! [spark_streaming] Kerberos authentication is not yet implemented")
		// TODO: Implement Kerberos authentication
	}

	ins.initialized = true
	log.Printf("I! [spark_streaming] Instance initialized successfully for YARN: %s", ins.YARNAddress)
	return nil
}

func (ins *Instance) Gather(slist *types.SampleList) {
	if !ins.initialized {
		log.Printf("W! [spark_streaming] Instance not initialized")
		return
	}

	begun := time.Now()

	// 1. Get running apps from YARN
	apps, err := ins.getRunningApps()
	if err != nil {
		log.Printf("E! [spark_streaming] Failed to get running apps: %v", err)
		ins.pushConnectionStatus(slist, 0)
		return
	}

	// Connection is successful
	ins.pushConnectionStatus(slist, 1)

	// 2. Filter apps by prefixes
	filteredApps := ins.filterAppsByPrefix(apps)

	if len(filteredApps) == 0 {
		log.Printf("I! [spark_streaming] No matching Spark Streaming applications found")
		return
	}

	log.Printf("I! [spark_streaming] Collecting metrics from %d applications", len(filteredApps))

	// 3. Collect metrics from each app concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, ins.MaxConcurrency)

	for _, app := range filteredApps {
		wg.Add(1)
		go func(app *YARNApp) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			ins.gatherAppMetrics(slist, app)
		}(app)
	}

	wg.Wait()

	elapsed := time.Since(begun)
	log.Printf("I! [spark_streaming] Collected metrics from %d apps in %s", len(filteredApps), elapsed)
}

func (ins *Instance) gatherAppMetrics(slist *types.SampleList, app *YARNApp) {
	// Base labels for this application
	baseLabels := ins.makeLabels(map[string]string{
		"app_id":       app.ID,
		"app_name":     app.Name,
		"state":        app.State,
		"final_status": app.FinalStatus,
		"user":         app.User,
		"queue":        app.Queue,
	})

	// Push application status metric
	slist.PushSample(inputName, "yarn_run_app", 1, baseLabels)

	// Get Spark Streaming metrics
	metrics, err := ins.getSparkStreamingMetrics(app.TrackingURL)
	if err != nil {
		log.Printf("E! [spark_streaming] Failed to get metrics for app %s (%s): %v", app.Name, app.ID, err)
		// Push error metric
		errorLabels := ins.makeLabels(map[string]string{
			"app_id":   app.ID,
			"app_name": app.Name,
		})
		slist.PushSample(inputName, "scrape_error", 1, errorLabels)
		return
	}

	// Metric labels (simpler version without state/finalStatus)
	metricLabels := ins.makeLabels(map[string]string{
		"app_id":   app.ID,
		"app_name": app.Name,
	})

	// Push Spark Streaming metrics
	if metrics.InputRateAvg > 0 {
		slist.PushSample(inputName, "input_rate_records_avg_stat", metrics.InputRateAvg, metricLabels)
	}

	if metrics.SchedulingDelayAvg > 0 {
		slist.PushSample(inputName, "batches_scheduling_delay_avg_stat", metrics.SchedulingDelayAvg, metricLabels)
	}

	if metrics.ProcessingTimeAvg > 0 {
		slist.PushSample(inputName, "batches_processing_time_avg_stat", metrics.ProcessingTimeAvg, metricLabels)
	}

	if metrics.TotalDelayAvg > 0 {
		slist.PushSample(inputName, "batches_total_delay_avg_stat", metrics.TotalDelayAvg, metricLabels)
	}

	// Push batch counts (always push, even if zero)
	slist.PushSample(inputName, "running_batches_current_num", metrics.RunningBatches, metricLabels)
	slist.PushSample(inputName, "waiting_batches_current_num", metrics.WaitingBatches, metricLabels)
	slist.PushSample(inputName, "completed_batches_current_num", metrics.CompletedBatches, metricLabels)

	log.Printf("D! [spark_streaming] Collected metrics for app %s: InputRate=%.2f, SchedulingDelay=%.2f, ProcessingTime=%.2f",
		app.Name, metrics.InputRateAvg, metrics.SchedulingDelayAvg, metrics.ProcessingTimeAvg)
}

func (ins *Instance) pushConnectionStatus(slist *types.SampleList, status float64) {
	labels := ins.makeLabels(map[string]string{
		"yarn_address": ins.YARNAddress,
	})
	slist.PushSample(inputName, "yarn_up", status, labels)
}

func (ins *Instance) makeLabels(extra map[string]string) map[string]string {
	labels := make(map[string]string)

	// Copy instance labels
	for k, v := range ins.Labels {
		labels[k] = v
	}

	// Add extra labels
	for k, v := range extra {
		labels[k] = v
	}

	return labels
}

func (ins *Instance) Drop() {
	if ins.cancel != nil {
		ins.cancel()
	}

	ins.initialized = false
	log.Printf("I! [spark_streaming] Instance dropped for YARN: %s", ins.YARNAddress)
}
