package metrics

import (
	"fmt"
	"reflect"
	"sync/atomic"
	"time"

	"git.sr.ht/~spc/go-log"

	"github.com/project-flotta/flotta-device-worker/internal/datatransfer"
	"github.com/project-flotta/flotta-operator/models"
)

const (
	dataTransferTargetName                     = "data transfer"
	DefaultDataTransferMetricsScrapingInterval = 60
)

var defaultDataTransferMetricsConfiguration = models.ComponentMetricsConfiguration{Interval: DefaultDataTransferMetricsScrapingInterval}

type DataTransferMetrics struct {
	latestConfig atomic.Value
	daemon       MetricsDaemon
}

func NewDataTransferMetrics(daemon MetricsDaemon) *DataTransferMetrics {
	return &DataTransferMetrics{
		daemon: daemon,
	}
}

func (dt *DataTransferMetrics) String() string {
	return "data transfer metrics"
}

func (dt *DataTransferMetrics) Init(config models.DeviceConfigurationMessage) error {
	return dt.Update(config)
}

func (dt *DataTransferMetrics) Update(config models.DeviceConfigurationMessage) error {
	newConfiguration := retrieveConfigurationOrDefault(config.Configuration)
	update, err := dt.checkForUpdates(newConfiguration)
	if err != nil {
		return err
	}
	if update {
		dt.updateTarget(newConfiguration)
	}
	return nil
}

func (dt *DataTransferMetrics) checkForUpdates(newConfiguration models.ComponentMetricsConfiguration) (bool, error) {
	latestConfig := dt.latestConfig.Load()
	if latestConfig != nil {
		oldConfiguration, ok := latestConfig.(*models.ComponentMetricsConfiguration)
		if !ok {
			return false, fmt.Errorf("invalid type %+v", latestConfig)
		}
		if oldConfiguration != nil && reflect.DeepEqual(newConfiguration, *oldConfiguration) {
			// Same configuration, no updates required.
			return false, nil
		}
	}
	return true, nil
}

func (dt *DataTransferMetrics) updateTarget(newConfiguration models.ComponentMetricsConfiguration) {
	if newConfiguration.Disabled {
		dt.daemon.DeleteTarget(dataTransferTargetName)
	} else {
		filter := getDataTransferSampleFilter(newConfiguration.AllowList)
		dt.daemon.AddTarget(dataTransferTargetName, []string{datatransfer.MetricsEndpoint}, time.Duration(newConfiguration.Interval)*time.Second, filter)
	}
	dt.latestConfig.Store(&newConfiguration)
}

func (dt *DataTransferMetrics) Deregister() error {
	log.Infof("Stopping %s", dt.String())
	dt.daemon.DeleteTarget(dataTransferTargetName)
	return nil
}

func retrieveConfigurationOrDefault(config *models.DeviceConfiguration) models.ComponentMetricsConfiguration {
	newConfiguration := defaultDataTransferMetricsConfiguration
	if config.Metrics != nil && config.Metrics.DataTransfer != nil {
		newConfiguration = *config.Metrics.DataTransfer
		if newConfiguration.Interval == 0 {
			newConfiguration.Interval = DefaultDataTransferMetricsScrapingInterval
		}
	}
	return newConfiguration
}

func getDataTransferSampleFilter(allowList *models.MetricsAllowList) SampleFilter {
	if allowList == nil {
		return DefaultDataTransferAllowList()
	}
	return NewRestrictiveAllowList(allowList)
}
