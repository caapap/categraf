package spark_streaming

// YARNApp represents a YARN application
type YARNApp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	State       string `json:"state"`
	FinalStatus string `json:"finalStatus"`
	TrackingURL string `json:"trackingUrl"`
	User        string `json:"user"`
	Queue       string `json:"queue"`
}

// YARNAppsResponse represents the response from YARN API
type YARNAppsResponse struct {
	Apps struct {
		App []YARNApp `json:"app"`
	} `json:"apps"`
}

// SparkStreamingMetrics represents the metrics collected from Spark Streaming UI
type SparkStreamingMetrics struct {
	// Input Rate (records/sec)
	InputRateAvg float64

	// Batch Delays (milliseconds)
	SchedulingDelayAvg float64
	ProcessingTimeAvg  float64
	TotalDelayAvg      float64

	// Batch Counts
	RunningBatches   int64
	WaitingBatches   int64
	CompletedBatches int64
}

// NewSparkStreamingMetrics creates a new SparkStreamingMetrics instance with zero values
func NewSparkStreamingMetrics() *SparkStreamingMetrics {
	return &SparkStreamingMetrics{
		InputRateAvg:       0,
		SchedulingDelayAvg: 0,
		ProcessingTimeAvg:  0,
		TotalDelayAvg:      0,
		RunningBatches:     0,
		WaitingBatches:     0,
		CompletedBatches:   0,
	}
}
