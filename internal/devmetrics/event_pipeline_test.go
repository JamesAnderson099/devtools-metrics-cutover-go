package devmetrics

import (
	"context"
	"encoding/json"
	"testing"
)

type recordingReporter struct {
	points []MetricPoint
}

func (r *recordingReporter) ReportMetric(_ context.Context, point MetricPoint) (json.RawMessage, error) {
	r.points = append(r.points, point)
	return json.RawMessage(`{"accepted":true}`), nil
}

func TestEventPipelineBusinessDecision(t *testing.T) {
	tests := []struct {
		name       string
		event      DeveloperEvent
		wantMetric string
		wantType   string
		wantValue  float64
	}{
		{"successful build increments throughput", DeveloperEvent{"evt-build-42", "build", "compiler", "succeeded", "", "2026-09-25T08:00:00Z"}, "devtools.build.completed", "counter", 1},
		{"release increments delivery count", DeveloperEvent{"evt-release-9", "release", "cli", "deployed", "", "2026-09-25T08:02:00Z"}, "devtools.release.completed", "counter", 1},
		{"resolved diagnostic clears gauge", DeveloperEvent{"evt-diag-7", "diagnostic", "language-server", "resolved", "warning", "2026-09-25T08:03:00Z"}, "devtools.diagnostic.open", "gauge", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := &recordingReporter{}
			got, err := (EventPipeline{Reporter: reporter}).Record(context.Background(), tt.event)
			if err != nil {
				t.Fatalf("Record() error = %v", err)
			}
			if got.Name != tt.wantMetric || got.Type != tt.wantType || got.Value != tt.wantValue {
				t.Fatalf("metric = %#v, want name=%q type=%q value=%v", got, tt.wantMetric, tt.wantType, tt.wantValue)
			}
			if got.IdempotencyKey != tt.event.EventID {
				t.Fatalf("idempotency key = %q, want event id %q", got.IdempotencyKey, tt.event.EventID)
			}
		})
	}
}
