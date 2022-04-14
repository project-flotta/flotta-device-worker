package metrics_test

import (
	"compress/gzip"
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/metrics"
	"github.com/prometheus/common/model"
	"net/http"
	"net/http/httptest"
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
