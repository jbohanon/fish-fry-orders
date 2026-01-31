package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP metrics
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		},
		[]string{"method", "path"},
	)

	// Order metrics
	OrderStatus = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "order_status",
			Help: "Current status of orders",
		},
		[]string{"status"},
	)

	OrderCompletionTime = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "order_completion_time_seconds",
			Help:    "Time taken to complete orders",
			Buckets: prometheus.ExponentialBuckets(1, 2, 10),
		},
	)

	// Chat metrics
	ChatMessagesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "chat_messages_total",
			Help: "Total number of chat messages",
		},
	)
)

// RecordHTTPRequest records HTTP request metrics
func RecordHTTPRequest(method, path string, status int, duration float64) {
	HTTPRequestsTotal.WithLabelValues(method, path, string(status)).Inc()
	HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
}

// RecordOrderStatus records order status metrics
func RecordOrderStatus(status string) {
	OrderStatus.WithLabelValues(status).Inc()
}

// RecordOrderCompletionTime records order completion time metrics
func RecordOrderCompletionTime(duration float64) {
	OrderCompletionTime.Observe(duration)
}

// RecordChatMessage records chat message metrics
func RecordChatMessage() {
	ChatMessagesTotal.Inc()
}
