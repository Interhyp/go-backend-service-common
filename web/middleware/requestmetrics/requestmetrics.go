package requestmetrics

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"strings"
	"time"
)

var (
	requestSummaryName       = "http_server_requests_seconds"
	requestSummaryLegacyName = "http_server_requests_seconds_sum"

	requestSummary       *prometheus.SummaryVec
	requestSummaryLegacy *prometheus.SummaryVec
)

func Setup() {
	requestSummary = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       requestSummaryName,
			Help:       "Request counts, durations, accumulated and in quantiles, partitioned by method, \"outcome\", status code, and HTTP path (grouped by patterns).",
			Objectives: map[float64]float64{0.5: 0.05, 0.95: 0.01, 0.99: 0.001},
		},
		[]string{"method", "outcome", "status", "uri"},
	)

	prometheus.MustRegister(requestSummaryLegacy)

	requestSummaryLegacy = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: requestSummaryLegacyName,
			Help: "(Legacy, replaced by \"http_server_requests_seconds(_sum|_count)?\") Accumulated request durations and counts partitioned by method, \"outcome\", status code, and HTTP path (grouped by patterns).",
		},
		[]string{"method", "outcome", "status", "uri"},
	)

	prometheus.MustRegister(requestSummary)
}

func RecordRequestMetrics(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		duration := time.Since(start)

		rctx := chi.RouteContext(r.Context())
		routePattern := strings.Join(rctx.RoutePatterns, "")
		routePattern = strings.Replace(routePattern, "/*/", "/", -1)

		requestSummary.WithLabelValues(r.Method, outcome(ww.Status()), fmt.Sprintf("%d", ww.Status()), routePattern).Observe(duration.Seconds())
		requestSummaryLegacy.WithLabelValues(r.Method, outcome(ww.Status()), fmt.Sprintf("%d", ww.Status()), routePattern).Observe(duration.Seconds())
	}
	return http.HandlerFunc(fn)
}

func outcome(status int) string {
	if status < 400 {
		return "SUCCESS"
	} else if status < 500 {
		return "CLIENT_ERROR"
	} else {
		return "SERVER_ERROR"
	}
}
