package metrics_test

import (
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/datatransfer"
	"github.com/project-flotta/flotta-device-worker/internal/metrics"
	"github.com/project-flotta/flotta-operator/models"
)

var _ = Describe("Data Transfer", func() {

	const (
		duration int32 = 123
	)

	var (
		customAllowList = &models.MetricsAllowList{
			Names: []string{"allow_a", "allow_b"}}
		customFilter = metrics.NewRestrictiveAllowList(customAllowList)

		noMetricsConfig = models.DeviceConfigurationMessage{
			Configuration: &models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{},
			},
		}
		customConfig = models.DeviceConfigurationMessage{
			Configuration: &models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{
					DataTransfer: &models.ComponentMetricsConfiguration{
						Interval:  duration,
						AllowList: customAllowList,
					},
				},
			},
		}
		defaultFilter = metrics.DefaultDataTransferAllowList()

		mockCtrl   *gomock.Controller
		daemonMock *metrics.MockMetricsDaemon
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		daemonMock = metrics.NewMockMetricsDaemon(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	When("Init", func() {

		It("default configuration", func() {
			// given
			daemonMock.EXPECT().AddTarget(gomock.Any(),
				gomock.Eq([]string{datatransfer.MetricsEndpoint}),
				gomock.Eq(time.Second*time.Duration(metrics.DefaultDataTransferMetricsScrapingInterval)),
				gomock.Eq(defaultFilter)).
				Times(1)
			sm := metrics.NewDataTransferMetrics(daemonMock)

			// when
			err := sm.Init(noMetricsConfig)

			// then
			Expect(err).ToNot(HaveOccurred())
			// daemonMock expectation met
		})

		It("custom configuration", func() {
			// given
			daemonMock.EXPECT().AddTarget(gomock.Any(),
				gomock.Eq([]string{datatransfer.MetricsEndpoint}),
				gomock.Eq(time.Second*time.Duration(duration)),
				gomock.Eq(customFilter))
			sm := metrics.NewDataTransferMetrics(daemonMock)

			// when
			err := sm.Init(customConfig)

			// then
			Expect(err).ToNot(HaveOccurred())
			// daemonMock expectation met
		})

		It("custom configuration with interval unset", func() {
			// given
			config := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Metrics: &models.MetricsConfiguration{
						DataTransfer: &models.ComponentMetricsConfiguration{
							AllowList: customAllowList,
						},
					},
				},
			}
			daemonMock.EXPECT().AddTarget(gomock.Any(),
				gomock.Eq([]string{datatransfer.MetricsEndpoint}),
				gomock.Eq(time.Second*time.Duration(metrics.DefaultDataTransferMetricsScrapingInterval)),
				gomock.Eq(customFilter))

			sm := metrics.NewDataTransferMetrics(daemonMock)

			// when
			err := sm.Init(config)

			// then
			Expect(err).ToNot(HaveOccurred())
			// daemonMock expectation met
		})

		It("disable data transfer at init", func() {
			// given
			config := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Metrics: &models.MetricsConfiguration{
						DataTransfer: &models.ComponentMetricsConfiguration{
							Disabled: true,
						},
					},
				},
			}
			daemonMock.EXPECT().DeleteTarget("data transfer")

			sm := metrics.NewDataTransferMetrics(daemonMock)

			// when
			err := sm.Init(config)

			// then
			Expect(err).ToNot(HaveOccurred())
			// daemonMock expectation met
		})

	})

	When("Update", func() {

		It("should not re-add endpoint for scraping when default configuration is used repeatedly in update", func() {
			// given
			daemonMock.EXPECT().AddTarget(gomock.Any(),
				gomock.Eq([]string{datatransfer.MetricsEndpoint}),
				gomock.Eq(time.Second*time.Duration(metrics.DefaultDataTransferMetricsScrapingInterval)),
				gomock.Eq(defaultFilter)).
				Times(1)

			dt := metrics.NewDataTransferMetrics(daemonMock)
			err := dt.Init(noMetricsConfig)
			Expect(err).ToNot(HaveOccurred())
			err = dt.Update(noMetricsConfig)
			Expect(err).To(Not(HaveOccurred()))

			// when
			err = dt.Update(noMetricsConfig)

			// then
			Expect(err).To(Not(HaveOccurred()))
			// daemonMock expectation met
		})

		It("should not re-add endpoint for scraping when same custom configuration is used repeatedly in update", func() {
			// given
			daemonMock.EXPECT().AddTarget(gomock.Any(),
				gomock.Eq([]string{datatransfer.MetricsEndpoint}),
				gomock.Eq(time.Second*time.Duration(duration)),
				gomock.Eq(customFilter)).
				Times(1)

			dt := metrics.NewDataTransferMetrics(daemonMock)
			err := dt.Init(customConfig)
			Expect(err).ToNot(HaveOccurred())

			err = dt.Update(customConfig)
			Expect(err).To(Not(HaveOccurred()))
			err = dt.Update(customConfig)
			Expect(err).To(Not(HaveOccurred()))

		})

		It("should re-add endpoint for scraping when configuration changes from default to custom", func() {
			// given
			daemonMock.EXPECT().AddTarget(gomock.Any(),
				gomock.Eq([]string{datatransfer.MetricsEndpoint}),
				gomock.Eq(time.Second*time.Duration(duration)),
				gomock.Eq(customFilter)).
				Times(1).
				After(
					daemonMock.EXPECT().AddTarget(gomock.Any(),
						gomock.Eq([]string{datatransfer.MetricsEndpoint}),
						gomock.Eq(time.Second*time.Duration(metrics.DefaultDataTransferMetricsScrapingInterval)),
						gomock.Eq(defaultFilter)).
						Times(1),
				)

			dt := metrics.NewDataTransferMetrics(daemonMock)
			err := dt.Init(noMetricsConfig)

			// when
			Expect(err).ToNot(HaveOccurred())
			err = dt.Update(customConfig)

			// then
			Expect(err).To(Not(HaveOccurred()))
			// daemonMock expectation met

		})

		It("should re-add  endpoint for scraping when configuration changes from custom to default", func() {
			// given
			daemonMock.EXPECT().AddTarget(gomock.Any(),
				gomock.Eq([]string{datatransfer.MetricsEndpoint}),
				gomock.Eq(time.Second*time.Duration(metrics.DefaultDataTransferMetricsScrapingInterval)),
				gomock.Eq(defaultFilter)).
				Times(1).
				After(
					daemonMock.EXPECT().AddTarget(gomock.Any(),
						gomock.Eq([]string{datatransfer.MetricsEndpoint}),
						gomock.Eq(time.Second*time.Duration(duration)),
						gomock.Eq(customFilter)).
						Times(1),
				)

			dt := metrics.NewDataTransferMetrics(daemonMock)
			err := dt.Init(customConfig)

			// when
			Expect(err).ToNot(HaveOccurred())
			err = dt.Update(noMetricsConfig)

			// then
			Expect(err).To(Not(HaveOccurred()))
			// daemonMock expectation met
		})

		It("should disable metrics", func() {
			// given
			daemonMock.EXPECT().DeleteTarget("data transfer").After(
				daemonMock.EXPECT().AddTarget(gomock.Any(),
					gomock.Eq([]string{datatransfer.MetricsEndpoint}),
					gomock.Eq(time.Second*time.Duration(metrics.DefaultDataTransferMetricsScrapingInterval)),
					gomock.Eq(defaultFilter)).
					Times(1),
			)

			config := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Metrics: &models.MetricsConfiguration{
						DataTransfer: &models.ComponentMetricsConfiguration{
							Disabled: true,
						},
					},
				},
			}

			sm := metrics.NewDataTransferMetrics(daemonMock)
			err := sm.Init(noMetricsConfig)
			Expect(err).ToNot(HaveOccurred())

			// when
			err = sm.Update(config)

			// then
			Expect(err).ToNot(HaveOccurred())
			// daemonMock expectation met
		})

	})

	When("Delete", func() {

		It("should delete target on de-registration", func() {
			// given
			daemonMock.EXPECT().DeleteTarget("data transfer")
			sm := metrics.NewDataTransferMetrics(daemonMock)

			// when
			err := sm.Deregister()

			// then
			Expect(err).ToNot(HaveOccurred())
			// daemonMock expectation met
		})
	})
})
