package devmetrics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// These call sites correspond to infrai.metrics.report, infrai.metrics.query,
// and infrai.account.usage.timeseries. They share one key and base URL.
const (
	metricsReportPath = "/v1/metrics/report"
	metricsQueryPath  = "/v1/metrics/query"
	usageSeriesPath   = "/v1/account/usage/timeseries"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type InfraiClient struct {
	BaseURL string
	APIKey  string
	HTTP    HTTPDoer
	Sleep   func(context.Context, time.Duration) error
}

type InfraiError struct {
	Code       string         `json:"code"`
	Details    map[string]any `json:"-"`
	HTTPStatus int            `json:"-"`
}

func (e *InfraiError) Error() string {
	if e.Code == "" {
		return "Infrai rejected the request"
	}
	return "Infrai rejected the request: " + e.Code
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type MetricPoint struct {
	Name           string            `json:"name"`
	Value          float64           `json:"value"`
	Type           string            `json:"type"`
	Tags           map[string]string `json:"tags,omitempty"`
	Timestamp      string            `json:"timestamp,omitempty"`
	IdempotencyKey string            `json:"idempotency_key"`
}

func (c *InfraiClient) ReportMetric(ctx context.Context, point MetricPoint) (json.RawMessage, error) {
	return c.call(ctx, http.MethodPost, metricsReportPath, nil, point)
}

func (c *InfraiClient) QueryMetric(ctx context.Context, name, agg string) (json.RawMessage, error) {
	query := url.Values{"name": {name}, "agg": {agg}}
	return c.call(ctx, http.MethodGet, metricsQueryPath, query, nil)
}

func (c *InfraiClient) UsageTimeseries(ctx context.Context) (json.RawMessage, error) {
	return c.call(ctx, http.MethodGet, usageSeriesPath, nil, nil)
}

func (c *InfraiClient) call(ctx context.Context, method, path string, query url.Values, payload any) (json.RawMessage, error) {
	if c.HTTP == nil {
		c.HTTP = http.DefaultClient
	}
	if c.Sleep == nil {
		c.Sleep = sleepContext
	}

	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		rawURL := strings.TrimRight(c.BaseURL, "/") + path
		if len(query) > 0 {
			rawURL += "?" + query.Encode()
		}
		req, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		res, err := c.HTTP.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}
		responseBody, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(responseBody, &env)
		if decodeErr == nil && !env.OK {
			infraiErr := decodeInfraiError(env.Error, res.StatusCode)
			if res.StatusCode != http.StatusTooManyRequests || attempt == 3 {
				return nil, infraiErr
			}
			if err := c.Sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return nil, err
			}
			continue
		}
		if decodeErr != nil {
			return nil, fmt.Errorf("decode Infrai envelope: %w", decodeErr)
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return nil, fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		return env.Data, nil
	}
	return nil, errors.New("retry attempts exhausted")
}

func decodeInfraiError(raw json.RawMessage, status int) *InfraiError {
	var details map[string]any
	_ = json.Unmarshal(raw, &details)
	code, _ := details["code"].(string)
	return &InfraiError{Code: code, Details: details, HTTPStatus: status}
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
