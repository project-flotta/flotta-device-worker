package metrics_test

import (
	"fmt"
	"github.com/jakub-dzon/k4e-operator/models"
	"net/http"
	"net/http/httptest"
	"time"

	gomock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/model"

	"github.com/project-flotta/flotta-device-worker/internal/metrics"
)

var _ = Describe("Daemon", func() {

	var (
		mockCtrl *gomock.Controller
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		defer GinkgoRecover()
		mockCtrl.Finish()
	})

	Context("TargetMetric", func() {

		var server *httptest.Server

		BeforeEach(func() {
			server = httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", `text/plain; version=0.0.4`)
					_, _ = w.Write([]byte(metricsPayload))
				}),
			)

		})

		AfterEach(func() {
			defer server.Close()
		})

		It("Get the metrics correctly", func() {
			// given

			store := metrics.NewMockAPI(mockCtrl)
			target := metrics.NewTargetMetric("wrk1", 1*time.Minute, []string{fmt.Sprintf("%s/metrics", server.URL)}, store, &metrics.PermissiveAllowList{})

			started := time.Now()
			checkExecuting := func() bool {
				return target.LatestSuccessRun().After(started)
			}

			// then
			store.EXPECT().AddVector(gomock.Any(), gomock.Any()).
				Times(1).
				Return(nil).
				Do(func(data model.Vector, labelsMap map[string]string) error {
					defer GinkgoRecover()
					Expect(data).Should(HaveLen(2))
					Expect(labelsMap).Should(HaveKeyWithValue("metric-source", "wrk1"))
					return nil
				})

			// when
			go target.Start()
			target.ForceEvent()

			// then
			Eventually(checkExecuting, 10, 1).Should(BeTrue(), "Run was not executed")

			target.Stop()
			Expect(target.IsStopped()).To(BeTrue())
		})

		It("Get filtered metrics correctly", func() {
			// given
			store := metrics.NewMockAPI(mockCtrl)
			filter := metrics.NewRestrictiveAllowList(&models.MetricsAllowList{
				Names: []string{"prometheus_samples_queue_capacity"},
			})
			target := metrics.NewTargetMetric("wrk1", 1*time.Minute, []string{fmt.Sprintf("%s/metrics", server.URL)}, store, filter)

			started := time.Now()
			checkExecuting := func() bool {
				return target.LatestSuccessRun().After(started)
			}

			// then
			store.EXPECT().AddVector(gomock.Any(), gomock.Any()).
				Times(1).
				Return(nil).
				Do(func(data model.Vector, labelsMap map[string]string) error {
					defer GinkgoRecover()
					Expect(data).Should(HaveLen(1))
					Expect(data[0].Metric).Should(HaveKeyWithValue(model.LabelName(model.MetricNameLabel), model.LabelValue("prometheus_samples_queue_capacity")))
					Expect(labelsMap).Should(HaveKeyWithValue("metric-source", "wrk1"))
					return nil
				})

			// when
			go target.Start()
			target.ForceEvent()

			// then
			Eventually(checkExecuting, 10, 1).Should(BeTrue(), "Run was not executed")

			target.Stop()
			Expect(target.IsStopped()).To(BeTrue())
		})

	})

	Context("Daemon", func() {

		It("Targets are added correctly", func() {

			// given
			daemon := metrics.NewMetricsDaemon(metrics.NewMockAPI(mockCtrl))

			// when
			daemon.AddTarget("wrk1", []string{"http://192.168.1.1:8080"}, time.Second)

			// then
			targets := daemon.GetTargets()
			Expect(targets).To(HaveLen(1))
			Expect(targets).To(Equal([]string{"wrk1"}))
		})

		It("Targets added/removed correctly", func() {

			// given
			daemon := metrics.NewMetricsDaemon(metrics.NewMockAPI(mockCtrl))

			// when
			daemon.AddTarget("wrk1", []string{"http://192.168.1.1:8080"}, time.Second)
			daemon.AddTarget("wrk2", []string{"http://192.168.1.1:8080"}, time.Second)
			daemon.AddTarget("wrk3", []string{"http://192.168.1.1:8080"}, time.Second)
			daemon.DeleteTarget("wrk2")

			// then
			targets := daemon.GetTargets()
			Expect(targets).To(HaveLen(2))

			Expect(targets).To(ConsistOf("wrk1", "wrk3"))
		})

	})
})
