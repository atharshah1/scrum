package observability

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type requestMetric struct {
	Count        int64
	Errors       int64
	TotalLatency time.Duration
}

type Metrics struct {
	mu       sync.RWMutex
	requests map[string]*requestMetric
}

func NewMetrics() *Metrics {
	return &Metrics{requests: map[string]*requestMetric{}}
}

func (m *Metrics) ObserveRequest(method, path string, status int, duration time.Duration) {
	key := method + " " + path
	m.mu.Lock()
	metric, ok := m.requests[key]
	if !ok {
		metric = &requestMetric{}
		m.requests[key] = metric
	}
	metric.Count++
	if status >= 400 {
		metric.Errors++
	}
	metric.TotalLatency += duration
	m.mu.Unlock()
}

func (m *Metrics) PrometheusText() string {
	m.mu.RLock()
	keys := make([]string, 0, len(m.requests))
	for key := range m.requests {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# HELP http_requests_total Total HTTP requests\n")
	b.WriteString("# TYPE http_requests_total counter\n")
	b.WriteString("# HELP http_request_errors_total Total HTTP error responses\n")
	b.WriteString("# TYPE http_request_errors_total counter\n")
	b.WriteString("# HELP http_request_duration_ms_total Total HTTP request latency in milliseconds\n")
	b.WriteString("# TYPE http_request_duration_ms_total counter\n")
	for _, key := range keys {
		parts := strings.SplitN(key, " ", 2)
		method := parts[0]
		path := "/"
		if len(parts) > 1 {
			path = parts[1]
		}
		metric := m.requests[key]
		b.WriteString(fmt.Sprintf("http_requests_total{method=%q,path=%q} %d\n", method, path, metric.Count))
		b.WriteString(fmt.Sprintf("http_request_errors_total{method=%q,path=%q} %d\n", method, path, metric.Errors))
		b.WriteString(fmt.Sprintf("http_request_duration_ms_total{method=%q,path=%q} %d\n", method, path, metric.TotalLatency.Milliseconds()))
	}
	m.mu.RUnlock()
	return b.String()
}
