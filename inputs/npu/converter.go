package npu

import (
	"fmt"
	"strconv"

	"flashcat.cloud/categraf/types"
)

// ConvertMetricsToSamples converts NPU metrics to Categraf samples
func ConvertMetricsToSamples(metrics *ChipMetrics, slist *types.SampleList) {
	if metrics == nil {
		return
	}

	chip := metrics.Chip
	labels := buildLabels(chip)

	// Basic metrics
	pushMetric(slist, "temperature", float64(metrics.Temperature), labels)
	pushMetric(slist, "power", float64(metrics.Power), labels)
	pushMetric(slist, "voltage", float64(metrics.Voltage), labels)
	pushMetric(slist, "utilization", float64(metrics.Utilization), labels)
	pushMetric(slist, "overall_utilization", float64(metrics.OverallUtilization), labels)
	pushMetric(slist, "vector_utilization", float64(metrics.VectorUtilization), labels)
	pushMetric(slist, "aicore_freq", float64(metrics.AICoreCurrentFreq), labels)

	// Health status (convert to numeric: 1=Healthy, 0=UnHealthy, -1=Abnormal)
	healthCode := healthStatusToCode(metrics.HealthStatus)
	pushMetric(slist, "health_status", float64(healthCode), labels)

	netHealthCode := healthStatusToCode(metrics.NetHealthStatus)
	pushMetric(slist, "network_health_status", float64(netHealthCode), labels)

	// Memory metrics
	if metrics.HBMTotal > 0 {
		pushMetric(slist, "hbm_total_mb", float64(metrics.HBMTotal), labels)
		pushMetric(slist, "hbm_used_mb", float64(metrics.HBMUsed), labels)
		pushMetric(slist, "hbm_free_mb", float64(metrics.HBMFree), labels)
		if metrics.HBMTotal > 0 {
			utilization := float64(metrics.HBMUsed) / float64(metrics.HBMTotal) * 100
			pushMetric(slist, "hbm_utilization", utilization, labels)
		}
	}

	if metrics.DDRTotal > 0 {
		pushMetric(slist, "ddr_total_mb", float64(metrics.DDRTotal), labels)
		pushMetric(slist, "ddr_used_mb", float64(metrics.DDRUsed), labels)
		pushMetric(slist, "ddr_free_mb", float64(metrics.DDRFree), labels)
		if metrics.DDRTotal > 0 {
			utilization := float64(metrics.DDRUsed) / float64(metrics.DDRTotal) * 100
			pushMetric(slist, "ddr_utilization", utilization, labels)
		}
	}

	// Error codes
	for i, code := range metrics.ErrorCodes {
		errorLabels := copyLabels(labels)
		errorLabels["error_index"] = strconv.Itoa(i)
		pushMetric(slist, "error_code", float64(code), errorLabels)
	}

	// Process information
	if metrics.DevProcessInfo != nil {
		pushMetric(slist, "process_count", float64(metrics.DevProcessInfo.ProcNum), labels)

		// Individual process memory usage
		for i := int32(0); i < metrics.DevProcessInfo.ProcNum; i++ {
			procInfo := metrics.DevProcessInfo.DevProcArray[i]
			procLabels := copyLabels(labels)
			procLabels["pid"] = strconv.FormatInt(int64(procInfo.Pid), 10)
			pushMetric(slist, "process_memory_mb", float64(procInfo.MemUsage), procLabels)
		}
	}

	// Device info metric (value=1, used for metadata)
	infoLabels := copyLabels(labels)
	if chip.ChipInfo != nil {
		infoLabels["chip_name"] = chip.ChipInfo.Name
		infoLabels["chip_type"] = chip.ChipInfo.Type
	}
	if chip.ElabelInfo != nil {
		infoLabels["serial_number"] = chip.ElabelInfo.SerialNumber
	}
	infoLabels["pcie_bus"] = chip.PCIeBusInfo
	pushMetric(slist, "info", 1, infoLabels)
}

// buildLabels creates standard labels for a chip
func buildLabels(chip HuaWeiAIChip) map[string]string {
	labels := map[string]string{
		"device_id": strconv.Itoa(int(chip.DeviceID)),
		"logic_id":  strconv.Itoa(int(chip.LogicID)),
		"card_id":   strconv.Itoa(int(chip.CardId)),
		"phy_id":    strconv.Itoa(int(chip.PhyId)),
	}

	// Add vNPU info if present
	if chip.VDevActivityInfo != nil {
		labels["vdev_id"] = strconv.Itoa(int(chip.VDevActivityInfo.VDevID))
	}

	return labels
}

// copyLabels creates a copy of the labels map
func copyLabels(labels map[string]string) map[string]string {
	copied := make(map[string]string, len(labels))
	for k, v := range labels {
		copied[k] = v
	}
	return copied
}

// pushMetric adds a metric to the sample list with validation
func pushMetric(slist *types.SampleList, name string, value float64, labels map[string]string) {
	// Skip invalid values
	if isInvalidValue(value) {
		return
	}

	metricName := fmt.Sprintf("%s_%s", inputName, name)
	slist.PushFront(types.NewSample(inputName, metricName, value, labels))
}

// isInvalidValue checks if a metric value is invalid
func isInvalidValue(value float64) bool {
	// Check for error sentinel values
	if value == float64(RetError) || value == float64(UnRetError) {
		return true
	}
	// Check for NaN or Inf
	if value != value { // NaN check
		return true
	}
	return false
}

// healthStatusToCode converts health status string to numeric code
func healthStatusToCode(status string) int {
	switch status {
	case HealthyStatus:
		return 1
	case UnHealthyStatus:
		return 0
	case AbnormalStatus:
		return -1
	default:
		return -1
	}
}

// ConvertChipListToSample creates a summary metric for all chips
func ConvertChipListToSample(chips []HuaWeiAIChip, slist *types.SampleList) {
	// Total chip count
	slist.PushFront(types.NewSample(inputName, "npu_chip_count", float64(len(chips)), nil))

	// Count by card
	cardCounts := make(map[int32]int)
	for _, chip := range chips {
		cardCounts[chip.CardId]++
	}

	for cardID, count := range cardCounts {
		labels := map[string]string{
			"card_id": strconv.Itoa(int(cardID)),
		}
		slist.PushFront(types.NewSample(inputName, "npu_chips_per_card", float64(count), labels))
	}
}
