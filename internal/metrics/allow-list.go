package metrics

import (
	"github.com/project-flotta/flotta-operator/models"
	"github.com/prometheus/common/model"
)

type SampleFilter interface {
	Filter(samples model.Vector) model.Vector
}

type RestrictiveAllowList struct {
	allowedNames map[string]struct{}
}

type PermissiveAllowList struct{}

func (f *PermissiveAllowList) Filter(samples model.Vector) model.Vector {
	return samples
}

func DefaultSystemAllowList() SampleFilter {
	names := map[string]struct{}{
		"node_cpu_frequency_hertz":             {},
		"node_cpu_scaling_frequency_min_hertz": {},
		"node_cpu_scaling_frequency_hertz":     {},
		"node_cpu_scaling_frequency_max_hertz": {},

		"node_disk_read_bytes_total":    {},
		"node_disk_written_bytes_total": {},

		"node_memory_MemAvailable_bytes": {},
		"node_memory_MemFree_bytes":      {},
		"node_memory_MemTotal_bytes":     {},

		"node_network_info": {},
	}
	// TODO: set defaults
	return &RestrictiveAllowList{
		allowedNames: names,
	}
}

func NewRestrictiveAllowList(list *models.MetricsAllowList) SampleFilter {
	names := make(map[string]struct{})

	if list != nil {
		for _, name := range list.Names {
			names[name] = struct{}{}
		}
	}

	return &RestrictiveAllowList{
		allowedNames: names,
	}
}

func (f *RestrictiveAllowList) Filter(samples model.Vector) model.Vector {
	var filtered model.Vector
	for _, sample := range samples {
		if _, ok := f.allowedNames[string(sample.Metric[model.MetricNameLabel])]; ok {
			filtered = append(filtered, sample)
		}
	}
	return filtered
}
