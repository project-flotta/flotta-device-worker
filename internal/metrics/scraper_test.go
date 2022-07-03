package metrics_test

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/common/model"
)

const (
	metricsPayload = `
# HELP http_requests_total Total number of HTTP requests made.
# TYPE http_requests_total counter
http_requests_total{code="200",handler="prometheus",method="get"} 119 123456
# HELP prometheus_samples_queue_capacity Capacity of the queue for unwritten samples.
# TYPE prometheus_samples_queue_capacity gauge
prometheus_samples_queue_capacity 4096 654321
`
)

var (
	expectedSamples = model.Vector{&model.Sample{
		Metric: model.Metric{
			model.MetricNameLabel: "http_requests_total",
			"code":                "200",
			"handler":             "prometheus",
			"method":              "get",
		},
		Value:     119,
		Timestamp: 123456,
	},
		&model.Sample{
			Metric: model.Metric{
				model.MetricNameLabel: "prometheus_samples_queue_capacity",
			},
			Value:     4096,
			Timestamp: 654321,
		},
	}
)

var _ = Describe("Scraper", func() {

	When("Using an HTTP URL", func() {

		It("should scrape metrics", func() {
			// given
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", `text/plain; version=0.0.4`)
					_, _ = w.Write([]byte(metricsPayload))
				}),
			)
			defer server.Close()

			scraper, err := metrics.NewHTTPScraper(server.URL)
			Expect(err).ToNot(HaveOccurred())

			// when
			samples, err := scraper.Scrape(context.TODO())

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(samples).To(HaveLen(2))
			Expect(samples).To(ContainElements(expectedSamples))
		})

		It("should scrape metrics with gzip encoding", func() {
			// given
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", `text/plain; version=0.0.4`)
					w.Header().Set("Content-Encoding", "gzip")
					gzipWriter := gzip.NewWriter(w)
					_, _ = gzipWriter.Write([]byte(metricsPayload))
					_ = gzipWriter.Close()
				}),
			)
			defer server.Close()

			scraper, err := metrics.NewHTTPScraper(server.URL)
			Expect(err).ToNot(HaveOccurred())

			// when
			samples, err := scraper.Scrape(context.TODO())

			// then
			Expect(err).ToNot(HaveOccurred())
			Expect(samples).To(HaveLen(2))
			Expect(samples).To(ContainElements(expectedSamples))
		})

		It("should report error on HTTP error", func() {
			// given
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}),
			)
			defer server.Close()

			scraper, err := metrics.NewHTTPScraper(server.URL)
			Expect(err).ToNot(HaveOccurred())

			// when
			_, err = scraper.Scrape(context.TODO())

			// then
			Expect(err).To(HaveOccurred())
		})

		It("should report error on metrics decoding error", func() {
			// given
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", `text/plain; version=0.0.4`)
					_, _ = w.Write([]byte("!!!!"))
				}),
			)
			defer server.Close()

			scraper, err := metrics.NewHTTPScraper(server.URL)
			Expect(err).ToNot(HaveOccurred())

			// when
			_, err = scraper.Scrape(context.TODO())

			// then
			Expect(err).To(HaveOccurred())
		})
	})

	When("Using a prometheus gatherer", func() {
		var (
			registry *prometheus.Registry
			factory  promauto.Factory
		)

		It("should scrape metrics", func() {
			//given
			registry = prometheus.NewRegistry()
			factory = promauto.With(registry)
			m := factory.NewCounterVec(
				prometheus.CounterOpts{
					Name: "flotta_agent_datasync_files_transferred_counter",
					Help: "help",
				}, []string{"workloadName", "direction"})
			m2 := factory.NewCounterVec(
				prometheus.CounterOpts{
					Name: "flotta_agent_datasync_bytes_transferred_counter",
					Help: "help",
				}, []string{"workloadName", "direction"})

			m.WithLabelValues("wrk1", "egress").Add(10)
			m2.WithLabelValues("wrk1", "egress").Add(200)
			//when
			scrapper := metrics.NewObjectScraper(registry)
			r, err := scrapper.Scrape(context.Background())
			//then
			Expect(err).NotTo(HaveOccurred())
			Expect(len(r)).To(Equal(2))
			Expect(r[0].Metric).To(Equal(model.Metric{model.MetricNameLabel: "flotta_agent_datasync_bytes_transferred_counter",
				"workloadName": "wrk1", "direction": "egress"}))
			Expect(r[0].Value).To(Equal(model.SampleValue(200)))
			Expect(r[0].Timestamp).To(BeNumerically(">", 0))
			Expect(r[0].Timestamp).To(BeNumerically("<=", model.Now()))
			//And
			Expect(r[1].Metric).To(Equal(model.Metric{model.MetricNameLabel: "flotta_agent_datasync_files_transferred_counter",
				"workloadName": "wrk1", "direction": "egress"}))
			Expect(r[1].Value).To(Equal(model.SampleValue(10)))
			Expect(r[1].Timestamp).To(BeNumerically(">", 0))
			Expect(r[1].Timestamp).To(BeNumerically("<=", model.Now()))
		})
	})

})
