# Ollama Monitoring Plugin for Categraf

纯被动采集模式的 Ollama 监控插件，通过 HTTP 代理拦截所有 Ollama API 请求来采集指标。

## 功能特性

- ✅ **纯被动采集**：作为透明代理，无额外请求负载
- ✅ **实时指标**：捕获每个真实请求的详细指标
- ✅ **流式支持**：支持流式和非流式响应
- ✅ **完整指标**：请求数、Token 消耗、响应时间、成功率等
- ✅ **多模型支持**：自动识别并按模型分组统计
- ✅ **Prometheus 兼容**：同时暴露 `/metrics` 端点

## 工作原理

```
客户端 → Categraf Ollama Plugin (:8000) → Ollama Backend (:11434)
              ↓
         采集指标
              ↓
      推送到 Categraf 后端
```

## 配置说明

### 基础配置

```toml
[[instances]]
# Ollama 后端地址
ollama_url = "http://172.31.101.238:11434"

# 代理监听地址
proxy_listen_addr = ":8000"

# HTTP 超时
timeout = "30s"

# 采集间隔
interval = "15s"

# 自定义标签
[instances.labels]
service = "ollama"
env = "production"
```

### 配置参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `ollama_url` | string | `http://localhost:11434` | Ollama 后端服务地址 |
| `proxy_listen_addr` | string | `:8000` | 代理监听地址 |
| `timeout` | duration | `30s` | HTTP 请求超时时间 |
| `interval` | duration | `15s` | 指标推送间隔 |
| `labels` | map | - | 自定义标签 |

## 采集的指标

### 1. 请求计数指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `ollama_requests_total` | Counter | model | 请求总数 |
| `ollama_requests_success` | Counter | model | 成功请求数 |
| `ollama_requests_failed` | Counter | model | 失败请求数 |

### 2. Token 指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `ollama_tokens_generated_total` | Counter | model | 生成的 Token 总数 |
| `ollama_prompt_tokens_total` | Counter | model | Prompt Token 总数 |

### 3. 响应时间指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `ollama_response_duration_seconds` | Gauge | model, quantile | 响应时间（支持 P50/P95/P99/avg/min/max） |
| `ollama_model_load_duration_seconds` | Gauge | model, quantile | 模型加载时间（支持 P50/P95/avg） |

## 部署方式

### 1. 编译

```bash
cd /iflytek/dev/aiops/ollama-exporter/categraf
make build
```

### 2. 配置

```bash
# 编辑配置文件
vim conf/input.ollama/ollama.toml

# 修改 ollama_url 为实际的 Ollama 后端地址
# 修改 proxy_listen_addr 为代理监听地址（默认 :8000）
```

### 3. 运行

```bash
# 启动 Categraf
./categraf --configs conf

# 或者只测试 ollama 插件
./categraf --test --inputs ollama
```

### 4. 修改客户端配置

将客户端的 Ollama 连接地址修改为代理地址：

```bash
# 之前
export OLLAMA_HOST=http://172.31.101.238:11434

# 现在
export OLLAMA_HOST=http://localhost:8000
```

或在代码中：

```python
# 之前
client = ollama.Client(host='http://172.31.101.238:11434')

# 现在
client = ollama.Client(host='http://localhost:8000')
```

## API 端点

### 1. 代理端点（自动转发）

- `POST /api/chat` - Chat API（带指标记录）
- `POST /api/generate` - Generate API（带指标记录）
- `*` - 其他所有 Ollama API（透明转发）

### 2. 监控端点

- `GET /metrics` - Prometheus 格式的指标
- `GET /health` - 健康检查

## 使用示例

### 查看实时指标

```bash
# Prometheus 格式
curl http://localhost:8000/metrics

# 健康检查
curl http://localhost:8000/health
```

### 测试代理功能

```bash
# 通过代理调用 Ollama
curl http://localhost:8000/api/generate -d '{
  "model": "qwen2.5:0.5b",
  "prompt": "Why is the sky blue?"
}'

# 查看指标是否更新
curl http://localhost:8000/metrics | grep ollama_requests_total
```

## 监控面板

建议在 Grafana 中创建以下面板：

1. **请求统计**
   - 总请求数、成功率、错误率
   - 按模型分组的请求趋势

2. **Token 消耗**
   - 总 Token 数趋势
   - 按模型分组的 Token 消耗
   - Prompt tokens vs Generated tokens

3. **性能指标**
   - P95/P99 响应时间
   - 模型加载时间
   - 请求延迟分布

4. **模型使用情况**
   - 各模型的使用频率
   - 各模型的平均响应时间
   - 各模型的 Token 效率

## 告警规则示例

```yaml
# 响应时间过长
- alert: OllamaSlowResponse
  expr: ollama_response_duration_seconds{quantile="0.95"} > 10
  for: 5m
  annotations:
    summary: "Ollama response time is too slow"

# 错误率过高
- alert: OllamaHighErrorRate
  expr: rate(ollama_requests_failed[5m]) / rate(ollama_requests_total[5m]) > 0.1
  for: 5m
  annotations:
    summary: "Ollama error rate is too high"

# 后端不可用
- alert: OllamaBackendDown
  expr: up{job="ollama"} == 0
  for: 1m
  annotations:
    summary: "Ollama backend is down"
```

## 故障排查

### 1. 代理无法启动

```bash
# 检查端口是否被占用
netstat -tlnp | grep 8000

# 查看日志
tail -f logs/categraf.log | grep ollama
```

### 2. 后端连接失败

```bash
# 测试后端连接
curl http://172.31.101.238:11434/api/version

# 检查配置文件中的 ollama_url
vim conf/input.ollama/ollama.toml
```

### 3. 指标未上报

```bash
# 测试插件采集
./categraf --test --inputs ollama

# 检查是否有请求通过代理
curl http://localhost:8000/metrics
```

## 性能考虑

- **内存使用**：每个模型最多保留 1000 个响应时间样本
- **并发处理**：使用 HTTP Transport 连接池，默认 100 个空闲连接
- **数据推送**：按配置的 `interval` 批量推送指标
- **透明代理**：除指标记录外，对请求无额外延迟

## 与 Python Exporter 的对比

| 特性 | Python Exporter | Categraf Plugin |
|------|-----------------|-----------------|
| 语言 | Python | Go |
| 性能 | 中等 | 高 |
| 内存占用 | 50-100MB | 20-30MB |
| 部署 | 需要 Python 环境 | 单一二进制文件 |
| 推送模式 | Pull (Prometheus) | Push (Remote Write) |
| 配置复杂度 | 简单 | 简单 |

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

与 Categraf 保持一致

