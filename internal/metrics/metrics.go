package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"strconv"
	"time"
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

	// Database health metrics
	DatabaseUp = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fish_fry_database_up",
			Help: "Database connectivity state (1=up, 0=down)",
		},
	)

	DatabasePingDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "fish_fry_database_ping_duration_seconds",
			Help:    "Database ping duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.005, 2, 10),
		},
	)

	// Order lifecycle metrics
	OrdersCreatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fish_fry_orders_created_total",
			Help: "Total number of created orders",
		},
	)

	OrdersUpdatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fish_fry_orders_updated_total",
			Help: "Total number of updated orders",
		},
	)

	OrderStatusTransitionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "fish_fry_order_status_transitions_total",
			Help: "Total number of order status transitions",
		},
		[]string{"from", "to"},
	)

	OrderItemsPerOrder = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "fish_fry_order_items_per_order",
			Help:    "Distribution of item counts per order write",
			Buckets: prometheus.LinearBuckets(1, 1, 12),
		},
	)

	OrderValueDollars = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "fish_fry_order_value_dollars",
			Help:    "Distribution of order value in dollars",
			Buckets: []float64{5, 10, 15, 20, 30, 40, 50, 75, 100, 150, 200},
		},
	)

	// Session metrics
	ActiveSession = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fish_fry_active_session",
			Help: "Whether there is currently an active session (1=yes, 0=no)",
		},
	)

	SessionTimeRemainingSeconds = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fish_fry_session_time_remaining_seconds",
			Help: "Seconds remaining in the current active session",
		},
	)

	SessionsClosedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "fish_fry_sessions_closed_total",
			Help: "Total number of closed sessions",
		},
	)

	// Realtime transport metrics
	ActiveWebSocketClients = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fish_fry_websocket_clients",
			Help: "Current number of active WebSocket clients",
		},
	)
)

// RecordHTTPRequest records HTTP request metrics
func RecordHTTPRequest(method, path string, status int, duration float64) {
	HTTPRequestsTotal.WithLabelValues(method, path, strconv.Itoa(status)).Inc()
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

// RecordDatabasePing records database liveness and ping latency.
func RecordDatabasePing(up bool, duration float64) {
	if up {
		DatabaseUp.Set(1)
	} else {
		DatabaseUp.Set(0)
	}
	DatabasePingDuration.Observe(duration)
}

func RecordOrderCreated(itemCount int, orderValue float64) {
	OrdersCreatedTotal.Inc()
	OrderItemsPerOrder.Observe(float64(itemCount))
	OrderValueDollars.Observe(orderValue)
}

func RecordOrderUpdated(itemCount int, orderValue float64) {
	OrdersUpdatedTotal.Inc()
	OrderItemsPerOrder.Observe(float64(itemCount))
	OrderValueDollars.Observe(orderValue)
}

func RecordOrderStatusTransition(fromStatus, toStatus string) {
	OrderStatusTransitionsTotal.WithLabelValues(fromStatus, toStatus).Inc()
}

func RecordSessionState(active bool, expiresAt *time.Time) {
	if !active {
		ActiveSession.Set(0)
		SessionTimeRemainingSeconds.Set(0)
		return
	}

	ActiveSession.Set(1)
	if expiresAt == nil {
		SessionTimeRemainingSeconds.Set(0)
		return
	}

	remaining := expiresAt.Sub(time.Now()).Seconds()
	if remaining < 0 {
		remaining = 0
	}
	SessionTimeRemainingSeconds.Set(remaining)
}

func RecordSessionClosed() {
	SessionsClosedTotal.Inc()
}

func RecordWebSocketClients(count int) {
	ActiveWebSocketClients.Set(float64(count))
}
