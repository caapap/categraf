package spark_streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// getRunningApps fetches running applications from YARN ResourceManager
func (ins *Instance) getRunningApps() ([]*YARNApp, error) {
	url := fmt.Sprintf("http://%s%s", ins.YARNAddress, ins.YARNAppAPIPath)

	ctx, cancel := context.WithTimeout(ins.ctx, ins.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := ins.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request YARN API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var result YARNAppsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Filter only RUNNING applications
	var runningApps []*YARNApp
	for i := range result.Apps.App {
		if result.Apps.App[i].State == "RUNNING" {
			runningApps = append(runningApps, &result.Apps.App[i])
		}
	}

	log.Printf("D! [spark_streaming] Found %d running applications from YARN", len(runningApps))
	return runningApps, nil
}

// filterAppsByPrefix filters applications by configured prefixes
func (ins *Instance) filterAppsByPrefix(apps []*YARNApp) []*YARNApp {
	if len(ins.AppPrefixes) == 0 {
		log.Printf("D! [spark_streaming] No app_prefixes configured, monitoring all applications")
		return apps // No filter, return all
	}

	var filtered []*YARNApp
	for _, app := range apps {
		for _, prefix := range ins.AppPrefixes {
			if strings.HasPrefix(app.Name, prefix) {
				filtered = append(filtered, app)
				log.Printf("D! [spark_streaming] Matched app: %s (ID: %s) with prefix: %s",
					app.Name, app.ID, prefix)
				break
			}
		}
	}

	log.Printf("D! [spark_streaming] Filtered %d/%d applications by prefixes", len(filtered), len(apps))
	return filtered
}

// verifyYARNConnection verifies connection to YARN ResourceManager
func (ins *Instance) verifyYARNConnection() error {
	url := fmt.Sprintf("http://%s/ws/v1/cluster/info", ins.YARNAddress)

	ctx, cancel := context.WithTimeout(context.Background(), ins.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := ins.client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	log.Printf("I! [spark_streaming] Successfully connected to YARN at %s", ins.YARNAddress)
	return nil
}
