package devmetrics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type Reporter interface {
	ReportMetric(context.Context, MetricPoint) (json.RawMessage, error)
}

type DeveloperEvent struct {
	EventID    string `json:"event_id"`
	Kind       string `json:"kind"`
	Project    string `json:"project"`
	Status     string `json:"status"`
	Severity   string `json:"severity,omitempty"`
	OccurredAt string `json:"occurred_at"`
}

type EventPipeline struct {
	Reporter Reporter
}

func (p EventPipeline) Record(ctx context.Context, event DeveloperEvent) (MetricPoint, error) {
	point, err := metricFor(event)
	if err != nil {
		return MetricPoint{}, err
	}
	if _, err := p.Reporter.ReportMetric(ctx, point); err != nil {
		return MetricPoint{}, err
	}
	return point, nil
}

func metricFor(event DeveloperEvent) (MetricPoint, error) {
	if event.EventID == "" || event.Project == "" || event.Status == "" {
		return MetricPoint{}, fmt.Errorf("event_id, project, and status are required")
	}
	if _, err := time.Parse(time.RFC3339, event.OccurredAt); err != nil {
		return MetricPoint{}, fmt.Errorf("occurred_at must be RFC3339")
	}

	tags := map[string]string{"project": event.Project, "status": event.Status}
	point := MetricPoint{Value: 1, Tags: tags, Timestamp: event.OccurredAt, IdempotencyKey: event.EventID}
	switch event.Kind {
	case "build":
		point.Name, point.Type = "devtools.build.completed", "counter"
	case "release":
		point.Name, point.Type = "devtools.release.completed", "counter"
	case "diagnostic":
		if event.Severity == "" {
			return MetricPoint{}, fmt.Errorf("severity is required for diagnostic events")
		}
		point.Name, point.Type = "devtools.diagnostic.open", "gauge"
		point.Tags["severity"] = event.Severity
		if event.Status == "resolved" {
			point.Value = 0
		}
	default:
		return MetricPoint{}, fmt.Errorf("kind must be build, release, or diagnostic")
	}
	return point, nil
}
