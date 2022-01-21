package metrics

import (
	"fmt"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"

	"github.com/jakub-dzon/k4e-device-worker/internal/workload/podman"
	"github.com/jakub-dzon/k4e-operator/models"
)

const (
	defaultInterval int32 = 60
)

type WorkloadMetrics struct {
	latestConfig   models.WorkloadList
	daemon         MetricsDaemon
	workloadConfig map[string]*models.Workload
	lock           sync.RWMutex
}

func NewWorkloadMetrics(daemon MetricsDaemon) *WorkloadMetrics {
	return &WorkloadMetrics{daemon: daemon}
}

func (wrkM *WorkloadMetrics) getWorkload(workloadName string) *models.Workload {
	wrkM.lock.Lock()
	defer wrkM.lock.Unlock()
	return wrkM.workloadConfig[workloadName]
}

func (wrkM *WorkloadMetrics) Init(config models.DeviceConfigurationMessage) error {
	return wrkM.Update(config)
}

func (wrkM *WorkloadMetrics) Update(config models.DeviceConfigurationMessage) error {
	cfg := map[string]*models.Workload{}
	for _, workload := range config.Workloads {
		cfg[workload.Name] = workload
	}

	wrkM.lock.Lock()
	defer wrkM.lock.Unlock()
	wrkM.latestConfig = config.Workloads
	wrkM.workloadConfig = cfg
	log.Infof("metrics workload config update: %+v", cfg)
	return nil
}

func (wrkM *WorkloadMetrics) WorkloadRemoved(workloadName string) {
	log.Infof("removing target metrics for workload '%v'", workloadName)
	wrkM.daemon.DeleteTarget(workloadName)
	return
}

func (wrkM *WorkloadMetrics) WorkloadStarted(workloadName string, report []*podman.PodReport) {
	for _, workload := range report {
		cfg := wrkM.getWorkload(workloadName)
		if cfg == nil {
			log.Infof("workload '%v' started but it's not part of config", workloadName)
			continue
		}

		if cfg.Metrics == nil {
			continue
		}

		urls := []string{}
		for _, workloadReport := range report {
			urls = append(urls, getWorkloadUrls(workloadReport, cfg)...)
		}

		interval := defaultInterval
		if cfg.Metrics.Interval > 0 {
			interval = cfg.Metrics.Interval
		}
		// log for this is part of the AddTarget function
		wrkM.daemon.AddTarget(workload.Name, urls, time.Duration(interval)*time.Second)
	}
}

func getWorkloadUrls(report *podman.PodReport, config *models.Workload) []string {
	res := []string{}
	metricsPath := config.Metrics.Path
	port := config.Metrics.Port
	for _, container := range report.Containers {
		if customConfig, ok := config.Metrics.Containers[container.Name]; ok {
			if customConfig.Disabled {
				continue
			}
			res = append(res,
				fmt.Sprintf("http://%s:%d%s",
					container.IPAddress, customConfig.Port,
					getPathOrDefault(customConfig.Path)))
		} else {
			res = append(res,
				fmt.Sprintf("http://%s:%d%s",
					container.IPAddress, port,
					getPathOrDefault(metricsPath)))
		}
	}
	return res
}

func getPathOrDefault(path string) string {
	if path == "" {
		return "/"
	}
	return path
}
