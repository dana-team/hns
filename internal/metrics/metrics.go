package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// InitializeHNSMetrics registers the relevant metrics.
func InitializeHNSMetrics() {
	metrics.Registry.MustRegister(
		snsAllocatedResources,
		snsFreeResources,
		snsTotalResources,
	)
}

var (
	snsAllocatedResources = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sns_allocated_resources",
			Help: "Indication of the quantity of an allocated subnamespace resource",
		}, []string{"name", "namespace", "resource"},
	)
)

var (
	snsFreeResources = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sns_free_resources",
			Help: "Indication of the quantity of a free subnamespace resource",
		}, []string{"name", "namespace", "resource"},
	)
)

var (
	snsTotalResources = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sns_total_resources",
			Help: "Indication of the total quantity of a subnamespace resource",
		}, []string{"name", "namespace", "resource"},
	)
)

// ObserveSNSAllocatedResource sets the allocated metric as per the quantity.
func ObserveSNSAllocatedResource(name, namespace, resource string, quantity float64) {
	snsAllocatedResources.With(prometheus.Labels{
		"name":      name,
		"namespace": namespace,
		"resource":  resource,
	}).Set(quantity)
}

// ObserveSNSFreeResource sets the allocatable metric as per the quantity.
func ObserveSNSFreeResource(name, namespace, resource string, quantity float64) {
	snsFreeResources.With(prometheus.Labels{
		"name":      name,
		"namespace": namespace,
		"resource":  resource,
	}).Set(quantity)
}

// ObserveSNSTotalResource sets the total metric as per the quantity.
func ObserveSNSTotalResource(name, namespace, resource string, quantity float64) {
	snsTotalResources.With(prometheus.Labels{
		"name":      name,
		"namespace": namespace,
		"resource":  resource,
	}).Set(quantity)
}
