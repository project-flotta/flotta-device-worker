package metrics_test

import (
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/metrics"
	"github.com/project-flotta/flotta-device-worker/internal/service"
	"github.com/project-flotta/flotta-operator/models"
)

var _ = Describe("System", func() {

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
					System: &models.ComponentMetricsConfiguration{
						Interval:  duration,
						AllowList: customAllowList,
					},
				},
			},
		}
		defaultFilter = metrics.DefaultSystemAllowList()

		mockCtrl         *gomock.Controller
		daemonMock       *metrics.MockMetricsDaemon
		nodeExporterMock *service.MockService
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		daemonMock = metrics.NewMockMetricsDaemon(mockCtrl)
		nodeExporterMock = service.NewMockService(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	It("should add node_exporter endpoint for scraping at init: default configuration", func() {
		// given
		daemonMock.EXPECT().AddTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
			gomock.Eq(defaultFilter)).
			Times(1)
		nodeExporterMock.EXPECT().Enable()
		nodeExporterMock.EXPECT().Start()
		sm := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)

		// when
		err := sm.Init(noMetricsConfig)

		// then
		Expect(err).ToNot(HaveOccurred())
		// daemonMock expectation met
	})

	It("should add node_exporter endpoint for scraping at init: custom configuration", func() {
		// given
		daemonMock.EXPECT().AddTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(duration)),
			gomock.Eq(customFilter))
		nodeExporterMock.EXPECT().Enable()
		nodeExporterMock.EXPECT().Start()
		sm := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)

		// when
		err := sm.Init(customConfig)

		// then
		Expect(err).ToNot(HaveOccurred())
		// daemonMock expectation met
	})

	It("should add node_exporter endpoint for scraping at init: custom configuration with interval unset", func() {
		// given
		config := models.DeviceConfigurationMessage{
			Configuration: &models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{
					System: &models.ComponentMetricsConfiguration{
						AllowList: customAllowList,
					},
				},
			},
		}
		daemonMock.EXPECT().AddTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
			gomock.Eq(customFilter))
		nodeExporterMock.EXPECT().Enable()
		nodeExporterMock.EXPECT().Start()

		sm := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)

		// when
		err := sm.Init(config)

		// then
		Expect(err).ToNot(HaveOccurred())
		// daemonMock expectation met
	})

	It("should not re-add node_exporter endpoint for scraping when default configuration is used on init and in update", func() {
		// given
		daemonMock.EXPECT().AddTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
			gomock.Eq(defaultFilter)).
			Times(1)
		nodeExporterMock.EXPECT().Enable()
		nodeExporterMock.EXPECT().Start()

		systemMetrics := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)
		err := systemMetrics.Init(noMetricsConfig)
		Expect(err).ToNot(HaveOccurred())

		// when
		err = systemMetrics.Update(noMetricsConfig)

		// then
		Expect(err).To(Not(HaveOccurred()))
		// daemonMock expectation met
	})

	It("should not re-add node_exporter endpoint for scraping when default configuration is used repeatedly in update", func() {
		// given
		daemonMock.EXPECT().AddTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
			gomock.Eq(defaultFilter)).
			Times(1)
		nodeExporterMock.EXPECT().Enable()
		nodeExporterMock.EXPECT().Start()

		systemMetrics := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)
		err := systemMetrics.Init(noMetricsConfig)
		Expect(err).ToNot(HaveOccurred())
		err = systemMetrics.Update(noMetricsConfig)
		Expect(err).To(Not(HaveOccurred()))

		// when
		err = systemMetrics.Update(noMetricsConfig)

		// then
		Expect(err).To(Not(HaveOccurred()))
		// daemonMock expectation met
	})

	It("should not re-add node_exporter endpoint for scraping when same custom configuration is used on init and in update", func() {
		// given
		daemonMock.EXPECT().AddTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(duration)),
			gomock.Eq(customFilter)).
			Times(1)

		nodeExporterMock.EXPECT().Enable()
		nodeExporterMock.EXPECT().Start()

		systemMetrics := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)
		err := systemMetrics.Init(customConfig)
		Expect(err).ToNot(HaveOccurred())
		err = systemMetrics.Update(customConfig)
		Expect(err).To(Not(HaveOccurred()))

		// when
		err = systemMetrics.Update(customConfig)

		// then
		Expect(err).To(Not(HaveOccurred()))
		// daemonMock expectation met
	})

	It("should not re-add node_exporter endpoint for scraping when same custom configuration is used repeatedly in update", func() {
		// given
		daemonMock.EXPECT().AddTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(duration)),
			gomock.Eq(customFilter)).
			Times(1)

		nodeExporterMock.EXPECT().Enable()
		nodeExporterMock.EXPECT().Start()

		systemMetrics := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)
		err := systemMetrics.Init(customConfig)
		Expect(err).ToNot(HaveOccurred())

		// when
		err = systemMetrics.Update(customConfig)

		// then
		Expect(err).To(Not(HaveOccurred()))
		// daemonMock expectation met
	})

	It("should re-add node_exporter endpoint for scraping when configuration changes from default to custom", func() {
		// given
		daemonMock.EXPECT().AddTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(duration)),
			gomock.Eq(customFilter)).
			Times(1).
			After(
				daemonMock.EXPECT().AddTarget(gomock.Any(),
					gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
					gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
					gomock.Eq(defaultFilter)).
					Times(1),
			)
		nodeExporterMock.EXPECT().Enable().Times(2)
		nodeExporterMock.EXPECT().Start().Times(2)

		systemMetrics := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)
		err := systemMetrics.Init(noMetricsConfig)

		// when
		Expect(err).ToNot(HaveOccurred())
		err = systemMetrics.Update(customConfig)

		// then
		Expect(err).To(Not(HaveOccurred()))
		// daemonMock expectation met
	})

	It("should re-add node_exporter endpoint for scraping when configuration changes from custom to default", func() {
		// given
		daemonMock.EXPECT().AddTarget(gomock.Any(),
			gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
			gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
			gomock.Eq(defaultFilter)).
			Times(1).
			After(
				daemonMock.EXPECT().AddTarget(gomock.Any(),
					gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
					gomock.Eq(time.Second*time.Duration(duration)),
					gomock.Eq(customFilter)).
					Times(1),
			)
		nodeExporterMock.EXPECT().Enable().Times(2)
		nodeExporterMock.EXPECT().Start().Times(2)

		systemMetrics := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)
		err := systemMetrics.Init(customConfig)

		// when
		Expect(err).ToNot(HaveOccurred())
		err = systemMetrics.Update(noMetricsConfig)

		// then
		Expect(err).To(Not(HaveOccurred()))
		// daemonMock expectation met
	})

	It("should disable node_exporter at init", func() {
		// given
		config := models.DeviceConfigurationMessage{
			Configuration: &models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{
					System: &models.ComponentMetricsConfiguration{
						Disabled: true,
					},
				},
			},
		}
		daemonMock.EXPECT().DeleteTarget("system")
		nodeExporterMock.EXPECT().Disable()
		nodeExporterMock.EXPECT().Stop()

		sm := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)

		// when
		err := sm.Init(config)

		// then
		Expect(err).ToNot(HaveOccurred())
		// daemonMock expectation met
	})

	It("should enabling node_exporter at init fail on starting", func() {
		// given
		nodeExporterMock.EXPECT().Enable()
		nodeExporterMock.EXPECT().Start().Return(fmt.Errorf("boom"))

		sm := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)

		// when
		err := sm.Init(noMetricsConfig)

		// then
		Expect(err).To(HaveOccurred())
		// daemonMock expectation met
	})

	It("should enabling node_exporter at init fail", func() {
		// given
		nodeExporterMock.EXPECT().Enable().Return(fmt.Errorf("boom"))

		sm := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)

		// when
		err := sm.Init(noMetricsConfig)

		// then
		Expect(err).To(HaveOccurred())
		// daemonMock expectation met
	})

	It("should disable node_exporter at init fail on disabling", func() {
		// given
		config := models.DeviceConfigurationMessage{
			Configuration: &models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{
					System: &models.ComponentMetricsConfiguration{
						Disabled: true,
					},
				},
			},
		}
		nodeExporterMock.EXPECT().Stop()
		nodeExporterMock.EXPECT().Disable().Return(fmt.Errorf("boom"))

		sm := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)

		// when
		err := sm.Init(config)

		// then
		Expect(err).To(HaveOccurred())
		// daemonMock expectation met
	})

	It("should disable node_exporter at init fail on stopping", func() {
		// given
		config := models.DeviceConfigurationMessage{
			Configuration: &models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{
					System: &models.ComponentMetricsConfiguration{
						Disabled: true,
					},
				},
			},
		}
		nodeExporterMock.EXPECT().Stop().Return(fmt.Errorf("boom"))

		sm := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)

		// when
		err := sm.Init(config)

		// then
		Expect(err).To(HaveOccurred())
		// daemonMock expectation met
	})

	It("should disable node_exporter at update", func() {
		// given
		daemonMock.EXPECT().DeleteTarget("system").After(
			daemonMock.EXPECT().AddTarget(gomock.Any(),
				gomock.Eq([]string{metrics.NodeExporterMetricsEndpoint}),
				gomock.Eq(time.Second*time.Duration(metrics.DefaultSystemMetricsScrapingInterval)),
				gomock.Eq(defaultFilter)).
				Times(1),
		)
		nodeExporterMock.EXPECT().Disable().
			After(
				nodeExporterMock.EXPECT().Enable(),
			)

		nodeExporterMock.EXPECT().Stop().
			After(
				nodeExporterMock.EXPECT().Start(),
			)

		config := models.DeviceConfigurationMessage{
			Configuration: &models.DeviceConfiguration{
				Metrics: &models.MetricsConfiguration{
					System: &models.ComponentMetricsConfiguration{
						Disabled: true,
					},
				},
			},
		}

		sm := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)
		err := sm.Init(noMetricsConfig)
		Expect(err).ToNot(HaveOccurred())

		// when
		err = sm.Update(config)

		// then
		Expect(err).ToNot(HaveOccurred())
		// daemonMock expectation met
	})

	It("should delete system target on de-registration", func() {
		// given
		daemonMock.EXPECT().DeleteTarget("system")
		sm := metrics.NewSystemMetricsWithNodeExporter(daemonMock, nodeExporterMock)

		// when
		err := sm.Deregister()

		// then
		Expect(err).ToNot(HaveOccurred())
		// daemonMock expectation met
	})
})
