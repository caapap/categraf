# Spark Streaming Input Plugin

## 概述

Spark Streaming 插件用于监控运行在 Apache YARN 上的 Spark Streaming 应用程序。该插件通过 YARN ResourceManager API 获取运行中的应用列表，然后解析每个 Spark Streaming 应用的 Web UI 页面来提取关键性能指标。

## 功能特性

- ✅ 自动发现 YARN 上运行的 Spark Streaming 应用
- ✅ 支持应用名称前缀过滤
- ✅ 并发采集多个应用的指标
- ✅ 完整的流式处理性能指标
- ✅ 支持多 YARN 集群监控
- ⏳ Kerberos 认证支持（计划中）

## 配置示例

```toml
[[instances]]
# YARN ResourceManager 地址
yarn_address = "10.130.1.117:8088"

# 监控的应用前缀
app_prefixes = [
    "dhzp-strategy",
    "dhzp-asynclean",
    "dhzp-recognition"
]

# 采集间隔
interval = "60s"

# 自定义标签
[instances.labels]
cluster = "production"
env = "prod"
```

## 采集指标

### 1. 连接状态指标

| 指标名称 | 类型 | 说明 | 标签 |
|---------|------|------|------|
| `spark_streaming_yarn_up` | Gauge | YARN 连接状态 (1=正常, 0=异常) | `yarn_address` |

### 2. 应用状态指标

| 指标名称 | 类型 | 说明 | 标签 |
|---------|------|------|------|
| `spark_streaming_yarn_run_app` | Gauge | 运行中的应用 (值恒为1) | `app_id`, `app_name`, `state`, `final_status`, `user`, `queue` |
| `spark_streaming_scrape_error` | Gauge | 抓取错误标志 (仅在出错时存在) | `app_id`, `app_name` |

### 3. 流式处理性能指标

| 指标名称 | 类型 | 单位 | 说明 | 标签 |
|---------|------|------|------|------|
| `spark_streaming_input_rate_records_avg_stat` | Gauge | records/sec | 平均输入记录速率 | `app_id`, `app_name` |
| `spark_streaming_batches_scheduling_delay_avg_stat` | Gauge | ms | 平均批次调度延迟 | `app_id`, `app_name` |
| `spark_streaming_batches_processing_time_avg_stat` | Gauge | ms | 平均批次处理时间 | `app_id`, `app_name` |
| `spark_streaming_batches_total_delay_avg_stat` | Gauge | ms | 平均批次总延迟 | `app_id`, `app_name` |
| `spark_streaming_running_batches_current_num` | Gauge | count | 当前运行中的批次数量 | `app_id`, `app_name` |
| `spark_streaming_waiting_batches_current_num` | Gauge | count | 当前等待中的批次数量 | `app_id`, `app_name` |
| `spark_streaming_completed_batches_current_num` | Gauge | count | 已完成的批次数量 | `app_id`, `app_name` |

## 工作原理

```
┌─────────────────────────────────────────────────────────────┐
│                     Categraf 插件流程                         │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
         1. 调用 YARN ResourceManager API
            GET http://yarn:8088/ws/v1/cluster/apps
                           │
                           ▼
         2. 过滤运行中的 Spark Streaming 应用
            (根据 app_prefixes 配置)
                           │
                           ▼
         3. 并发访问每个应用的 Streaming UI
            GET http://trackingUrl/streaming/
                           │
                           ▼
         4. 解析 HTML 页面提取指标
            - 解析统计表格 (#stat-table)
            - 解析批次计数 (#runningBatches, etc.)
                           │
                           ▼
         5. 推送指标到 Categraf 队列
            - 添加自定义标签
            - 转换时间单位到毫秒
                           │
                           ▼
         6. Categraf 批量推送到后端
            (Prometheus Remote Write)
```

## 配置参数详解

### 必需参数

| 参数 | 类型 | 说明 | 示例 |
|-----|------|------|------|
| `yarn_address` | string | YARN ResourceManager 地址 | `"10.130.1.117:8088"` |

### 可选参数

| 参数 | 类型 | 默认值 | 说明 |
|-----|------|--------|------|
| `yarn_app_api_path` | string | `/ws/v1/cluster/apps` | YARN API 路径 |
| `app_prefixes` | []string | `[]` (监控所有) | 应用名称前缀过滤列表 |
| `timeout` | duration | `10s` | HTTP 请求超时时间 |
| `max_concurrency` | int | `10` | 最大并发抓取应用数 |
| `skip_verify` | bool | `false` | 跳过 TLS 证书验证 |
| `interval` | duration | 全局配置 | 采集间隔 |

### 标签配置

通过 `[instances.labels]` 可以为所有指标添加自定义标签：

```toml
[instances.labels]
cluster = "production"
region = "beijing"
env = "prod"
datacenter = "dc1"
```

## 使用场景

### 1. 单集群监控

```toml
[[instances]]
yarn_address = "yarn-master:8088"
app_prefixes = ["myapp-"]
interval = "30s"

[instances.labels]
cluster = "prod-cluster-1"
```

### 2. 多集群监控

```toml
# 生产集群
[[instances]]
yarn_address = "prod-yarn:8088"
app_prefixes = ["prod-"]
[instances.labels]
cluster = "production"

# 测试集群
[[instances]]
yarn_address = "test-yarn:8088"
app_prefixes = ["test-"]
[instances.labels]
cluster = "staging"
```

### 3. 监控所有应用（不过滤）

```toml
[[instances]]
yarn_address = "yarn-master:8088"
# 不设置 app_prefixes，监控所有运行中的应用
```

## 告警规则示例

### PromQL 查询示例

```promql
# 1. 高调度延迟告警（超过 1 秒）
spark_streaming_batches_scheduling_delay_avg_stat > 1000

# 2. 高处理时间告警（超过 5 秒）
spark_streaming_batches_processing_time_avg_stat > 5000

# 3. 等待批次堆积告警（超过 10 个）
spark_streaming_waiting_batches_current_num > 10

# 4. 输入速率下降告警（低于 100 records/sec）
spark_streaming_input_rate_records_avg_stat < 100

# 5. YARN 连接失败告警
spark_streaming_yarn_up == 0

# 6. 应用抓取失败告警
spark_streaming_scrape_error > 0
```

### Prometheus 告警规则

```yaml
groups:
  - name: spark_streaming
    interval: 30s
    rules:
      - alert: SparkStreamingHighSchedulingDelay
        expr: spark_streaming_batches_scheduling_delay_avg_stat > 1000
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Spark Streaming 调度延迟过高"
          description: "应用 {{ $labels.app_name }} 的平均调度延迟为 {{ $value }}ms"

      - alert: SparkStreamingBatchBacklog
        expr: spark_streaming_waiting_batches_current_num > 10
        for: 3m
        labels:
          severity: critical
        annotations:
          summary: "Spark Streaming 批次堆积"
          description: "应用 {{ $labels.app_name }} 有 {{ $value }} 个等待批次"

      - alert: SparkStreamingYARNDown
        expr: spark_streaming_yarn_up == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "YARN 连接失败"
          description: "无法连接到 YARN ResourceManager {{ $labels.yarn_address }}"
```

## Grafana 仪表盘

### 推荐面板

1. **应用概览**
   - 运行中的应用数量
   - YARN 连接状态
   - 抓取错误统计

2. **性能指标**
   - 输入速率时序图
   - 调度延迟时序图
   - 处理时间时序图
   - 总延迟时序图

3. **批次状态**
   - 运行/等待/完成批次数量堆叠图
   - 批次处理效率（处理时间 vs 调度延迟）

4. **应用排行**
   - Top 5 高延迟应用
   - Top 5 高负载应用（按输入速率）

### Grafana 查询示例

```
# 应用数量
count(spark_streaming_yarn_run_app)

# 平均输入速率
avg(spark_streaming_input_rate_records_avg_stat) by (app_name)

# 批次处理总览
sum(spark_streaming_running_batches_current_num) by (cluster)
sum(spark_streaming_waiting_batches_current_num) by (cluster)
sum(spark_streaming_completed_batches_current_num) by (cluster)
```

## 故障排查

### 问题 1：无法连接 YARN

**现象**：日志显示 "Failed to connect to YARN"

**解决方法**：
1. 检查 `yarn_address` 配置是否正确
2. 确认网络连通性：`curl http://yarn-address:8088/ws/v1/cluster/info`
3. 检查防火墙规则

### 问题 2：无法解析 Spark UI

**现象**：日志显示 "Failed to get metrics for app"

**解决方法**：
1. 确认应用的 `trackingUrl` 可访问
2. 检查 Spark 版本兼容性（支持 2.4+ / 3.x）
3. 手动访问 `trackingUrl/streaming/` 检查页面结构

### 问题 3：应用未被监控

**现象**：配置了 `app_prefixes` 但没有采集到指标

**解决方法**：
1. 检查应用名称是否匹配前缀
2. 确认应用状态为 RUNNING
3. 查看日志中的 "Matched app" 信息

### 问题 4：性能问题

**现象**：采集时间过长

**解决方法**：
1. 减少 `app_prefixes` 过滤范围
2. 增加 `max_concurrency` 并发数
3. 增加 `timeout` 超时时间
4. 增加 `interval` 采集间隔

## 性能优化

### 1. 并发控制

默认最多并发抓取 10 个应用，可根据实际情况调整：

```toml
max_concurrency = 20  # 增加并发数
```

### 2. 超时配置

根据网络状况调整超时时间：

```toml
timeout = "15s"  # 增加超时时间
```

### 3. 采集间隔

建议根据应用数量调整：

```toml
# 应用较少（1-10个）
interval = "30s"

# 应用较多（10-50个）
interval = "60s"

# 应用很多（50+个）
interval = "120s"
```

## 限制和注意事项

1. **Spark 版本兼容性**：
   - 支持 Spark 2.4.0+ 和 3.x
   - 不同版本的 UI 结构可能略有差异

2. **YARN 版本兼容性**：
   - 支持 YARN 2.7.0+ 和 3.x
   - 依赖 YARN REST API v1

3. **Kerberos 认证**：
   - 当前版本暂不支持
   - 计划在后续版本中实现

4. **资源消耗**：
   - 每个应用采集需要 1 个 HTTP 请求到 YARN + 1 个到 Spark UI
   - 100 个应用大约需要 200 个 HTTP 请求/周期
   - 建议合理设置采集间隔

5. **HTML 解析依赖**：
   - 依赖 Spark Streaming UI 的 HTML 结构
   - 如果 Spark 版本大幅升级，可能需要更新解析逻辑

## 开发信息

### 依赖库

```go
github.com/PuerkitoBio/goquery  // HTML 解析
```

### 文件结构

```
inputs/spark_streaming/
├── spark_streaming.go      # 插件注册
├── instance.go             # 采集逻辑
├── types.go                # 类型定义
├── yarn_client.go          # YARN API 客户端
├── spark_ui_parser.go      # Spark UI 解析器
└── README.md               # 文档
```

### 未来计划

- [ ] Kerberos 认证支持
- [ ] 支持 HTTPS (TLS)
- [ ] 更多的批次详细指标
- [ ] 支持 Spark Structured Streaming
- [ ] 支持直接从 Spark Metrics API 采集

## 许可证

与 Categraf 主项目相同

## 贡献

欢迎提交 Issue 和 Pull Request！

## 参考资料

- [Categraf 文档](https://github.com/flashcatcloud/categraf)
- [Spark Monitoring Guide](https://spark.apache.org/docs/latest/monitoring.html)
- [YARN REST API](https://hadoop.apache.org/docs/stable/hadoop-yarn/hadoop-yarn-site/ResourceManagerRest.html)

