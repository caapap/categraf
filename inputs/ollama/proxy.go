package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// startProxy 启动 HTTP 代理服务器
func (ins *Instance) startProxy() error {
	if ins.ProxyListenAddr == "" {
		ins.ProxyListenAddr = ":8000" // 默认端口
	}

	mux := http.NewServeMux()

	// 代理 /api/chat 和 /api/generate（需要记录指标）
	mux.HandleFunc("/api/chat", ins.proxyWithMetrics)
	mux.HandleFunc("/api/generate", ins.proxyWithMetrics)

	// 其他请求直接转发（不记录指标）
	mux.HandleFunc("/", ins.simpleProxy)

	// 暴露 /metrics 端点（兼容 Prometheus）
	mux.HandleFunc("/metrics", ins.serveMetrics)

	// 健康检查端点
	mux.HandleFunc("/health", ins.healthCheck)

	ins.proxyServer = &http.Server{
		Addr:    ins.ProxyListenAddr,
		Handler: mux,
	}

	go func() {
		log.Printf("I! [ollama] Starting proxy server on %s", ins.ProxyListenAddr)
		if err := ins.proxyServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("E! [ollama] Proxy server error: %v", err)
		}
	}()

	return nil
}

// proxyWithMetrics 带指标记录的代理（处理 /api/chat 和 /api/generate）
func (ins *Instance) proxyWithMetrics(w http.ResponseWriter, r *http.Request) {
	// 读取请求体
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("E! [ollama] Failed to read request body: %v", err)
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	// 解析请求，提取模型名称
	var reqBody OllamaRequest
	if err := json.Unmarshal(bodyBytes, &reqBody); err != nil {
		log.Printf("E! [ollama] Failed to parse request: %v", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	model := reqBody.Model
	if model == "" {
		model = "unknown"
	}

	// 记录请求开始时间
	startTime := time.Now()

	// 创建代理请求
	proxyReq, err := http.NewRequestWithContext(ins.ctx, r.Method, ins.OllamaURL+r.URL.Path, bytes.NewBuffer(bodyBytes))
	if err != nil {
		log.Printf("E! [ollama] Failed to create proxy request: %v", err)
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// 复制请求头
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// 发送请求
	resp, err := ins.client.Do(proxyReq)
	if err != nil {
		log.Printf("E! [ollama] Proxy request failed: %v", err)
		ins.metrics.RecordFailure(model)
		http.Error(w, "Proxy request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// 检查是否为流式响应
	isStream := reqBody.Stream
	if isStream {
		// 流式响应处理
		ins.handleStreamResponse(w, resp, model, startTime)
	} else {
		// 非流式响应处理
		ins.handleNonStreamResponse(w, resp, model, startTime)
	}
}

// handleNonStreamResponse 处理非流式响应
func (ins *Instance) handleNonStreamResponse(w http.ResponseWriter, resp *http.Response, model string, startTime time.Time) {
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("E! [ollama] Failed to read response: %v", err)
		ins.metrics.RecordFailure(model)
		return
	}

	// 解析响应，提取指标
	var ollamaResp OllamaResponse
	if err := json.Unmarshal(respBytes, &ollamaResp); err != nil {
		log.Printf("E! [ollama] Failed to parse response: %v", err)
		ins.metrics.RecordFailure(model)
	} else {
		// 记录成功指标
		duration := time.Since(startTime).Seconds()
		ins.metrics.RecordSuccess(model, &ollamaResp, duration)
	}

	// 返回响应给客户端
	w.Write(respBytes)
}

// handleStreamResponse 处理流式响应
func (ins *Instance) handleStreamResponse(w http.ResponseWriter, resp *http.Response, model string, startTime time.Time) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Printf("E! [ollama] Response writer doesn't support flushing")
		ins.metrics.RecordFailure(model)
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	var lastChunk StreamChunk
	hasError := false

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// 写入原始数据到客户端
		w.Write(line)
		w.Write([]byte("\n"))
		flusher.Flush()

		// 解析最后一个 chunk 以获取指标
		var chunk StreamChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			log.Printf("W! [ollama] Failed to parse stream chunk: %v", err)
			hasError = true
			continue
		}

		if chunk.Done {
			lastChunk = chunk
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("E! [ollama] Error reading stream: %v", err)
		hasError = true
	}

	// 记录指标
	if hasError {
		ins.metrics.RecordFailure(model)
	} else if lastChunk.Done {
		// 转换为 OllamaResponse 格式
		ollamaResp := &OllamaResponse{
			Model:              lastChunk.Model,
			TotalDuration:      lastChunk.TotalDuration,
			LoadDuration:       lastChunk.LoadDuration,
			PromptEvalCount:    lastChunk.PromptEvalCount,
			PromptEvalDuration: lastChunk.PromptEvalDuration,
			EvalCount:          lastChunk.EvalCount,
			EvalDuration:       lastChunk.EvalDuration,
			Done:               lastChunk.Done,
		}
		duration := time.Since(startTime).Seconds()
		ins.metrics.RecordSuccess(model, ollamaResp, duration)
	}
}

// simpleProxy 简单代理（不记录指标）
func (ins *Instance) simpleProxy(w http.ResponseWriter, r *http.Request) {
	// 读取请求体
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	r.Body.Close()

	// 创建代理请求
	proxyReq, err := http.NewRequestWithContext(ins.ctx, r.Method, ins.OllamaURL+r.URL.Path, bytes.NewBuffer(bodyBytes))
	if err != nil {
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	// 复制请求头
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// 发送请求
	resp, err := ins.client.Do(proxyReq)
	if err != nil {
		http.Error(w, "Proxy request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 复制响应头
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// 复制响应体
	io.Copy(w, resp.Body)
}

// serveMetrics 暴露 Prometheus 格式的指标
func (ins *Instance) serveMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	var output strings.Builder

	// 请求总数
	output.WriteString("# HELP ollama_requests_total Total number of requests\n")
	output.WriteString("# TYPE ollama_requests_total counter\n")
	for model, count := range ins.metrics.GetRequestCount() {
		fmt.Fprintf(&output, "ollama_requests_total{model=\"%s\"} %d\n", model, count)
	}

	// 成功请求数
	output.WriteString("# HELP ollama_requests_success Total number of successful requests\n")
	output.WriteString("# TYPE ollama_requests_success counter\n")
	for model, count := range ins.metrics.GetSuccessCount() {
		fmt.Fprintf(&output, "ollama_requests_success{model=\"%s\"} %d\n", model, count)
	}

	// 失败请求数
	output.WriteString("# HELP ollama_requests_failed Total number of failed requests\n")
	output.WriteString("# TYPE ollama_requests_failed counter\n")
	for model, count := range ins.metrics.GetFailureCount() {
		fmt.Fprintf(&output, "ollama_requests_failed{model=\"%s\"} %d\n", model, count)
	}

	// Token 总数
	output.WriteString("# HELP ollama_tokens_generated_total Total number of tokens generated\n")
	output.WriteString("# TYPE ollama_tokens_generated_total counter\n")
	for model, count := range ins.metrics.GetTokenCount() {
		fmt.Fprintf(&output, "ollama_tokens_generated_total{model=\"%s\"} %d\n", model, count)
	}

	// Prompt tokens
	output.WriteString("# HELP ollama_prompt_tokens_total Total number of prompt tokens\n")
	output.WriteString("# TYPE ollama_prompt_tokens_total counter\n")
	for model, count := range ins.metrics.GetPromptTokens() {
		fmt.Fprintf(&output, "ollama_prompt_tokens_total{model=\"%s\"} %d\n", model, count)
	}

	// 响应时间统计
	output.WriteString("# HELP ollama_response_duration_seconds Response duration in seconds\n")
	output.WriteString("# TYPE ollama_response_duration_seconds gauge\n")
	for model, stats := range ins.metrics.GetDurationStats() {
		fmt.Fprintf(&output, "ollama_response_duration_seconds{model=\"%s\",quantile=\"0.5\"} %.6f\n", model, stats.P50)
		fmt.Fprintf(&output, "ollama_response_duration_seconds{model=\"%s\",quantile=\"0.95\"} %.6f\n", model, stats.P95)
		fmt.Fprintf(&output, "ollama_response_duration_seconds{model=\"%s\",quantile=\"0.99\"} %.6f\n", model, stats.P99)
		fmt.Fprintf(&output, "ollama_response_duration_seconds{model=\"%s\",quantile=\"avg\"} %.6f\n", model, stats.Avg)
	}

	// 模型加载时间统计
	output.WriteString("# HELP ollama_model_load_duration_seconds Model load duration in seconds\n")
	output.WriteString("# TYPE ollama_model_load_duration_seconds gauge\n")
	for model, stats := range ins.metrics.GetLoadDurationStats() {
		fmt.Fprintf(&output, "ollama_model_load_duration_seconds{model=\"%s\",quantile=\"0.5\"} %.6f\n", model, stats.P50)
		fmt.Fprintf(&output, "ollama_model_load_duration_seconds{model=\"%s\",quantile=\"0.95\"} %.6f\n", model, stats.P95)
		fmt.Fprintf(&output, "ollama_model_load_duration_seconds{model=\"%s\",quantile=\"avg\"} %.6f\n", model, stats.Avg)
	}

	w.Write([]byte(output.String()))
}

// healthCheck 健康检查端点
func (ins *Instance) healthCheck(w http.ResponseWriter, r *http.Request) {
	// 检查 Ollama 后端是否可达
	ctx, cancel := context.WithTimeout(ins.ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", ins.OllamaURL+"/api/version", nil)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "ERROR: %v\n", err)
		return
	}

	resp, err := ins.client.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "ERROR: Ollama backend unreachable: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprintf(w, "ERROR: Ollama backend returned status %d\n", resp.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK\n")
}
