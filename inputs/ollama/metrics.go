package ollama

import (
	"sort"
	"sync"
	"time"
)

// MetricsStore 存储被动采集的指标
type MetricsStore struct {
	mu              sync.RWMutex
	requestCount    map[string]int64     // model -> 请求总数
	requestSuccess  map[string]int64     // model -> 成功请求数
	requestFailed   map[string]int64     // model -> 失败请求数
	tokenCount      map[string]int64     // model -> token 总数
	promptTokens    map[string]int64     // model -> prompt tokens
	durationSamples map[string][]float64 // model -> 响应时间样本（秒）
	loadDurations   map[string][]float64 // model -> 模型加载时间（秒）
	lastUpdate      time.Time
}

// NewMetricsStore 创建新的指标存储
func NewMetricsStore() *MetricsStore {
	return &MetricsStore{
		requestCount:    make(map[string]int64),
		requestSuccess:  make(map[string]int64),
		requestFailed:   make(map[string]int64),
		tokenCount:      make(map[string]int64),
		promptTokens:    make(map[string]int64),
		durationSamples: make(map[string][]float64),
		loadDurations:   make(map[string][]float64),
		lastUpdate:      time.Now(),
	}
}

// RecordSuccess 记录成功的请求
func (m *MetricsStore) RecordSuccess(model string, resp *OllamaResponse, duration float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requestCount[model]++
	m.requestSuccess[model]++
	m.tokenCount[model] += resp.EvalCount
	m.promptTokens[model] += resp.PromptEvalCount

	// 记录响应时间
	m.durationSamples[model] = append(m.durationSamples[model], duration)
	m.limitSamples(m.durationSamples, model)

	// 记录模型加载时间（纳秒转秒）
	if resp.LoadDuration > 0 {
		loadDur := float64(resp.LoadDuration) / 1e9
		m.loadDurations[model] = append(m.loadDurations[model], loadDur)
		m.limitSamples(m.loadDurations, model)
	}

	m.lastUpdate = time.Now()
}

// RecordFailure 记录失败的请求
func (m *MetricsStore) RecordFailure(model string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.requestCount[model]++
	m.requestFailed[model]++
	m.lastUpdate = time.Now()
}

// limitSamples 限制样本数量，避免内存无限增长
func (m *MetricsStore) limitSamples(samples map[string][]float64, model string) {
	maxSamples := 1000
	if len(samples[model]) > maxSamples {
		// 保留最近的一半样本
		samples[model] = samples[model][maxSamples/2:]
	}
}

// GetRequestCount 获取请求总数
func (m *MetricsStore) GetRequestCount() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int64)
	for k, v := range m.requestCount {
		result[k] = v
	}
	return result
}

// GetSuccessCount 获取成功请求数
func (m *MetricsStore) GetSuccessCount() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int64)
	for k, v := range m.requestSuccess {
		result[k] = v
	}
	return result
}

// GetFailureCount 获取失败请求数
func (m *MetricsStore) GetFailureCount() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int64)
	for k, v := range m.requestFailed {
		result[k] = v
	}
	return result
}

// GetTokenCount 获取 token 总数
func (m *MetricsStore) GetTokenCount() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int64)
	for k, v := range m.tokenCount {
		result[k] = v
	}
	return result
}

// GetPromptTokens 获取 prompt tokens
func (m *MetricsStore) GetPromptTokens() map[string]int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]int64)
	for k, v := range m.promptTokens {
		result[k] = v
	}
	return result
}

// GetDurationStats 获取响应时间统计
func (m *MetricsStore) GetDurationStats() map[string]*Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Stats)
	for model, samples := range m.durationSamples {
		if len(samples) > 0 {
			result[model] = calculateStats(samples)
		}
	}
	return result
}

// GetLoadDurationStats 获取加载时间统计
func (m *MetricsStore) GetLoadDurationStats() map[string]*Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*Stats)
	for model, samples := range m.loadDurations {
		if len(samples) > 0 {
			result[model] = calculateStats(samples)
		}
	}
	return result
}

// Stats 统计数据
type Stats struct {
	Avg float64
	P50 float64
	P95 float64
	P99 float64
	Min float64
	Max float64
}

// calculateStats 计算统计值
func calculateStats(values []float64) *Stats {
	if len(values) == 0 {
		return &Stats{}
	}

	// 复制并排序
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// 计算平均值
	var sum float64
	for _, v := range sorted {
		sum += v
	}
	avg := sum / float64(len(sorted))

	// 计算百分位数
	p50 := sorted[len(sorted)*50/100]
	p95 := sorted[len(sorted)*95/100]
	p99 := sorted[len(sorted)*99/100]

	return &Stats{
		Avg: avg,
		P50: p50,
		P95: p95,
		P99: p99,
		Min: sorted[0],
		Max: sorted[len(sorted)-1],
	}
}

