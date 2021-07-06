package configuration

import (
	"git.sr.ht/~spc/go-log"
	"github.com/jakub-dzon/k4e-operator/models"
	"reflect"
)

var (
	defaultHeartbeatConfiguration = models.HeartbeatConfiguration{
		HardwareProfile: &models.HardwareProfileConfiguration{},
		PeriodSeconds:   60,
	}
	defaultDeviceConfiguration = models.DeviceConfiguration{
		Heartbeat: &defaultHeartbeatConfiguration,
	}
	defaultDeviceConfigurationMessage = models.DeviceConfigurationMessage{
		Configuration: &defaultDeviceConfiguration,
	}
)

type Observer interface {
	Update(configuration models.DeviceConfigurationMessage) error
}

type Manager struct {
	deviceConfiguration *models.DeviceConfigurationMessage

	observers []Observer
}

func NewConfigurationManager() *Manager {
	mgr := Manager{
		deviceConfiguration: &defaultDeviceConfigurationMessage,
		observers:           make([]Observer, 0),
	}
	return &mgr
}

func (m *Manager) RegisterObserver(observer Observer) {
	m.observers = append(m.observers, observer)
}

func (m *Manager) GetDeviceConfiguration() models.DeviceConfiguration {
	return *m.deviceConfiguration.Configuration
}

func (m *Manager) Update(message models.DeviceConfigurationMessage) error {
	if !(reflect.DeepEqual(message.Configuration, m.deviceConfiguration.Configuration) ||
		!reflect.DeepEqual(message.Workloads, m.deviceConfiguration.Workloads)) {
		log.Info("Updating configuration: %v", message)
		for _, observer := range m.observers {
			err := observer.Update(message)
			if err != nil {
				return err
			}
		}
		m.deviceConfiguration = &message
	} else {
		log.Info("Configuration didn't change")
	}

	return nil
}

func (m *Manager) GetConfigurationVersion() string {
	version := m.deviceConfiguration.Version
	log.Infof("Configuration version: %v", version)
	return version
}
