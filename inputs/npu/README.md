# NPU Input Plugin

华为昇腾 NPU（Neural Processing Unit）监控插件，用于 Categraf 采集 Ascend NPU 设备的指标数据。

## 目录

- [概述](#概述)
- [快速开始](#快速开始)
- [系统要求](#系统要求)
- [配置说明](#配置说明)
- [采集指标](#采集指标)
- [架构设计](#架构设计)
- [故障排查](#故障排查)
- [性能调优](#性能调优)
- [集成说明](#集成说明)
- [版本历史](#版本历史)

## 概述

NPU 插件通过 `ascend-common` SDK 直接调用华为 Ascend NPU 设备的 API，实现高性能、低延迟的指标采集。

### 支持的设备

- Ascend 910A
- Ascend 910B
- Ascend 310P
- 其他 Ascend NPU 系列

### 核心特性

- ✅ **后台采集**：独立 goroutine 定时采集，Gather 时直接读缓存，延迟 <10ms
- ✅ **错误降级**：初始化失败时静默禁用，不影响 Categraf 启动
- ✅ **资源管理**：优雅启动和关闭，自动清理资源
- ✅ **缓存机制**：减少硬件访问频率，提升性能
- ✅ **完全自包含**：ascend-common SDK 已集成到 `pkg/ascend-common/`

## 快速开始

### 1. 前置检查

```bash
# 检查 NPU 设备
ls -l /dev/davinci*

# 检查驱动版本
cat /proc/driver/ascend/version

# 检查 ascend-common SDK（已集成到 categraf）
ls -la pkg/ascend-common/
```

### 2. 配置依赖

```bash
cd categraf

# 确认 go.mod 中的 replace 路径正确
grep "ascend-common" go.mod

# 应该看到：
# require (
#     ascend-common v0.0.0
# )
# replace (
#     ascend-common => ./pkg/ascend-common
# )

# 下载依赖
go mod tidy
```

### 3. 编译

```bash
# 编译 Categraf
go build

# 验证编译成功
./categraf --version
```

### 4. 配置插件

编辑 `conf/input.npu/npu.toml`：

```toml
[[instances]]
# 启用 NPU 监控（默认 false，需显式启用）
enabled = true

# 采集间隔（默认 5s，范围 1s-60s）
update_interval = "5s"

# 缓存时间（默认 65s）
cache_time = "65s"

# 自定义标签（可选）
[instances.labels]
environment = "production"
cluster = "ai-cluster-01"
```

### 5. 测试运行

```bash
# 测试模式（不启动完整服务）
./categraf --test --inputs npu --configs conf/

# 预期输出：
# npu_up 1
# npu_chip_count 8
# npu_temperature{device_id="0",...} 45.0
# npu_power{device_id="0",...} 120.5
# ...
```

### 6. 正式运行

```bash
# 前台运行（查看日志）
./categraf --configs conf/

# 后台运行
nohup ./categraf --configs conf/ > categraf.log 2>&1 &

# 查看日志
tail -f categraf.log | grep -i npu
```

## 系统要求

### 硬件要求

- 华为 Ascend NPU 硬件已安装

### 软件要求

1. **Ascend Driver**：NPU 驱动必须已安装
   ```bash
   # 检查驱动版本
   cat /proc/driver/ascend/version
   ```

2. **Ascend Toolkit**（可选）：开发工具包
   ```bash
   # 典型安装路径
   /usr/local/Ascend/ascend-toolkit/latest/
   ```

3. **ascend-common SDK**：已集成到 `categraf/pkg/ascend-common/`
   - 无需额外安装
   - 通过 go.mod replace 自动引用

### 权限要求

运行 Categraf 的进程必须能够访问 NPU 设备：

```bash
# 检查设备权限
ls -l /dev/davinci*

# 添加用户到设备组（如需要）
sudo usermod -a -G HwHiAiUser <username>
# 重新登录使权限生效
```

## 配置说明

### 配置文件位置

`conf/input.npu/npu.toml`

### 配置参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | `false` | 是否启用 NPU 监控（必须显式设置为 `true`） |
| `update_interval` | duration | `5s` | 指标采集间隔（范围：1s-60s） |
| `cache_time` | duration | `65s` | 缓存过期时间（建议 > update_interval） |
| `labels` | map | - | 自定义标签，会附加到所有指标 |

### 配置示例

#### 基础配置

```toml
[[instances]]
enabled = true
update_interval = "5s"
cache_time = "65s"
```

#### 生产环境配置

```toml
[[instances]]
enabled = true
update_interval = "10s"  # 降低采集频率
cache_time = "120s"      # 增加缓存时间

[instances.labels]
environment = "production"
datacenter = "beijing"
cluster = "ai-cluster-01"
```

## 采集指标

### 基础指标

| 指标 | 描述 | 单位 | 标签 |
|------|------|------|------|
| `npu_temperature` | NPU 温度 | °C | device_id, logic_id, card_id, phy_id |
| `npu_power` | 功耗 | W | device_id, logic_id, card_id, phy_id |
| `npu_voltage` | 电压 | V | device_id, logic_id, card_id, phy_id |
| `npu_utilization` | AI Core 利用率 | % | device_id, logic_id, card_id, phy_id |
| `npu_overall_utilization` | 整体利用率 | % | device_id, logic_id, card_id, phy_id |
| `npu_vector_utilization` | Vector Core 利用率 | % | device_id, logic_id, card_id, phy_id |
| `npu_aicore_freq` | AI Core 频率 | MHz | device_id, logic_id, card_id, phy_id |
| `npu_health_status` | 健康状态（1=正常，0=异常，-1=未知） | - | device_id, logic_id, card_id, phy_id |
| `npu_network_health_status` | 网络健康状态 | - | device_id, logic_id, card_id, phy_id |

### 内存指标

| 指标 | 描述 | 单位 | 标签 |
|------|------|------|------|
| `npu_hbm_total_mb` | HBM 总内存 | MB | device_id, logic_id, card_id, phy_id |
| `npu_hbm_used_mb` | HBM 已用内存 | MB | device_id, logic_id, card_id, phy_id |
| `npu_hbm_free_mb` | HBM 空闲内存 | MB | device_id, logic_id, card_id, phy_id |
| `npu_hbm_utilization` | HBM 利用率 | % | device_id, logic_id, card_id, phy_id |
| `npu_ddr_total_mb` | DDR 总内存 | MB | device_id, logic_id, card_id, phy_id |
| `npu_ddr_used_mb` | DDR 已用内存 | MB | device_id, logic_id, card_id, phy_id |
| `npu_ddr_free_mb` | DDR 空闲内存 | MB | device_id, logic_id, card_id, phy_id |
| `npu_ddr_utilization` | DDR 利用率 | % | device_id, logic_id, card_id, phy_id |

### 进程指标

| 指标 | 描述 | 单位 | 标签 |
|------|------|------|------|
| `npu_process_count` | 进程数 | count | device_id, logic_id, card_id, phy_id |
| `npu_process_memory_mb` | 进程内存使用 | MB | device_id, logic_id, card_id, phy_id, pid |

### 错误指标

| 指标 | 描述 | 单位 | 标签 |
|------|------|------|------|
| `npu_error_code` | 设备错误码 | - | device_id, logic_id, card_id, phy_id, error_index |

### 信息指标

| 指标 | 描述 | 单位 | 标签 |
|------|------|------|------|
| `npu_info` | 设备信息 | 1 | device_id, logic_id, card_id, phy_id, chip_name, chip_type, serial_number, pcie_bus |
| `npu_chip_count` | 芯片总数 | count | - |
| `npu_chips_per_card` | 每卡芯片数 | count | card_id |

### 状态指标

| 指标 | 描述 | 单位 | 标签 |
|------|------|------|------|
| `npu_up` | 采集器状态（1=正常，0=异常） | - | - |
| `npu_scrape_duration_seconds` | 采集耗时 | seconds | - |

### 标签说明

- **device_id**: 物理设备 ID
- **logic_id**: 逻辑设备 ID（驱动分配）
- **card_id**: 卡 ID（每卡可能有多个设备）
- **phy_id**: 物理 ID
- **vdev_id**: 虚拟设备 ID（vNPU 场景）
- **chip_name**: NPU 芯片型号名称
- **chip_type**: NPU 芯片类型
- **serial_number**: 设备序列号
- **pcie_bus**: PCIe 总线信息
- **pid**: 进程 ID（进程指标）
- **error_index**: 错误码索引（多个错误时）

## 架构设计

### 目录结构

```
categraf/
├── inputs/npu/              # NPU 插件代码
│   ├── npu.go               # Input 接口实现
│   ├── collector.go         # 核心采集器
│   ├── types.go             # 数据类型定义
│   ├── cache.go             # 缓存管理
│   ├── converter.go         # 格式转换
│   └── README.md            # 本文档
│
├── pkg/ascend-common/       # Ascend SDK（已集成）
│   ├── api/                 # API 定义
│   ├── devmanager/          # 设备管理
│   ├── common-utils/        # 通用工具
│   ├── go.mod
│   ├── go.sum
│   └── README.md            # SDK 文档
│
├── conf/input.npu/
│   └── npu.toml             # 配置文件
│
└── go.mod                   # replace: ascend-common => ./pkg/ascend-common
```

### 数据流程

```
┌─────────────────────────────────────────────────────┐
│                   Categraf                          │
│                                                     │
│  ┌──────────────────────────────────────────────┐  │
│  │           NPU Input Plugin                   │  │
│  │                                              │  │
│  │  ┌────────────┐      ┌─────────────┐       │  │
│  │  │   npu.go   │─────▶│ collector.go│       │  │
│  │  │  (Input)   │      │  (Collector)│       │  │
│  │  └────────────┘      └──────┬──────┘       │  │
│  │                              │              │  │
│  │                              ▼              │  │
│  │                       ┌─────────────┐      │  │
│  │                       │   cache.go  │      │  │
│  │                       │   (Cache)   │      │  │
│  │                       └─────────────┘      │  │
│  │                                             │  │
│  │  ┌────────────┐                            │  │
│  │  │converter.go│◀───────────────────────────┤  │
│  │  │(Converter) │                            │  │
│  │  └──────┬─────┘                            │  │
│  └─────────┼──────────────────────────────────┘  │
│            │                                      │
│            ▼                                      │
│    ┌───────────────┐                             │
│    │ SampleList    │                             │
│    │ (Metrics)     │                             │
│    └───────────────┘                             │
└─────────────────────────────────────────────────┘
            │
            ▼
    ┌───────────────┐
    │ ascend-common │
    │     SDK       │
    └───────┬───────┘
            │
            ▼
    ┌───────────────┐
    │  NPU Driver   │
    └───────┬───────┘
            │
            ▼
    ┌───────────────┐
    │  NPU Hardware │
    └───────────────┘
```

### 后台采集机制

插件使用后台 goroutine 定时采集指标：

1. **初始化阶段**：发现 NPU 设备并初始化 SDK
2. **后台循环**：按 `update_interval` 定时采集指标
3. **缓存存储**：将指标存入内存缓存（TTL = `cache_time`）
4. **Gather 调用**：从缓存读取指标，转换为 Categraf Sample 格式

**优势：**
- Gather() 调用延迟低（<10ms，直接读缓存）
- 减少硬件访问频率
- 采集间隔稳定可控

## 故障排查

### 编译失败 - 找不到 ascend-common

**错误信息：**
```
could not import ascend-common/devmanager
```

**解决方法：**
```bash
# 检查 ascend-common 是否存在
ls -la pkg/ascend-common/

# 如果不存在，重新获取
cd /tmp
git clone https://gitcode.com/Ascend/mind-cluster.git --depth 1
cp -r /tmp/mind-cluster/component/ascend-common /path/to/categraf/pkg/
rm -rf /tmp/mind-cluster

# 重新编译
cd /path/to/categraf
go mod tidy
go build
```

### 运行时报错 - 无法初始化设备

**错误信息：**
```
failed to create NPU collector: failed to initialize device manager
```

**解决方法：**
```bash
# 1. 检查驱动
cat /proc/driver/ascend/version

# 2. 检查设备权限
ls -l /dev/davinci*

# 3. 添加用户到设备组
sudo usermod -a -G HwHiAiUser $USER
# 重新登录使权限生效

# 4. 检查设备状态
npu-smi info
```

### 没有采集到指标

**检查步骤：**
```bash
# 1. 确认已启用
grep "enabled = true" conf/input.npu/npu.toml

# 2. 查看日志
grep -i "npu" categraf.log

# 3. 测试模式运行
./categraf --test --inputs npu --configs conf/

# 4. 查看初始化日志
grep "NPU monitoring initialized" categraf.log

# 5. 查看发现的设备
grep "discovered.*NPU chip" categraf.log
```

### 缺少内存指标

**可能原因：**
- 某些 NPU 型号不支持所有内存类型
- 驱动版本兼容性问题
- SDK API 可用性

**解决方法：** 查看 Ascend 文档确认您的 NPU 型号支持的功能

### CGO 编译问题

**如果遇到 CGO 相关错误：**
```bash
# 启用 CGO
export CGO_ENABLED=1

# 设置 C 编译器
export CC=gcc

# 如果需要指定库路径
export CGO_CFLAGS="-I/usr/local/Ascend/driver/include"
export CGO_LDFLAGS="-L/usr/local/Ascend/driver/lib64"

# 重新编译
go build
```

### 高 CPU 使用率

**解决方法：**
1. 增加 `update_interval`（如 10s 或 15s）
2. 增加 `cache_time` 以减少缓存更新频率
3. 检查 NPU 设备数量是否过多

## 性能调优

### 推荐配置

#### 小规模部署（1-8 NPU）
```toml
[[instances]]
enabled = true
update_interval = "5s"   # 默认值
cache_time = "65s"       # 默认值
```

#### 中等规模（8-32 NPU）
```toml
[[instances]]
enabled = true
update_interval = "10s"  # 降低采集频率
cache_time = "120s"      # 增加缓存时间
```

#### 大规模部署（32+ NPU）
```toml
[[instances]]
enabled = true
update_interval = "30s"  # 进一步降低频率
cache_time = "180s"      # 更长的缓存
```

### 资源消耗

典型资源消耗（每个 NPU 设备）：
- **内存**：~5-10 MB
- **CPU**：<1%（10s 采集间隔）
- **网络**：无（本地采集）

### 扩展性

| NPU 数量 | 推荐配置 | 预期负载 |
|---------|---------|---------|
| 1-8 | update_interval=5s | 低 |
| 8-32 | update_interval=10s | 中 |
| 32+ | update_interval=30s | 中-高 |

## 集成说明

### 依赖管理

**ascend-common SDK 集成**
- ✅ 已从 [Ascend mind-cluster](https://gitcode.com/Ascend/mind-cluster) 获取
- ✅ 已集成到 `categraf/pkg/ascend-common/`
- ✅ 完全自包含，无需外部目录依赖

**go.mod 配置**
```go
require (
    ascend-common v0.0.0
)

replace (
    ascend-common => ./pkg/ascend-common  // 指向本地 pkg 目录
)
```

### 优势

1. **自包含部署**：只需 categraf 目录即可
2. **版本锁定**：不受外部仓库更新影响
3. **简化依赖**：单一依赖路径，编译时自动解析

### 更新 ascend-common

如需更新到新版本：

```bash
# 1. 克隆最新版本
cd /tmp
git clone https://gitcode.com/Ascend/mind-cluster.git --depth 1

# 2. 备份当前版本（可选）
cd /path/to/categraf
mv pkg/ascend-common pkg/ascend-common.bak

# 3. 拷贝新版本
cp -r /tmp/mind-cluster/component/ascend-common pkg/

# 4. 更新依赖
go mod tidy

# 5. 测试
go build
./categraf --test --inputs npu

# 6. 清理
rm -rf /tmp/mind-cluster
```

### 代码结构

- `npu.go`: Categraf Input 接口实现
- `collector.go`: NPU 设备发现和指标采集
- `types.go`: NPU 设备和指标数据结构
- `cache.go`: 线程安全的内存缓存（TTL）
- `converter.go`: 指标格式转换（内部 → Categraf）

## 监控示例

### Prometheus 查询示例

```promql
# NPU 温度
npu_temperature

# NPU 利用率
npu_utilization

# NPU 内存使用率
npu_hbm_utilization

# 高温告警（>80°C）
npu_temperature > 80

# 高利用率设备（>90%）
npu_utilization > 90

# 内存不足告警（>95%）
npu_hbm_utilization > 95
```

### Grafana Dashboard 建议

1. **概览面板**：总设备数、在线状态、平均利用率
2. **性能面板**：各设备利用率、温度、功耗
3. **内存面板**：HBM/DDR 使用情况
4. **健康面板**：健康状态、错误码、网络状态
5. **进程面板**：进程数、进程内存使用

## 参考资源

- [Huawei Ascend 官方文档](https://www.hiascend.com/)
- [Ascend mind-cluster 仓库](https://gitcode.com/Ascend/mind-cluster)
- [ascend-common 组件](https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-common)
- [Categraf 文档](https://github.com/flashcatcloud/categraf)
- [SDK 文档](../pkg/ascend-common/README.md)

## 版本历史

### v1.0.0 (2025-11-15)

**初始发布**
- ✅ 基础 NPU 指标采集
- ✅ 支持 Ascend 910A/910B/310P
- ✅ 后台采集与缓存机制
- ✅ 健康状态和错误监控
- ✅ 内存指标（HBM/DDR）
- ✅ 进程跟踪
- ✅ ascend-common SDK 本地化集成
- ✅ 支持 24+ 指标类型

**核心代码：** 863 行  
**文档：** 1500+ 行

## License

本插件是 Categraf 的一部分，遵循相同的许可证条款。

## 贡献

欢迎贡献！请：
1. 使用真实 NPU 硬件进行测试
2. 遵循现有代码风格
3. 为新功能添加测试
4. 更新文档
