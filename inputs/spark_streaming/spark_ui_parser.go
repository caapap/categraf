package spark_streaming

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	// Regex patterns for parsing batch counts
	batchRunningPattern   = regexp.MustCompile(`\((\d+)\)`)        // Running/Waiting: (3)
	batchCompletedPattern = regexp.MustCompile(`(\d+)\s+out\s+of`) // Completed: 5 out of 10
)

// getSparkStreamingMetrics fetches and parses Spark Streaming metrics from Web UI
func (ins *Instance) getSparkStreamingMetrics(trackingURL string) (*SparkStreamingMetrics, error) {
	// Ensure trackingURL ends with /streaming/
	streamingURL := strings.TrimSuffix(trackingURL, "/") + "/streaming/"

	ctx, cancel := context.WithTimeout(ins.ctx, ins.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", streamingURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := ins.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch streaming page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse HTML using goquery
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	metrics := NewSparkStreamingMetrics()

	// Parse Streaming Statistics
	ins.parseStreamingStatistics(doc, metrics)

	// Parse Batch Counts
	ins.parseBatchCounts(doc, metrics)

	return metrics, nil
}

// parseStreamingStatistics parses the streaming statistics table
func (ins *Instance) parseStreamingStatistics(doc *goquery.Document, metrics *SparkStreamingMetrics) {
	doc.Find("#stat-table td[style='vertical-align: middle;'] div[style='width: 160px;']").Each(func(i int, s *goquery.Selection) {
		text := s.Text()

		// Parse Input Rate
		if strings.Contains(text, "Input Rate") {
			metrics.InputRateAvg = ins.parseInputRate(text)
		}

		// Parse Scheduling Delay
		if strings.Contains(text, "Scheduling Delay") {
			metrics.SchedulingDelayAvg = ins.parseTimeValue(text, "Avg:")
		}

		// Parse Processing Time
		if strings.Contains(text, "Processing Time") {
			metrics.ProcessingTimeAvg = ins.parseTimeValue(text, "Avg:")
		}

		// Parse Total Delay
		if strings.Contains(text, "Total Delay") {
			metrics.TotalDelayAvg = ins.parseTimeValue(text, "Avg:")
		}
	})
}

// parseBatchCounts parses the batch counts (running, waiting, completed)
func (ins *Instance) parseBatchCounts(doc *goquery.Document, metrics *SparkStreamingMetrics) {
	// Parse Running Batches
	runningText := doc.Find("#runningBatches h4 a").Text()
	metrics.RunningBatches = ins.parseBatchCount(runningText, batchRunningPattern)

	// Parse Waiting Batches
	waitingText := doc.Find("#waitingBatches h4 a").Text()
	metrics.WaitingBatches = ins.parseBatchCount(waitingText, batchRunningPattern)

	// Parse Completed Batches
	completedText := doc.Find("#completedBatches h4 a").Text()
	metrics.CompletedBatches = ins.parseBatchCount(completedText, batchCompletedPattern)
}

// parseInputRate parses input rate from text like "Input Rate 123.45 records/sec"
func (ins *Instance) parseInputRate(text string) float64 {
	// Extract the average value
	// Format: "Input Rate 0.0 records/sec (avg: 123.45)"
	if idx := strings.Index(text, "(avg:"); idx > 0 {
		avgStr := text[idx+5:]
		avgStr = strings.TrimSpace(strings.TrimSuffix(avgStr, ")"))
		avgStr = strings.Split(avgStr, " ")[0]
		if val, err := strconv.ParseFloat(avgStr, 64); err == nil {
			return val
		}
	}

	// Fallback: try to parse the first number
	parts := strings.Fields(text)
	for i, part := range parts {
		if i > 0 && i < len(parts)-1 { // Skip first and last parts
			if val, err := strconv.ParseFloat(part, 64); err == nil {
				return val
			}
		}
	}

	return 0
}

// parseTimeValue parses time values like "Avg: 123 ms" or "Avg: 1 min 30 s"
func (ins *Instance) parseTimeValue(text string, prefix string) float64 {
	// Find the "Avg:" part
	idx := strings.Index(text, prefix)
	if idx < 0 {
		return 0
	}

	// Extract the time string after "Avg:"
	timeStr := text[idx+len(prefix):]
	timeStr = strings.TrimSpace(timeStr)

	// Convert to milliseconds
	return ins.convertTimeToMS(timeStr)
}

// convertTimeToMS converts time string to milliseconds
func (ins *Instance) convertTimeToMS(timeStr string) float64 {
	totalMS := 0.0
	parts := strings.Fields(timeStr)

	for i := 0; i < len(parts); i += 2 {
		if i+1 >= len(parts) {
			break
		}

		valueStr := parts[i]
		unit := parts[i+1]

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			log.Printf("W! [spark_streaming] Failed to parse time value: %s", valueStr)
			continue
		}

		// Convert to milliseconds
		switch unit {
		case "ms":
			totalMS += value
		case "s", "seconds", "sec":
			totalMS += value * 1000
		case "min", "minutes":
			totalMS += value * 60 * 1000
		case "h", "hours":
			totalMS += value * 60 * 60 * 1000
		case "d", "days":
			totalMS += value * 24 * 60 * 60 * 1000
		default:
			log.Printf("W! [spark_streaming] Unknown time unit: %s", unit)
		}
	}

	return totalMS
}

// parseBatchCount parses batch count from text using regex pattern
func (ins *Instance) parseBatchCount(text string, pattern *regexp.Regexp) int64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	matches := pattern.FindStringSubmatch(text)
	if len(matches) > 1 {
		if count, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
			return count
		}
	}

	log.Printf("D! [spark_streaming] Failed to parse batch count from: %s", text)
	return 0
}
