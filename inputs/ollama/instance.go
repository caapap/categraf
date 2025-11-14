package ollama

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"flashcat.cloud/categraf/config"
	"flashcat.cloud/categraf/types"
)

type Instance struct {
	config.InstanceConfig

	// Ollama 配置
	OllamaURL       string          `toml:"ollama_url"`
	Timeout         config.Duration `toml:"timeout"`
	ProxyListenAddr string          `toml:"proxy_listen_addr"` // 代理监听地址

	// HTTP 客户端
	client *http.Client
	ctx    context.Context
	cancel context.CancelFunc

	// 内部状态
	initialized bool

	// 指标存储
	metrics *MetricsStore

	// 代理服务器
	proxyServer *http.Server
}

// Init 初始化实例
func (ins *Instance) Init() error {
	if ins.OllamaURL == "" {
		ins.OllamaURL = "http://localhost:11434"
	}

	if ins.Timeout <= 0 {
		ins.Timeout = config.Duration(time.Second * 30)
	}

	if ins.ProxyListenAddr == "" {
		ins.ProxyListenAddr = ":8000"
	}

	// 创建上下文
	ins.ctx, ins.cancel = context.WithCancel(context.Background())

	// 创建 HTTP 客户端
	ins.client = &http.Client{
		Timeout: time.Duration(ins.Timeout),
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// 初始化指标存储
	ins.metrics = NewMetricsStore()

	// 验证 Ollama 连接
	if err := ins.verifyConnection(); err != nil {
		log.Printf("W! [ollama] Failed to connect to Ollama at %s: %v", ins.OllamaURL, err)
		log.Printf("W! [ollama] Will retry on proxy requests")
		// 不返回错误，允许延迟连接
	} else {
		log.Printf("I! [ollama] Successfully connected to Ollama at %s", ins.OllamaURL)
	}

	// 启动代理服务器
	if err := ins.startProxy(); err != nil {
		return fmt.Errorf("failed to start proxy: %w", err)
	}

	ins.initialized = true
	log.Printf("I! [ollama] Plugin initialized successfully (proxy=%s, backend=%s)", ins.ProxyListenAddr, ins.OllamaURL)
	return nil
}

// Drop 清理资源
func (ins *Instance) Drop() {
	if ins.cancel != nil {
		ins.cancel()
	}

	if ins.proxyServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := ins.proxyServer.Shutdown(ctx); err != nil {
			log.Printf("E! [ollama] Error shutting down proxy server: %v", err)
		} else {
			log.Printf("I! [ollama] Proxy server shut down successfully")
		}
	}
}

// Gather 采集指标（定期调用）
func (ins *Instance) Gather(slist *types.SampleList) {
	if !ins.initialized {
		return
	}

	// 采集被动收集的指标
	ins.gatherMetrics(slist)
}

// gatherMetrics 采集所有指标
func (ins *Instance) gatherMetrics(slist *types.SampleList) {
	// 1. 请求总数
	for model, count := range ins.metrics.GetRequestCount() {
		labels := ins.makeLabels(map[string]string{"model": model})
		slist.PushSample(inputName, "requests_total", count, labels)
	}

	// 2. 成功请求数
	for model, count := range ins.metrics.GetSuccessCount() {
		labels := ins.makeLabels(map[string]string{"model": model})
		slist.PushSample(inputName, "requests_success", count, labels)
	}

	// 3. 失败请求数
	for model, count := range ins.metrics.GetFailureCount() {
		labels := ins.makeLabels(map[string]string{"model": model})
		slist.PushSample(inputName, "requests_failed", count, labels)
	}

	// 4. Token 总数
	for model, count := range ins.metrics.GetTokenCount() {
		labels := ins.makeLabels(map[string]string{"model": model})
		slist.PushSample(inputName, "tokens_generated_total", count, labels)
	}

	// 5. Prompt tokens
	for model, count := range ins.metrics.GetPromptTokens() {
		labels := ins.makeLabels(map[string]string{"model": model})
		slist.PushSample(inputName, "prompt_tokens_total", count, labels)
	}

	// 6. 响应时间统计
	for model, stats := range ins.metrics.GetDurationStats() {
		// P50
		labels50 := ins.makeLabels(map[string]string{"model": model, "quantile": "0.5"})
		slist.PushSample(inputName, "response_duration_seconds", stats.P50, labels50)

		// P95
		labels95 := ins.makeLabels(map[string]string{"model": model, "quantile": "0.95"})
		slist.PushSample(inputName, "response_duration_seconds", stats.P95, labels95)

		// P99
		labels99 := ins.makeLabels(map[string]string{"model": model, "quantile": "0.99"})
		slist.PushSample(inputName, "response_duration_seconds", stats.P99, labels99)

		// Avg
		labelsAvg := ins.makeLabels(map[string]string{"model": model, "quantile": "avg"})
		slist.PushSample(inputName, "response_duration_seconds", stats.Avg, labelsAvg)

		// Min/Max
		labelsMin := ins.makeLabels(map[string]string{"model": model, "quantile": "min"})
		slist.PushSample(inputName, "response_duration_seconds", stats.Min, labelsMin)

		labelsMax := ins.makeLabels(map[string]string{"model": model, "quantile": "max"})
		slist.PushSample(inputName, "response_duration_seconds", stats.Max, labelsMax)
	}

	// 7. 模型加载时间统计
	for model, stats := range ins.metrics.GetLoadDurationStats() {
		// P50
		labels50 := ins.makeLabels(map[string]string{"model": model, "quantile": "0.5"})
		slist.PushSample(inputName, "model_load_duration_seconds", stats.P50, labels50)

		// P95
		labels95 := ins.makeLabels(map[string]string{"model": model, "quantile": "0.95"})
		slist.PushSample(inputName, "model_load_duration_seconds", stats.P95, labels95)

		// Avg
		labelsAvg := ins.makeLabels(map[string]string{"model": model, "quantile": "avg"})
		slist.PushSample(inputName, "model_load_duration_seconds", stats.Avg, labelsAvg)
	}
}

// makeLabels 合并实例标签和额外标签
func (ins *Instance) makeLabels(extra map[string]string) map[string]string {
	labels := make(map[string]string)
	for k, v := range ins.Labels {
		labels[k] = v
	}
	for k, v := range extra {
		labels[k] = v
	}
	return labels
}

// verifyConnection 验证 Ollama 连接
func (ins *Instance) verifyConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ins.Timeout))
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", ins.OllamaURL+"/api/version", nil)
	if err != nil {
		return err
	}

	resp, err := ins.client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
