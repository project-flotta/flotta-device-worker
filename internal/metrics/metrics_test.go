package metrics_test

import (
	"io/ioutil"
	"math/rand"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/metrics"
)

var _ = Describe("Metrics", func() {
	var (
		tsdb *metrics.TSDB
	)

	BeforeEach(func() {
		tmpDir, err := ioutil.TempDir("", "metrics")
		Expect(err).ToNot(HaveOccurred())

		tsdb, err = metrics.NewTSDB(tmpDir)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := tsdb.Deregister()
		Expect(err).ToNot(HaveOccurred())
	})

	It("should record and retrieve metric2", func() {
		// given
		value1 := rand.Float64() * 100
		seriesOne := map[string]string{
			"labelA": "a",
			"labelB": "b",
		}

		seriesTwoValue1 := rand.Float64() * 100
		seriesTwo := map[string]string{
			"label1": "1",
			"label2": "2",
		}
		seriesTwoValue2 := rand.Float64() * 100
		before := time.Now()

		// when
		err := tsdb.AddMetric(value1, seriesOne)
		Expect(err).ToNot(HaveOccurred())

		err = tsdb.AddMetric(seriesTwoValue1, seriesTwo)
		Expect(err).ToNot(HaveOccurred())

		// To report value for a different timestamp
		time.Sleep(time.Millisecond)

		err = tsdb.AddMetric(seriesTwoValue2, seriesTwo)
		Expect(err).ToNot(HaveOccurred())

		metricSet, err := tsdb.GetMetricsForTimeRange(before, time.Now(), false)
		Expect(err).ToNot(HaveOccurred())

		// then
		Expect(metricSet).To(HaveLen(2))
		Expect(metricSet[0].Labels).To(Equal(seriesOne))
		Expect(metricSet[0].DataPoints).To(HaveLen(1))
		Expect(metricSet[0].DataPoints[0].Value).To(Equal(value1))

		Expect(metricSet[1].Labels).To(Equal(seriesTwo))
		Expect(metricSet[1].DataPoints).To(HaveLen(2))
		Expect(metricSet[1].DataPoints[0].Value).To(Equal(seriesTwoValue1))
		Expect(metricSet[1].DataPoints[1].Value).To(Equal(seriesTwoValue2))
	})
})
