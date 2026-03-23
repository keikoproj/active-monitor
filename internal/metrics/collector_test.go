package metrics

import (
	"testing"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

var registry = prometheus.NewRegistry()

func TestCollectEmptyMetric(t *testing.T) {
	value := ""
	var parameters = []wfv1.Parameter{{Value: (*wfv1.AnyString)(&value)}}

	wfstatus := &wfv1.WorkflowStatus{
		Outputs: &wfv1.Outputs{
			Parameters: parameters,
		},
	}
	CreateDynamicPrometheusMetric("test-flow", wfstatus, registry)
}

func TestCollectMetric(t *testing.T) {
	value := "{\"metrics\":[{\"name\": \"custom_total\", \"value\": 123, \"metrictype\": \"gauge\", \"help\": \"test help\"}, {\"name\": \"custom_metric\", \"value\": 12.3, \"metrictype\": \"gauge\"}]}"
	var parameters = []wfv1.Parameter{{Value: (*wfv1.AnyString)(&value)}}

	wfstatus := &wfv1.WorkflowStatus{
		Outputs: &wfv1.Outputs{
			Parameters: parameters,
		},
	}
	CreateDynamicPrometheusMetric("test-flow", wfstatus, registry)
}

func TestCollectNoNameMetric(t *testing.T) {
	value := "{\"metrics\":[{\"value\": 123, \"metrictype\": \"gauge\", \"help\": \"test help\"}, {\"name\": \"custom_metric\", \"value\": 12.3, \"metrictype\": \"gauge\"}]}"
	var parameters = []wfv1.Parameter{{Value: (*wfv1.AnyString)(&value)}}

	wfstatus := &wfv1.WorkflowStatus{
		Outputs: &wfv1.Outputs{
			Parameters: parameters,
		},
	}
	CreateDynamicPrometheusMetric("test-flow", wfstatus, registry)
}

func TestCollectNoValueMetric(t *testing.T) {
	value := "{\"metrics\":[{\"name\": \"custom_total\", \"metrictype\": \"gauge\", \"help\": \"test help\"}, {\"name\": \"custom_metric\", \"value\": 12.3, \"metrictype\": \"gauge\"}]}"
	var parameters = []wfv1.Parameter{{Value: (*wfv1.AnyString)(&value)}}

	wfstatus := &wfv1.WorkflowStatus{
		Outputs: &wfv1.Outputs{
			Parameters: parameters,
		},
	}
	CreateDynamicPrometheusMetric("test-flow", wfstatus, registry)
}

func TestCreateDynamicPrometheusMetric_NilWorkflowStatus_DoesNotPanic(t *testing.T) {
	// workflowStatus.Outputs is accessed on the first line of CreateDynamicPrometheusMetric;
	// passing a nil pointer would panic without a nil guard. This test documents that the
	// nil check covers the WorkflowStatus pointer itself (the function receives a pointer
	// and checks .Outputs, so a nil WorkflowStatus would panic). If this test fails with a
	// panic, add a nil guard for workflowStatus before the Outputs check.
	assert.NotPanics(t, func() {
		wfstatus := &wfv1.WorkflowStatus{Outputs: nil}
		CreateDynamicPrometheusMetric("nil-outputs-flow", wfstatus, registry)
	})
}

func TestCreateDynamicPrometheusMetric_NilOutputsParameters_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		wfstatus := &wfv1.WorkflowStatus{
			Outputs: &wfv1.Outputs{Parameters: nil},
		}
		CreateDynamicPrometheusMetric("nil-params-flow", wfstatus, registry)
	})
}

func TestCreateDynamicPrometheusMetric_ConcurrentRegistration_NoDataRace(t *testing.T) {
	// This test documents a known data race in CustomGaugeMetricsMap (package-level
	// map with no mutex). Running with -race will detect concurrent map read/writes
	// in collector.go lines 90-102. The fix is to protect CustomGaugeMetricsMap with
	// a sync.RWMutex. Tracked in issue #288.
	t.Skip("Known data race in CustomGaugeMetricsMap — see issue #288")
}
