package deregistration

import (
	"git.sr.ht/~spc/go-log"
	"github.com/jakub-dzon/k4e-device-worker/internal/configuration"
	"github.com/jakub-dzon/k4e-operator/models"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload"
	"github.com/jakub-dzon/k4e-device-worker/internal/heartbeat"
)

type Deregistration struct {
	workloads    *workload.WorkloadManager
	config       *configuration.Manager
	heartbeat	*heartbeat.Heartbeat
}

func NewDeregistration(workloadsManager *workload.WorkloadManager, configManager *configuration.Manager, heartbeatManager *heartbeat.Heartbeat) (*Deregistration, error) {
	deregstration := Deregistration{
		workloads:                   workloadsManager,
		config:                      configManager,
		heartbeat:                      heartbeatManager,
	}
	return &deregstration, nil
}

func (d *Deregistration) Update(configuration models.DeviceConfigurationMessage) error {
	err := d.workloads.Deregister()
	if err != nil {
		log.Errorf("failed to delete device-config file: %v", err)
	}

	err = d.config.Deregister()
	if err != nil {
		log.Errorf("failed to delete device-config file: %v", err)
	}

	err = d.heartbeat.Deregister()
	if err != nil {
		log.Errorf("failed to stop heartbeat: %v", err)
	}
	return nil
}
