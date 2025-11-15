package npu

import (
	"time"

	"flashcat.cloud/categraf/config"
	"flashcat.cloud/categraf/inputs"
	"flashcat.cloud/categraf/types"
	"github.com/sirupsen/logrus"
)

const inputName = "npu"

func init() {
	inputs.Add(inputName, func() inputs.Input {
		return &NPU{}
	})
}

// NPU implements the Categraf Input interface for Huawei Ascend NPU monitoring
type NPU struct {
	config.PluginConfig

	// Configuration options
	Enabled        bool            `toml:"enabled"`         // Enable NPU monitoring
	UpdateInterval config.Duration `toml:"update_interval"` // Metric update interval
	CacheTime      config.Duration `toml:"cache_time"`      // Cache expiration time

	// Internal state
	collector   *NpuCollector
	initialized bool
}

// Clone creates a copy of the NPU input
func (n *NPU) Clone() inputs.Input {
	return &NPU{}
}

// Name returns the plugin name
func (n *NPU) Name() string {
	return inputName
}

// Init initializes the NPU input plugin
func (n *NPU) Init() error {
	// Check if enabled
	if !n.Enabled {
		logrus.Info("NPU monitoring is disabled (enabled=false)")
		return nil
	}

	// Set default values
	if n.UpdateInterval == 0 {
		n.UpdateInterval = config.Duration(5 * time.Second)
	}
	if n.CacheTime == 0 {
		n.CacheTime = config.Duration(65 * time.Second)
	}

	// Validate intervals
	if n.UpdateInterval < config.Duration(1*time.Second) {
		logrus.Warn("update_interval too small, using 1s minimum")
		n.UpdateInterval = config.Duration(1 * time.Second)
	}
	if n.UpdateInterval > config.Duration(60*time.Second) {
		logrus.Warn("update_interval too large, using 60s maximum")
		n.UpdateInterval = config.Duration(60 * time.Second)
	}

	// Create collector
	collector, err := NewNpuCollector(time.Duration(n.CacheTime), time.Duration(n.UpdateInterval))
	if err != nil {
		logrus.Errorf("failed to create NPU collector: %v", err)
		logrus.Warn("NPU monitoring will be disabled due to initialization failure")
		// Don't return error - allow Categraf to continue without NPU monitoring
		n.Enabled = false
		return nil
	}

	n.collector = collector

	// Start background collection
	if err := n.collector.Start(); err != nil {
		logrus.Errorf("failed to start NPU collector: %v", err)
		logrus.Warn("NPU monitoring will be disabled due to start failure")
		n.Enabled = false
		return nil
	}

	n.initialized = true
	logrus.Infof("NPU monitoring initialized successfully (update_interval=%s, cache_time=%s)",
		n.UpdateInterval, n.CacheTime)

	return nil
}

// Gather collects NPU metrics
func (n *NPU) Gather(slist *types.SampleList) {
	// Check if enabled and initialized
	if !n.Enabled {
		return
	}

	if !n.initialized || n.collector == nil {
		logrus.Debug("NPU collector not initialized, skipping gather")
		return
	}

	// Record scrape start time
	begun := time.Now()
	defer func() {
		elapsed := time.Since(begun).Seconds()
		slist.PushFront(types.NewSample(inputName, "scrape_duration_seconds", elapsed))
	}()

	// Get chip list
	chips := n.collector.GetChips()
	if len(chips) == 0 {
		logrus.Debug("no NPU chips found")
		slist.PushFront(types.NewSample(inputName, "up", 0))
		return
	}

	// Mark scraper as up
	slist.PushFront(types.NewSample(inputName, "up", 1))

	// Convert chip list to summary metrics
	ConvertChipListToSample(chips, slist)

	// Collect metrics for each chip
	successCount := 0
	for _, chip := range chips {
		metrics, exists := n.collector.GetMetrics(chip.PhyId)
		if !exists {
			logrus.Debugf("no metrics found for chip %d (phy_id=%d)", chip.LogicID, chip.PhyId)
			continue
		}

		// Convert metrics to Categraf samples
		ConvertMetricsToSamples(metrics, slist)
		successCount++
	}

	logrus.Debugf("collected metrics for %d/%d NPU chips", successCount, len(chips))
}

// Drop performs cleanup when the input is removed
func (n *NPU) Drop() {
	if n.collector != nil {
		logrus.Info("stopping NPU collector")
		n.collector.Stop()
	}
}
