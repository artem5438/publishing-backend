package api

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPRequestsTotal — счётчик HTTP-запросов (method, path-паттерн Chi, status).
var HTTPRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests by method, route pattern, and status code.",
	},
	[]string{"method", "path", "status"},
)

// HTTPRequestDuration — гистограмма длительности запроса в секундах (method, path-паттерн Chi).
var HTTPRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds by method and route pattern.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"method", "path"},
)
