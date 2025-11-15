package npu

import (
	"context"
	"fmt"
	"sync"
	"time"

	"ascend-common/devmanager"
	"ascend-common/devmanager/common"

	"github.com/sirupsen/logrus"
)

// NpuCollector manages NPU device discovery and metric collection
type NpuCollector struct {
	dmgr       *devmanager.DeviceManager
	cache      *Cache
	cacheTime  time.Duration
	updateTime time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     *sync.WaitGroup

	initialized bool
}

// NewNpuCollector creates a new NPU collector instance
func NewNpuCollector(cacheTime, updateTime time.Duration) (*NpuCollector, error) {
	// Initialize device manager with auto-detection
	dmgr, err := devmanager.AutoInit("", 0)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize device manager: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	collector := &NpuCollector{
		dmgr:       dmgr,
		cache:      NewCache(),
		cacheTime:  cacheTime,
		updateTime: updateTime,
		ctx:        ctx,
		cancel:     cancel,
		wg:         &sync.WaitGroup{},
	}

	return collector, nil
}

// Start begins background metric collection
func (nc *NpuCollector) Start() error {
	// Initial chip list collection
	chips, err := nc.discoverChips()
	if err != nil {
		return fmt.Errorf("failed to discover NPU chips: %w", err)
	}

	if len(chips) == 0 {
		return fmt.Errorf("no NPU chips found")
	}

	logrus.Infof("discovered %d NPU chip(s)", len(chips))
	nc.cache.Set(CacheKeyChipList, chips, nc.cacheTime)

	// Start chip list update goroutine
	nc.wg.Add(1)
	go nc.chipListUpdateLoop()

	// Start metrics collection goroutine
	nc.wg.Add(1)
	go nc.metricsCollectionLoop()

	nc.initialized = true
	return nil
}

// Stop gracefully stops the collector
func (nc *NpuCollector) Stop() {
	if nc.cancel != nil {
		nc.cancel()
	}
	if nc.wg != nil {
		nc.wg.Wait()
	}
	logrus.Info("NPU collector stopped")
}

// GetChips returns the cached chip list
func (nc *NpuCollector) GetChips() []HuaWeiAIChip {
	value, exists := nc.cache.Get(CacheKeyChipList)
	if !exists {
		logrus.Warn("chip list not in cache, attempting discovery")
		chips, err := nc.discoverChips()
		if err != nil {
			logrus.Errorf("failed to discover chips: %v", err)
			return []HuaWeiAIChip{}
		}
		nc.cache.Set(CacheKeyChipList, chips, nc.cacheTime)
		return chips
	}

	chips, ok := value.([]HuaWeiAIChip)
	if !ok {
		logrus.Error("invalid chip list in cache")
		return []HuaWeiAIChip{}
	}

	return chips
}

// GetMetrics returns cached metrics for a specific chip
func (nc *NpuCollector) GetMetrics(phyID int32) (*ChipMetrics, bool) {
	key := GetMetricsCacheKey(phyID)
	value, exists := nc.cache.Get(key)
	if !exists {
		return nil, false
	}

	metrics, ok := value.(*ChipMetrics)
	if !ok {
		return nil, false
	}

	return metrics, true
}

// chipListUpdateLoop periodically updates the chip list
func (nc *NpuCollector) chipListUpdateLoop() {
	defer nc.wg.Done()

	ticker := time.NewTicker(1 * time.Minute) // Update chip list every minute
	defer ticker.Stop()

	for {
		select {
		case <-nc.ctx.Done():
			logrus.Info("chip list update loop stopped")
			return
		case <-ticker.C:
			chips, err := nc.discoverChips()
			if err != nil {
				logrus.Errorf("failed to update chip list: %v", err)
				continue
			}
			nc.cache.Set(CacheKeyChipList, chips, nc.cacheTime)
			logrus.Debugf("updated chip list: %d chips", len(chips))
		}
	}
}

// metricsCollectionLoop periodically collects metrics for all chips
func (nc *NpuCollector) metricsCollectionLoop() {
	defer nc.wg.Done()

	ticker := time.NewTicker(nc.updateTime)
	defer ticker.Stop()

	for {
		select {
		case <-nc.ctx.Done():
			logrus.Info("metrics collection loop stopped")
			return
		case <-ticker.C:
			nc.collectAllMetrics()
		}
	}
}

// collectAllMetrics collects metrics for all discovered chips
func (nc *NpuCollector) collectAllMetrics() {
	chips := nc.GetChips()
	if len(chips) == 0 {
		return
	}

	for _, chip := range chips {
		metrics := nc.collectChipMetrics(chip)
		key := GetMetricsCacheKey(chip.PhyId)
		nc.cache.Set(key, metrics, nc.cacheTime)
	}
}

// discoverChips discovers all NPU chips in the system
func (nc *NpuCollector) discoverChips() ([]HuaWeiAIChip, error) {
	chipList := make([]HuaWeiAIChip, 0)

	cardNum, cards, err := nc.dmgr.GetCardList()
	if err != nil || cardNum == 0 {
		return nil, fmt.Errorf("failed to get card list: %w", err)
	}

	for _, cardID := range cards {
		deviceNum, err := nc.dmgr.GetDeviceNumInCard(cardID)
		if err != nil {
			logrus.Errorf("failed to get device count for card %d: %v", cardID, err)
			continue
		}

		for deviceID := int32(0); deviceID < deviceNum; deviceID++ {
			chip, err := nc.getChipInfo(cardID, deviceID)
			if err != nil {
				logrus.Errorf("failed to get chip info for card %d device %d: %v", cardID, deviceID, err)
				continue
			}
			chipList = append(chipList, chip)
		}
	}

	return chipList, nil
}

// getChipInfo retrieves detailed information for a specific chip
func (nc *NpuCollector) getChipInfo(cardID, deviceID int32) (HuaWeiAIChip, error) {
	var chip HuaWeiAIChip

	// Get logic ID
	logicID, err := nc.dmgr.GetDeviceLogicID(cardID, deviceID)
	if err != nil {
		return chip, fmt.Errorf("failed to get logic ID: %w", err)
	}

	chip.LogicID = logicID
	chip.CardId = cardID
	chip.MainBoardId = nc.dmgr.GetMainBoardId()

	// Get physical ID
	phyID, err := nc.dmgr.GetPhysicIDFromLogicID(logicID)
	if err != nil {
		logrus.Warnf("failed to get physical ID for logic ID %d: %v", logicID, err)
	}
	chip.PhyId = phyID
	chip.DeviceID = phyID

	// Get chip info
	chipInfo, err := nc.dmgr.GetChipInfo(logicID)
	if err != nil {
		logrus.Warnf("failed to get chip info for logic ID %d: %v", logicID, err)
		chipInfo = &common.ChipInfo{}
	}
	chip.ChipInfo = chipInfo

	// Get board info
	boardInfo, err := nc.dmgr.GetBoardInfo(logicID)
	if err != nil {
		logrus.Debugf("failed to get board info for logic ID %d: %v", logicID, err)
		boardInfo = common.BoardInfo{}
	}
	chip.BoardInfo = &boardInfo

	// Get PCIe bus info
	pcieInfo, err := nc.dmgr.GetPCIeBusInfo(logicID)
	if err != nil {
		logrus.Debugf("failed to get PCIe info for logic ID %d: %v", logicID, err)
		pcieInfo = ""
	}
	chip.PCIeBusInfo = pcieInfo

	// Get electronic label info
	elabelInfo, err := nc.dmgr.GetCardElabelV2(cardID)
	if err != nil {
		logrus.Debugf("failed to get elabel info for card %d: %v", cardID, err)
		chip.ElabelInfo = &ElabelInfo{SerialNumber: "NA"}
	} else {
		chip.ElabelInfo = &ElabelInfo{
			SerialNumber: elabelInfo.SerialNumber,
		}
	}

	return chip, nil
}

// collectChipMetrics collects all metrics for a single chip
func (nc *NpuCollector) collectChipMetrics(chip HuaWeiAIChip) *ChipMetrics {
	logicID := chip.LogicID
	metrics := &ChipMetrics{
		Chip:      chip,
		Timestamp: time.Now(),
	}

	// Collect health status
	health, err := nc.dmgr.GetDeviceHealth(logicID)
	if err != nil || health != 0 {
		metrics.HealthStatus = UnHealthyStatus
	} else {
		metrics.HealthStatus = HealthyStatus
	}

	// Collect error codes
	_, errorCodes, err := nc.dmgr.GetDeviceAllErrorCode(logicID)
	if err != nil {
		metrics.ErrorCodes = []int64{}
	} else {
		metrics.ErrorCodes = errorCodes
	}

	// Collect temperature
	temp, err := nc.dmgr.GetDeviceTemperature(logicID)
	if err != nil {
		metrics.Temperature = RetError
	} else {
		metrics.Temperature = int(temp)
	}

	// Collect power
	power, err := nc.dmgr.GetDevicePowerInfo(logicID)
	if err != nil {
		metrics.Power = float32(RetError)
	} else {
		metrics.Power = power
	}

	// Collect voltage
	voltage, err := nc.dmgr.GetDeviceVoltage(logicID)
	if err != nil {
		metrics.Voltage = float32(UnRetError)
	} else {
		metrics.Voltage = voltage
	}

	// Collect AI Core utilization
	util, err := nc.dmgr.GetDeviceUtilizationRate(logicID, common.AICore)
	if err != nil {
		metrics.Utilization = RetError
	} else {
		metrics.Utilization = int(util)
	}

	// Collect overall utilization
	overallUtil, err := nc.dmgr.GetDeviceUtilizationRate(logicID, common.Overall)
	if err != nil {
		metrics.OverallUtilization = RetError
	} else {
		metrics.OverallUtilization = int(overallUtil)
	}

	// Collect vector core utilization
	vectorUtil, err := nc.dmgr.GetDeviceUtilizationRate(logicID, common.VectorCore)
	if err != nil {
		metrics.VectorUtilization = RetError
	} else {
		metrics.VectorUtilization = int(vectorUtil)
	}

	// Collect AI Core frequency
	freq, err := nc.dmgr.GetDeviceFrequency(logicID, common.AICoreCurrentFreq)
	if err != nil {
		metrics.AICoreCurrentFreq = UnRetError
	} else {
		metrics.AICoreCurrentFreq = freq
	}

	// Collect network health status
	if nc.dmgr.IsTrainingCard() {
		netCode, err := nc.dmgr.GetDeviceNetWorkHealth(logicID)
		if err != nil {
			metrics.NetHealthStatus = AbnormalStatus
		} else if netCode == NetworkInit || netCode == NetworkSuccess {
			metrics.NetHealthStatus = HealthyStatus
		} else {
			metrics.NetHealthStatus = UnHealthyStatus
		}
	} else {
		metrics.NetHealthStatus = AbnormalStatus
	}

	// Collect process info
	processInfo, err := nc.dmgr.GetDevProcessInfo(logicID)
	if err != nil {
		metrics.DevProcessInfo = &common.DevProcessInfo{}
	} else {
		metrics.DevProcessInfo = processInfo
	}

	// Collect memory info (HBM)
	hbmInfo, err := nc.dmgr.GetDeviceMemoryInfo(logicID)
	if err == nil {
		metrics.HBMTotal = uint64(hbmInfo.MemorySize)
		metrics.HBMUsed = uint64(hbmInfo.MemoryUsage)
		metrics.HBMFree = metrics.HBMTotal - metrics.HBMUsed
	}

	return metrics
}

// IsInitialized returns whether the collector has been initialized
func (nc *NpuCollector) IsInitialized() bool {
	return nc.initialized
}
