package metrics

import (
	"encoding/json"
	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"strings"
)

var hcName = "healthcheck_name"
var wf = "workflow"

// MonitorProcessed will be used to track the number of processed events
var (
	MonitorSuccess = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "healthcheck_success_count",
		Help: "The total number of successful healthcheck resources",
	},
		[]string{hcName, wf},
	)
	MonitorError = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "healthcheck_error_count",
		Help: "The total number of errored healthcheck resources",
	},
		[]string{hcName, wf},
	)
	MonitorRuntime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "healthcheck_runtime_seconds",
		Help: "Time taken for the workflow to complete.",
	},
		[]string{hcName, wf},
	)
	MonitorStartedTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "healthcheck_starttime",
		Help: "Time taken for the workflow to complete.",
	},
		[]string{hcName, wf},
	)
	MonitorFinishedTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "healthcheck_finishedtime",
		Help: "Time taken for the workflow to complete.",
	},
		[]string{hcName, wf},
	)

	CustomGaugeMetricsMap = make(map[string]*prometheus.GaugeVec)
)

// customMetricMap defines the custom metric structure
type customMetricMap struct {
	Name       string
	Help       string
	Metrictype string
	Value      float64
}

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(MonitorSuccess, MonitorError, MonitorRuntime, MonitorStartedTime, MonitorFinishedTime)
}

// CreateDynamicPrometheusMetric initializes and registers custom metrics dynamically
func CreateDynamicPrometheusMetric(name string, workflowStatus *wfv1.WorkflowStatus, registry *prometheus.Registry) {
	if workflowStatus.Outputs == nil || workflowStatus.Outputs.Parameters == nil {
		return
	}

	for _, parameter := range workflowStatus.Outputs.Parameters {
		var jsonMap map[string][]interface{}
		json.Unmarshal([]byte(*parameter.Value), &jsonMap)

		for _, customMetricRaw := range jsonMap["metrics"] {
			var metric customMetricMap
			if err := mapstructure.Decode(customMetricRaw.(map[string]interface{}), &metric); err != nil {
				log.Errorf("Failed to decode metric for %s: %s", name, err)
				continue
			}

			if metric.Name == "" {
				log.Errorf("Skipping metric collection. Invalid metric %s: %#v", name, metric)
				continue
			}

			// replace "-" to "_" to make it Prometheus friendly metric names
			metric.Name = strings.ReplaceAll(name, "-", "_") + "_" + metric.Name

			if _, ok := CustomGaugeMetricsMap[metric.Name]; !ok {
				CustomGaugeMetricsMap[metric.Name] = prometheus.NewGaugeVec(
					prometheus.GaugeOpts{
						Name: metric.Name,
						Help: metric.Help,
					},
					[]string{hcName},
				)
				if err := registry.Register(CustomGaugeMetricsMap[metric.Name]); err != nil {
					log.Errorf("Error registering %s metric %s\n", metric.Name, err)
				}
			}
			CustomGaugeMetricsMap[metric.Name].With(prometheus.Labels{hcName: name}).Set(metric.Value)
			log.Printf("Successfully collected metric for %s, metric: %#v", name, metric)
		}
		log.Debugf("Here is the registered CustomGaugeMetricsMap %v\n", CustomGaugeMetricsMap)
	}
}
