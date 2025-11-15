package npu

import (
	"time"

	"ascend-common/devmanager/common"
)

// HuaWeiAIChip represents a Huawei Ascend NPU chip
// Simplified version from npu-exporter collector/common/types.go
type HuaWeiAIChip struct {
	// Basic identification
	LogicID     int32 // Logical device ID
	PhyId       int32 // Physical device ID
	DeviceID    int32 // Device ID (same as PhyId)
	CardId      int32 // Card ID
	MainBoardId int32 // Mainboard ID
	VDieID      int32 // Virtual die ID

	// Device information
	ChipInfo         *common.ChipInfo         // Chip information
	BoardInfo        *common.BoardInfo        // Board information
	ElabelInfo       *ElabelInfo              // Electronic label info
	PCIeBusInfo      string                   // PCIe bus information
	VDevInfos        *common.VirtualDevInfo   // Virtual device info (for vNPU)
	VDevActivityInfo *common.VDevActivityInfo // Active vNPU info
}

// ElabelInfo contains electronic label information
type ElabelInfo struct {
	SerialNumber string
}

// ChipMetrics contains collected metrics for a chip
type ChipMetrics struct {
	Chip      HuaWeiAIChip
	Timestamp time.Time

	// Health and status
	HealthStatus    string  // Health status: "Healthy", "UnHealthy", "Abnormal"
	NetHealthStatus string  // Network health status
	ErrorCodes      []int64 // Error codes

	// Utilization
	Utilization        int // AI Core utilization (%)
	OverallUtilization int // Overall utilization (%)
	VectorUtilization  int // Vector Core utilization (%)

	// Temperature and power
	Temperature int     // Temperature (°C)
	Power       float32 // Power consumption (W)
	Voltage     float32 // Voltage (V)

	// Frequency
	AICoreCurrentFreq uint32 // AI Core current frequency (MHz)

	// Memory (will be filled by memory collectors)
	HBMTotal uint64 // HBM total memory (MB)
	HBMUsed  uint64 // HBM used memory (MB)
	HBMFree  uint64 // HBM free memory (MB)
	DDRTotal uint64 // DDR total memory (MB)
	DDRUsed  uint64 // DDR used memory (MB)
	DDRFree  uint64 // DDR free memory (MB)

	// Process information
	DevProcessInfo *common.DevProcessInfo
}

// Health status constants
const (
	HealthyStatus   = "Healthy"
	UnHealthyStatus = "UnHealthy"
	AbnormalStatus  = "Abnormal"
)

// Network health status codes
const (
	NetworkInit    = 0
	NetworkSuccess = 1
)

// Error return values
const (
	RetError   = -1
	UnRetError = 0xFFFFFFFF
)
