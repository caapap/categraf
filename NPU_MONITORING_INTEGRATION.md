# NPU 监控集成开发方案

## 1. 项目背景

### 1.1 目标
将华为昇腾 NPU（Neural Processing Unit）监控功能集成到 Categraf 中，实现对 NPU 设备的指标采集和上报。

### 1.2 参考实现
- **nvidia_smi 插件**：通过执行 `nvidia-smi` 命令获取 GPU 指标
- **npu-exporter**：华为昇腾 NPU 的官方监控导出器（来源：https://gitcode.com/Ascend/mind-cluster）

## 2. 技术方案对比

### 2.1 nvidia_smi 实现方式（参考）

**架构特点：**
- 通过执行外部命令 `nvidia-smi` 获取指标
- 解析 CSV 格式输出
- 轻量级，无外部依赖
- 被动采集模式（每次 Gather 时执行命令）

**代码结构：**
```
inputs/nvidia_smi/
├── nvidia_smi.go      # 主入口，实现 Input 接口
├── scrape.go          # 执行命令并获取输出
├── parser.go          # 解析 CSV 输出
├── fields.go          # 字段定义和映射
├── types.go           # 数据类型定义
└── util.go            # 工具函数
```

**关键实现：**
```go
// 1. Init: 构建查询字段映射
func (s *GPUStats) Init() error {
    qFieldsOrdered, qFieldToRFieldMap, err := s.buildQFieldToRFieldMap()
    s.qFields = qFieldsOrdered
    s.qFieldToMetricInfoMap = buildQFieldToMetricInfoMap(qFieldToRFieldMap)
    return nil
}

// 2. Gather: 执行命令并解析结果
func (s *GPUStats) Gather(slist *types.SampleList) {
    currentTable, err := s.scrape(s.qFields)
    // 解析并推送指标
    for _, currentRow := range currentTable.rows {
        // 推送指标到 slist
    }
}
```

### 2.2 npu-exporter 实现方式（目标）

**架构特点：**
- 通过华为 `ascend-common` SDK 直接调用 API
- 使用缓存机制，后台 goroutine 定期采集
- 支持多维度指标（芯片、容器、网络、HBM、DDR等）
- 支持虚拟 NPU（vNPU）监控

**代码结构：**
```
component/npu-exporter/
├── cmd/npu-exporter/main.go          # 主程序入口
├── collector/
│   ├── common/
│   │   ├── npu_collector.go          # NPU 收集器核心
│   │   ├── types.go                  # 数据类型定义
│   │   └── metrics_collector.go     # 指标收集器接口
│   └── metrics/
│       ├── collector_for_npu.go      # NPU 基础指标
│       ├── collector_for_hbm.go      # HBM 内存指标
│       ├── collector_for_network.go  # 网络指标
│       └── ...
└── platforms/
    └── inputs/npu/
        └── npu.go                    # Telegraf 插件入口
```

**关键实现：**
```go
// 1. 初始化收集器（后台运行）
colcommon.Collector = colcommon.NewNpuCollector(...)
colcommon.StartCollect(wg, ctx, colcommon.Collector)

// 2. Gather 时从缓存获取数据
func (npu *WatchNPU) Gather(acc telegraf.Accumulator) error {
    chips := common.GetChipListWithVNPU(npu.collector)
    // 从缓存获取指标并推送
    fieldsMap = npu.gatherChain(fieldsMap, ...)
}
```

## 3. 集成方案设计

### 3.1 方案选择

**方案 A：直接调用 SDK（推荐）**
- ✅ 性能好，实时性强
- ✅ 功能完整，支持所有指标
- ❌ 需要依赖 `ascend-common` SDK
- ❌ 需要 CGO 支持（SDK 可能依赖 C 库）

**方案 B：通过命令调用（类似 nvidia_smi）**
- ✅ 无依赖，轻量级
- ✅ 实现简单
- ❌ 需要找到合适的命令行工具（如 `npu-smi`）
- ❌ 功能可能受限

**方案 C：HTTP 方式调用 npu-exporter**
- ✅ 解耦，无需直接依赖 SDK
- ✅ 可以复用现有 npu-exporter
- ❌ 需要额外部署 npu-exporter 服务
- ❌ 增加网络开销

**推荐方案：方案 A（直接调用 SDK）**
- 参考 npu-exporter 的实现
- 适配 Categraf 的 Input 接口
- 保持与 nvidia_smi 类似的代码结构

### 3.2 架构设计

```
inputs/npu/
├── npu.go                    # 主入口，实现 Input 接口
├── collector.go              # NPU 收集器封装
├── metrics.go                # 指标采集逻辑
├── types.go                  # 数据类型定义
├── cache.go                  # 缓存管理（可选）
└── README.md                 # 使用文档

conf/input.npu/
└── npu.toml                  # 配置文件
```

### 3.3 核心接口设计

```go
// inputs/npu/npu.go
type NPU struct {
    config.PluginConfig
    
    // 配置项
    Enabled         bool            `toml:"enabled"`          // 是否启用
    UpdateInterval  config.Duration `toml:"update_interval"` // 更新间隔
    CacheTime       config.Duration `toml:"cache_time"`      // 缓存时间
    
    // 内部状态
    collector       *NpuCollector
    initialized     bool
}

func (n *NPU) Init() error {
    // 1. 检查 enabled
    // 2. 初始化 ascend-common SDK
    // 3. 创建 NpuCollector
    // 4. 启动后台采集 goroutine
}

func (n *NPU) Gather(slist *types.SampleList) {
    // 1. 从缓存获取指标
    // 2. 转换为 Categraf Sample
    // 3. 推送到 slist
}
```

## 4. 实现步骤

### 4.1 第一阶段：基础框架搭建

**任务清单：**
1. ✅ 创建 `inputs/npu/` 目录结构
2. ✅ 实现基础的 Input 接口（Init、Gather、Clone、Name）
3. ✅ 添加配置文件 `conf/input.npu/npu.toml`
4. ✅ 实现 enabled 开关（默认 false）
5. ✅ 添加空地址检查（类似 spark_streaming）

**代码示例：**
```go
// inputs/npu/npu.go
package npu

import (
    "flashcat.cloud/categraf/config"
    "flashcat.cloud/categraf/inputs"
    "flashcat.cloud/categraf/types"
)

const inputName = "npu"

type NPU struct {
    config.PluginConfig
    Enabled bool `toml:"enabled"`
}

func init() {
    inputs.Add(inputName, func() inputs.Input {
        return &NPU{}
    })
}

func (n *NPU) Init() error {
    if !n.Enabled {
        return nil
    }
    // TODO: 初始化 SDK 和收集器
    return nil
}

func (n *NPU) Gather(slist *types.SampleList) {
    if !n.Enabled || !n.initialized {
        return
    }
    // TODO: 采集指标
}
```

### 4.2 第二阶段：SDK 集成

**任务清单：**
1. 添加 `ascend-common` SDK 依赖
2. 实现 `NpuCollector` 封装
3. 实现设备发现和初始化
4. 实现后台采集 goroutine

**依赖管理：**
```go
// go.mod 中添加
require (
    ascend-common/api v1.0.0
    ascend-common/devmanager v1.0.0
    // ...
)
```

**关键代码：**
```go
// inputs/npu/collector.go
import (
    "ascend-common/api"
    "ascend-common/devmanager"
)

type NpuCollector struct {
    Dmgr   devmanager.DeviceInterface
    cache  *Cache
    ctx    context.Context
    cancel context.CancelFunc
}

func NewNpuCollector() (*NpuCollector, error) {
    dmgr, err := devmanager.AutoInit("", 0)
    if err != nil {
        return nil, err
    }
    
    ctx, cancel := context.WithCancel(context.Background())
    return &NpuCollector{
        Dmgr:   dmgr,
        cache:  NewCache(),
        ctx:    ctx,
        cancel: cancel,
    }, nil
}

func (nc *NpuCollector) StartCollect(interval time.Duration) {
    go func() {
        ticker := time.NewTicker(interval)
        defer ticker.Stop()
        
        for {
            select {
            case <-nc.ctx.Done():
                return
            case <-ticker.C:
                nc.collectMetrics()
            }
        }
    }()
}
```

### 4.3 第三阶段：指标采集实现

**任务清单：**
1. 实现基础指标采集（温度、功耗、利用率等）
2. 实现 HBM/DDR 内存指标
3. 实现网络指标
4. 实现容器关联指标（可选）

**指标映射：**
```go
// inputs/npu/metrics.go
func (n *NPU) collectBaseMetrics(chip common.HuaWeiAIChip) map[string]interface{} {
    fields := make(map[string]interface{})
    
    // 温度
    temp, _ := n.collector.Dmgr.GetDeviceTemperature(chip.LogicID)
    fields["temperature"] = temp
    
    // 功耗
    power, _ := n.collector.Dmgr.GetDevicePowerInfo(chip.LogicID)
    fields["power"] = power
    
    // 利用率
    util, _ := n.collector.Dmgr.GetDeviceUtilizationRate(chip.LogicID, common.AICore)
    fields["utilization"] = util
    
    // ... 更多指标
    
    return fields
}
```

### 4.4 第四阶段：指标推送

**任务清单：**
1. 实现指标到 Sample 的转换
2. 添加标签（device_id、card_id、logic_id 等）
3. 实现错误处理和降级策略

**代码示例：**
```go
func (n *NPU) Gather(slist *types.SampleList) {
    chips := n.collector.GetChipList()
    
    for _, chip := range chips {
        labels := map[string]string{
            "device_id": strconv.Itoa(int(chip.DeviceID)),
            "card_id":   strconv.Itoa(int(chip.CardId)),
            "logic_id":  strconv.Itoa(int(chip.LogicID)),
        }
        
        metrics := n.collector.GetMetrics(chip.PhyId)
        for name, value := range metrics {
            slist.PushSample(inputName, name, value, labels)
        }
    }
}
```

### 4.5 第五阶段：测试和优化

**任务清单：**
1. 单元测试
2. 集成测试
3. 性能优化（缓存、并发）
4. 文档编写

## 5. 配置文件设计

```toml
# conf/input.npu/npu.toml
[[instances]]
# 是否启用 NPU 监控（默认 false）
enabled = false

# 指标更新间隔（默认 5s）
update_interval = "5s"

# 缓存时间（默认 65s）
cache_time = "65s"

# 容器监控模式（可选：docker, containerd, isula）
# container_mode = "docker"

# 自定义标签
[instances.labels]
cluster = "production"
region = "beijing"
```

## 6. 指标清单

### 6.1 基础指标
- `npu_temperature` - NPU 温度（℃）
- `npu_power` - NPU 功耗（W）
- `npu_voltage` - NPU 电压（V）
- `npu_utilization` - AI Core 利用率（%）
- `npu_overall_utilization` - 整体利用率（%）
- `npu_vector_utilization` - Vector Core 利用率（%）
- `npu_aicore_freq` - AI Core 频率（MHz）
- `npu_health_status` - 健康状态（0=异常，1=正常）

### 6.2 内存指标
- `npu_hbm_total` - HBM 总内存（MB）
- `npu_hbm_used` - HBM 已用内存（MB）
- `npu_hbm_free` - HBM 空闲内存（MB）
- `npu_ddr_total` - DDR 总内存（MB）
- `npu_ddr_used` - DDR 已用内存（MB）

### 6.3 网络指标
- `npu_network_status` - 网络健康状态
- `npu_hccs_bandwidth` - HCCS 带宽（可选）

### 6.4 容器指标（可选）
- `npu_container_utilization` - 容器 NPU 利用率
- `npu_container_memory_total` - 容器 NPU 总内存
- `npu_container_memory_used` - 容器 NPU 已用内存

## 7. 依赖管理

### 7.1 外部依赖
- `ascend-common/api` - 华为昇腾 API
- `ascend-common/devmanager` - 设备管理器
- `ascend-common/common-utils` - 通用工具

### 7.2 依赖获取
```bash
# 需要从华为官方获取 SDK
# 可能需要添加到私有仓库或 vendor 目录
```

## 8. 兼容性考虑

### 8.1 与 nvidia_smi 的差异
- nvidia_smi：命令执行模式，无状态
- npu：SDK 调用模式，有状态（需要初始化）

### 8.2 降级策略
- 如果 SDK 初始化失败，静默跳过（不报错）
- 如果采集失败，记录错误但不中断其他指标采集
- 支持 enabled=false 时完全禁用

## 9. 测试计划

### 9.1 单元测试
- 测试 Init/Gather 接口
- 测试指标转换逻辑
- 测试错误处理

### 9.2 集成测试
- 测试与真实 NPU 设备的交互
- 测试多卡场景
- 测试容器场景

### 9.3 性能测试
- 采集延迟测试
- 内存占用测试
- CPU 占用测试

## 10. 风险评估

### 10.1 技术风险
- **风险**：SDK 依赖可能不稳定
- **缓解**：封装 SDK 调用，提供降级方案

- **风险**：CGO 依赖可能导致编译问题
- **缓解**：提供条件编译选项

### 10.2 兼容性风险
- **风险**：不同 NPU 型号可能行为不同
- **缓解**：参考 npu-exporter 的实现，已处理多型号兼容

## 11. 后续优化

1. 支持 Prometheus 格式导出（可选）
2. 支持自定义指标过滤
3. 支持指标聚合
4. 支持告警规则配置

## 12. 参考资源

- [nvidia_smi 实现](../inputs/nvidia_smi/)
- [npu-exporter 源码](https://gitcode.com/Ascend/mind-cluster/tree/master/component/npu-exporter)
- [华为昇腾文档](https://www.hiascend.com/)

## 13. 开发时间估算

- 第一阶段（基础框架）：2-3 天
- 第二阶段（SDK 集成）：3-5 天
- 第三阶段（指标采集）：5-7 天
- 第四阶段（指标推送）：2-3 天
- 第五阶段（测试优化）：3-5 天

**总计：15-23 个工作日**

