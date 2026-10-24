package devmetrics

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

func TestClientDecodesRejectionBeforeStatusAndRetriesRateLimit(t *testing.T) {
	attempts := 0
	client := &InfraiClient{
		BaseURL: "https://example.test",
		APIKey:  "test-key-from-env",
		HTTP: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if req.Method != http.MethodPost || req.URL.Path != metricsReportPath {
				t.Fatalf("request = %s %s", req.Method, req.URL.Path)
			}
			if attempts == 1 {
				return response(http.StatusTooManyRequests, `{"ok":false,"data":null,"error":{},"metadata":{}}`, "0"), nil
			}
			return response(http.StatusOK, `{"ok":true,"data":{"accepted":true},"error":null,"metadata":{}}`, ""), nil
		}),
		Sleep: func(context.Context, time.Duration) error { return nil },
	}

	_, err := client.ReportMetric(context.Background(), MetricPoint{Name: "devtools.build.completed", Value: 1, Type: "counter", IdempotencyKey: "evt-1"})
	if err != nil {
		t.Fatalf("ReportMetric() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func response(status int, body, retryAfter string) *http.Response {
	header := make(http.Header)
	if retryAfter != "" {
		header.Set("Retry-After", retryAfter)
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}
}
