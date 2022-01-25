package metrics

import (
	"git.sr.ht/~spc/go-log"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/jakub-dzon/k4e-device-worker/internal/configuration"
	"github.com/jakub-dzon/k4e-device-worker/internal/service"
	"github.com/jakub-dzon/k4e-operator/models"
)

const (
	systemTargetName                     = "system"
	DefaultSystemMetricsScrapingInterval = 60
	NodeExporterMetricsEndpoint          = "http://localhost:9100/metrics"
)

var defaultSystemMetricsConfiguration = models.SystemMetricsConfiguration{Interval: DefaultSystemMetricsScrapingInterval}

type SystemMetrics struct {
	latestConfig atomic.Value
	daemon       MetricsDaemon
	config       configuration.Manager
	nodeExporter service.Service
}

func NewSystemMetrics(daemon MetricsDaemon) (*SystemMetrics, error) {
	nodeExporter, err := service.NewSystemd("node_exporter", "", nil)
	if err != nil {
		return nil, err
	}
	return NewSystemMetricsWithNodeExporter(daemon, nodeExporter), nil
}

func NewSystemMetricsWithNodeExporter(daemon MetricsDaemon, nodeExporter service.Service) *SystemMetrics {
	return &SystemMetrics{
		daemon:       daemon,
		nodeExporter: nodeExporter,
	}
}

func (sm *SystemMetrics) Init(config models.DeviceConfigurationMessage) error {
	return sm.Update(config)
}

func (sm *SystemMetrics) Update(config models.DeviceConfigurationMessage) error {
	newConfiguration := expectedConfiguration(config.Configuration)
	latestConfig := sm.latestConfig.Load()
	if latestConfig != nil {
		oldConfiguration := latestConfig.(*models.SystemMetricsConfiguration)
		if oldConfiguration != nil && reflect.DeepEqual(newConfiguration, *oldConfiguration) {
			return nil
		}
	}

	if err := sm.ensureNodeExporterState(newConfiguration); err != nil {
		return err
	}

	if newConfiguration.Disabled {
		sm.daemon.DeleteTarget(systemTargetName)
	} else {
		filter := getSampleFilter(newConfiguration.AllowList)
		sm.daemon.AddFilteredTarget(systemTargetName, []string{NodeExporterMetricsEndpoint}, time.Duration(newConfiguration.Interval)*time.Second, filter)
	}

	sm.latestConfig.Store(&newConfiguration)
	return nil
}

func (sm *SystemMetrics) Deregister() error {
	log.Info("stopping system metrics")
	sm.daemon.DeleteTarget("system")
	return nil
}

func expectedConfiguration(config *models.DeviceConfiguration) models.SystemMetricsConfiguration {
	newConfiguration := defaultSystemMetricsConfiguration
	if config.Metrics != nil && config.Metrics.System != nil {
		newConfiguration = *config.Metrics.System
		if newConfiguration.Interval == 0 {
			newConfiguration.Interval = DefaultSystemMetricsScrapingInterval
		}
	}
	return newConfiguration
}

func getSampleFilter(allowList *models.MetricsAllowList) SampleFilter {
	if allowList == nil {
		return DefaultSystemAllowList()
	}
	return NewRestrictiveAllowList(allowList)
}

func (sm *SystemMetrics) ensureNodeExporterDisabled() error {
	if err := sm.nodeExporter.Stop(); err != nil {
		return err
	}
	if err := sm.nodeExporter.Disable(); err != nil {
		return err
	}
	return nil
}

func (sm *SystemMetrics) ensureNodeExporterEnabled() error {
	if err := sm.nodeExporter.Enable(); err != nil {
		return err
	}
	if err := sm.nodeExporter.Start(); err != nil {
		return err
	}
	return nil
}

func (sm *SystemMetrics) ensureNodeExporterState(config models.SystemMetricsConfiguration) error {
	if config.Disabled {
		return sm.ensureNodeExporterDisabled()
	}
	return sm.ensureNodeExporterEnabled()
}
