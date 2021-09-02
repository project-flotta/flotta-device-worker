package configuration

import (
	"encoding/json"
	"git.sr.ht/~spc/go-log"
	"github.com/jakub-dzon/k4e-operator/models"
	"io/ioutil"
	"path"
	"reflect"
	"sync/atomic"
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

	observers        []Observer
	deviceConfigFile string
	initialConfig    atomic.Value
}

func NewConfigurationManager(dataDir string) *Manager {
	deviceConfigFile := path.Join(dataDir, "device-config.json")
	log.Infof("Device config file: %s", deviceConfigFile)
	file, err := ioutil.ReadFile(deviceConfigFile)
	var deviceConfiguration models.DeviceConfigurationMessage
	initialConfig := atomic.Value{}
	initialConfig.Store(false)
	if err != nil {
		log.Error(err)
		deviceConfiguration = defaultDeviceConfigurationMessage
		initialConfig.Store(true)
	} else {
		err = json.Unmarshal(file, &deviceConfiguration)
		if err != nil {
			log.Error(err)
			deviceConfiguration = defaultDeviceConfigurationMessage
		}
	}
	mgr := Manager{
		observers:           make([]Observer, 0),
		deviceConfigFile:    deviceConfigFile,
		deviceConfiguration: &deviceConfiguration,
		initialConfig:       initialConfig,
	}
	return &mgr
}

func (m *Manager) RegisterObserver(observer Observer) {
	m.observers = append(m.observers, observer)
}

func (m *Manager) GetDeviceConfiguration() models.DeviceConfiguration {
	return *m.deviceConfiguration.Configuration
}

func (m *Manager) GetWorkloads() models.WorkloadList{
	return m.deviceConfiguration.Workloads
}

func (m *Manager) Update(message models.DeviceConfigurationMessage) error {
	configurationEqual := reflect.DeepEqual(message.Configuration, m.deviceConfiguration.Configuration)
	workloadsEqual := reflect.DeepEqual(message.Workloads, m.deviceConfiguration.Workloads)
	log.Tracef("Initial config: [%v]; workloads equal: [%v]; configurationEqual: [%v]", m.IsInitialConfig(), workloadsEqual, configurationEqual)
	if m.IsInitialConfig() || !(configurationEqual && workloadsEqual) {
		log.Tracef("Updating configuration: %v", message)
		for _, observer := range m.observers {
			err := observer.Update(message)
			if err != nil {
				return err
			}
		}

		// TODO: handle all the failure scenarios correctly; i.e. compensate all the changes that has already been introduces.
		file, err := json.MarshalIndent(message, "", " ")
		if err != nil {
			return err
		}
		log.Tracef("Writing config to %s: %v", m.deviceConfigFile, file)
		err = ioutil.WriteFile(m.deviceConfigFile, file, 0640)
		if err != nil {
			log.Error(err)
			return err
		}
		m.deviceConfiguration = &message
		m.initialConfig.Store(false)
	} else {
		log.Trace("Configuration didn't change")
	}

	return nil
}

func (m *Manager) GetConfigurationVersion() string {
	version := m.deviceConfiguration.Version
	log.Tracef("Configuration version: %v", version)
	return version
}

func (m *Manager) IsInitialConfig() bool {
	return m.initialConfig.Load().(bool)
}
