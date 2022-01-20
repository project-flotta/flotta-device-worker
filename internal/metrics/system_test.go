package metrics_test

import (
	"github.com/golang/mock/gomock"
	"github.com/jakub-dzon/k4e-device-worker/internal/metrics"
	"github.com/jakub-dzon/k4e-operator/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("System", func() {

	const (
		duration int32 = 123
	)

	var (
		defaultFilter = metrics.DefaultSystemAllowList()

		customAllowList = &models.MetricsAllowList{
			Names: []string{"allow_a", "allow_b"}}
		customFilter = metrics.NewRestrictiveAllowList(customAllowList)

		mockCtrl       *gomock.Controller
		daemonMock     *metrics.MockMetricsDaemon
		systemMetrics  *metrics.SystemMetrics
		configProvider *metrics.MockDeviceConfigurationProvider
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		daemonMock = metrics.NewMockMetricsDaemon(mockCtrl)
		configProvider = metrics.NewMockDeviceConfigurationProvider(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	It("should add node_exporter endpoint for scraping at start: default configuration", func() {
		// given
		configProvider.EXPECT().GetDeviceConfiguration().Return(models.DeviceConfiguration{})
		daemonMock.EXPECT().AddFilteredTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
			gomock.Eq(defaultFilter)).
			Times(1)

		// when
		systemMetrics = metrics.NewSystemMetrics(daemonMock, configProvider)

		// then
		// daemonMock expectation met
	})

	It("should add node_exporter endpoint for scraping at start: custom configuration", func() {
		// given
		config := models.DeviceConfiguration{
			Metrics: &models.MetricsConfiguration{
				System: &models.SystemMetricsConfiguration{
					Interval:  duration,
					AllowList: customAllowList,
				},
			},
		}
		configProvider.EXPECT().GetDeviceConfiguration().Return(config)
		daemonMock.EXPECT().AddFilteredTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(duration)),
			gomock.Eq(customFilter))

		// when
		systemMetrics = metrics.NewSystemMetrics(daemonMock, configProvider)

		// then
		// daemonMock expectation met
	})

	It("should add node_exporter endpoint for scraping at start: custom configuration with interval unset", func() {
		// given
		config := models.DeviceConfiguration{
			Metrics: &models.MetricsConfiguration{
				System: &models.SystemMetricsConfiguration{
					AllowList: customAllowList,
				},
			},
		}
		configProvider.EXPECT().GetDeviceConfiguration().Return(config)
		daemonMock.EXPECT().AddFilteredTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
			gomock.Eq(customFilter))

		// when
		systemMetrics = metrics.NewSystemMetrics(daemonMock, configProvider)

		// then
		// daemonMock expectation met
	})

	It("should not re-add node_exporter endpoint for scraping when default configuration is used on start and in update", func() {
		// given
		noMetricsConfig :=
			models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{},
			}
		configProvider.EXPECT().GetDeviceConfiguration().Return(noMetricsConfig)

		daemonMock.EXPECT().AddFilteredTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
			gomock.Eq(defaultFilter)).
			Times(1)

		systemMetrics = metrics.NewSystemMetrics(daemonMock, configProvider)

		// when
		err := systemMetrics.Update(models.DeviceConfigurationMessage{Configuration: &noMetricsConfig})

		// then
		Expect(err).To(Not(HaveOccurred()))
		// daemonMock expectation met
	})

	It("should not re-add node_exporter endpoint for scraping when default configuration is used repeatedly in update", func() {
		// given
		noMetricsConfig :=
			models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{},
			}
		configProvider.EXPECT().GetDeviceConfiguration().Return(noMetricsConfig)

		daemonMock.EXPECT().AddFilteredTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
			gomock.Eq(defaultFilter)).
			Times(1)

		systemMetrics = metrics.NewSystemMetrics(daemonMock, configProvider)
		err := systemMetrics.Update(models.DeviceConfigurationMessage{Configuration: &noMetricsConfig})
		Expect(err).To(Not(HaveOccurred()))

		// when
		err = systemMetrics.Update(models.DeviceConfigurationMessage{Configuration: &noMetricsConfig})

		// then
		Expect(err).To(Not(HaveOccurred()))
		// daemonMock expectation met
	})

	It("should not re-add node_exporter endpoint for scraping when same custom configuration is used on start and in update", func() {
		// given
		config :=
			models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{
					System: &models.SystemMetricsConfiguration{
						Interval:  duration,
						AllowList: customAllowList,
					},
				},
			}
		configProvider.EXPECT().GetDeviceConfiguration().Return(config)
		daemonMock.EXPECT().AddFilteredTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(duration)),
			gomock.Eq(customFilter)).
			Times(1)

		systemMetrics = metrics.NewSystemMetrics(daemonMock, configProvider)
		err := systemMetrics.Update(models.DeviceConfigurationMessage{Configuration: &config})
		Expect(err).To(Not(HaveOccurred()))

		// when
		err = systemMetrics.Update(models.DeviceConfigurationMessage{Configuration: &config})

		// then
		Expect(err).To(Not(HaveOccurred()))
		// daemonMock expectation met
	})

	It("should not re-add node_exporter endpoint for scraping when same custom configuration is used repeatedly in update", func() {
		// given
		config :=
			models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{
					System: &models.SystemMetricsConfiguration{
						Interval:  duration,
						AllowList: customAllowList,
					},
				},
			}
		configProvider.EXPECT().GetDeviceConfiguration().Return(config)
		daemonMock.EXPECT().AddFilteredTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(duration)),
			gomock.Eq(customFilter)).
			Times(1)

		systemMetrics = metrics.NewSystemMetrics(daemonMock, configProvider)

		// when
		err := systemMetrics.Update(models.DeviceConfigurationMessage{Configuration: &config})

		// then
		Expect(err).To(Not(HaveOccurred()))
		// daemonMock expectation met
	})

	It("should re-add node_exporter endpoint for scraping when configuration changes from default to custom", func() {
		// given
		noMetricsConfig :=
			models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{},
			}
		configProvider.EXPECT().GetDeviceConfiguration().Return(noMetricsConfig)

		config :=
			models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{
					System: &models.SystemMetricsConfiguration{
						Interval:  duration,
						AllowList: customAllowList,
					},
				},
			}

		daemonMock.EXPECT().AddFilteredTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(duration)),
			gomock.Eq(customFilter)).
			Times(1).
			After(
				daemonMock.EXPECT().AddFilteredTarget(gomock.Any(),
					gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
					gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
					gomock.Eq(defaultFilter)).
					Times(1),
			)

		systemMetrics = metrics.NewSystemMetrics(daemonMock, configProvider)

		// when
		err := systemMetrics.Update(models.DeviceConfigurationMessage{Configuration: &config})

		// then
		Expect(err).To(Not(HaveOccurred()))
		// daemonMock expectation met
	})

	It("should re-add node_exporter endpoint for scraping when configuration changes from custom to default", func() {
		// given
		noMetricsConfig :=
			models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{},
			}

		config :=
			models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{
					System: &models.SystemMetricsConfiguration{
						Interval:  duration,
						AllowList: customAllowList,
					},
				},
			}
		configProvider.EXPECT().GetDeviceConfiguration().Return(config)

		daemonMock.EXPECT().AddFilteredTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
			gomock.Eq(defaultFilter)).
			Times(1).
			After(
				daemonMock.EXPECT().AddFilteredTarget(gomock.Any(),
					gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
					gomock.Eq(time.Second*time.Duration(duration)),
					gomock.Eq(customFilter)).
					Times(1),
			)

		systemMetrics = metrics.NewSystemMetrics(daemonMock, configProvider)

		// when
		err := systemMetrics.Update(models.DeviceConfigurationMessage{Configuration: &noMetricsConfig})

		// then
		Expect(err).To(Not(HaveOccurred()))
		// daemonMock expectation met
	})
})
