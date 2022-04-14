package metrics_test

import (
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/metrics"
	"github.com/project-flotta/flotta-operator/models"
	"github.com/prometheus/common/model"
)

var _ = Describe("Allow lists", func() {
	commonInput := model.Vector{
		newSample("a"),
		withLabel(newSample("b"), "foo", "bar"),
		newSample("c"),
	}

	Context("Restrictive", func() {
		DescribeTable("should filter elements", func(allowedNames []string, inputVector, expectedOutput model.Vector) {
			// given
			allowList := models.MetricsAllowList{Names: allowedNames}
			filter := metrics.NewRestrictiveAllowList(&allowList)

			// when
			filtered := filter.Filter(inputVector)

			// then
			Expect(filtered).To(ConsistOf(expectedOutput))
		},
			Entry("empty input", []string{"a", "b", "c"}, model.Vector{}, model.Vector{}),

			Entry("all samples on the allow list - exactly", []string{"a", "b", "c"}, commonInput, commonInput),
			Entry("all samples on the allow list - additional allowed", []string{"a", "b", "c", "x", "y", "z"}, commonInput, commonInput),
			Entry("some samples on the allow list",
				[]string{"b", "c"},
				commonInput,
				model.Vector{withLabel(newSample("b"), "foo", "bar"), newSample("c")},
			),
			Entry("no samples on the allow list", []string{"x", "y", "z"}, commonInput, model.Vector{}),

			Entry("no samples on empty allow  list", []string{}, commonInput, model.Vector{}),
		)
	})

	Context("Default for system", func() {
		DescribeTable("should pass-through default elements", func(metricName string) {
			// given
			filter := metrics.DefaultSystemAllowList()
			inputVector := model.Vector{newSample(metricName)}

			// when
			filtered := filter.Filter(inputVector)

			// then
			Expect(filtered).To(ConsistOf(inputVector))
		},
			Entry("node_cpu_frequency_hertz", "node_cpu_frequency_hertz"),
			Entry("node_cpu_scaling_frequency_min_hertz", "node_cpu_scaling_frequency_min_hertz"),
			Entry("node_cpu_scaling_frequency_hertz", "node_cpu_scaling_frequency_hertz"),
			Entry("node_cpu_scaling_frequency_max_hertz", "node_cpu_scaling_frequency_max_hertz"),
			Entry("node_disk_read_bytes_total", "node_disk_read_bytes_total"),
			Entry("node_disk_written_bytes_total", "node_disk_written_bytes_total"),
			Entry("node_memory_MemAvailable_bytes", "node_memory_MemAvailable_bytes"),
			Entry("node_memory_MemFree_bytes", "node_memory_MemFree_bytes"),
			Entry("node_memory_MemTotal_bytes", "node_memory_MemTotal_bytes"),
			Entry("node_network_info", "node_network_info"),
		)

		It("should block value not from list", func() {
			// given
			filter := metrics.DefaultSystemAllowList()
			randomName := uuid.New().String()
			inputVector := model.Vector{newSample(randomName)}

			// when
			filtered := filter.Filter(inputVector)

			// then
			Expect(filtered).To(BeEmpty())
		})

	})

	Context("Permissive", func() {
		DescribeTable("should pass-through elements", func(inputVector model.Vector) {
			// given
			filter := metrics.PermissiveAllowList{}

			// when
			filtered := filter.Filter(inputVector)

			// then
			Expect(filtered).To(ConsistOf(inputVector))
		},
			Entry("empty input", model.Vector{}),
			Entry("non-empty input", commonInput))
	})
})

func newSample(name string) *model.Sample {
	return &model.Sample{Metric: model.Metric{model.MetricNameLabel: model.LabelValue(name)}}
}

func withLabel(sample *model.Sample, name, value string) *model.Sample {
	sample.Metric[model.LabelName(name)] = model.LabelValue(value)
	return sample
}
