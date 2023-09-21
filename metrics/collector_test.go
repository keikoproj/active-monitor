package metrics

import (
	"testing"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
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
