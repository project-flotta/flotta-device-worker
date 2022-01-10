package metrics_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	gomock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/model"

	"github.com/jakub-dzon/k4e-device-worker/internal/metrics"
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
			target := metrics.NewTargetMetric(
				"wrk1",
				1*time.Minute,
				[]string{fmt.Sprintf("%s/metrics", server.URL)},
				store)

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

	})

	Context("Daemon", func() {

		It("Targets are added correctly", func() {

			// given
			daemon := metrics.NewMetricsDaemon(metrics.NewMockAPI(mockCtrl))

			// when
			daemon.AddTarget("wrk1", []string{"http://192.168.1.1:8080"}, time.Second, map[string]string{})

			// then
			targets := daemon.GetTargets()
			Expect(targets).To(HaveLen(1))
			Expect(targets).To(Equal([]string{"wrk1"}))
		})

		It("Targets added/removed correctly", func() {

			// given
			daemon := metrics.NewMetricsDaemon(metrics.NewMockAPI(mockCtrl))

			// when
			daemon.AddTarget("wrk1", []string{"http://192.168.1.1:8080"}, time.Second, map[string]string{})
			daemon.AddTarget("wrk2", []string{"http://192.168.1.1:8080"}, time.Second, map[string]string{})
			daemon.AddTarget("wrk3", []string{"http://192.168.1.1:8080"}, time.Second, map[string]string{})
			daemon.DeleteTarget("wrk2")

			// then
			targets := daemon.GetTargets()
			Expect(targets).To(HaveLen(2))

			Expect(targets).To(ConsistOf("wrk1", "wrk3"))
		})

	})
})
