package configuration

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"reflect"
	"sync/atomic"
	"time"

	"git.sr.ht/~spc/go-log"

	"github.com/hashicorp/go-multierror"
	"github.com/jakub-dzon/k4e-operator/models"
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

//go:generate mockgen -package=configuration -destination=configuration_mock.go . Observer
type Observer interface {
	Init(configuration models.DeviceConfigurationMessage) error
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
	log.Infof("device config file: %s", deviceConfigFile)
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
	// Always trigger the Init phase when register an observer to retrieve the
	// current config.
	err := observer.Init(*m.deviceConfiguration)
	if err != nil {
		log.Error("Running config init observer failed: ", err)
	}
	m.observers = append(m.observers, observer)
}

func (m *Manager) GetDeviceConfiguration() models.DeviceConfiguration {
	return *m.deviceConfiguration.Configuration
}

func (m *Manager) GetDeviceID() string {
	return m.deviceConfiguration.DeviceID
}

func (m *Manager) GetWorkloads() models.WorkloadList {
	return m.deviceConfiguration.Workloads
}

func (m *Manager) GetSecrets() models.SecretList {
	return m.deviceConfiguration.Secrets
}

func (m *Manager) Update(message models.DeviceConfigurationMessage) error {

	configurationEqual := reflect.DeepEqual(message.Configuration, m.deviceConfiguration.Configuration)
	workloadsEqual := reflect.DeepEqual(message.Workloads, m.deviceConfiguration.Workloads)
	secretsEqual := isEqualUnorderedSecretLists(message.Secrets, m.deviceConfiguration.Secrets)
	log.Tracef("workloads equal: [%v]; configurationEqual: [%v]; secretsEqual: [%v]; DeviceID: [%s]", workloadsEqual, configurationEqual, secretsEqual, message.DeviceID)

	shouldUpdate := !(configurationEqual && workloadsEqual && secretsEqual)

	if m.IsInitialConfig() {
		log.Trace("force update because it's init phase")
		shouldUpdate = true
	}

	if !shouldUpdate {
		log.Trace("configuration didn't change")
		return nil
	}

	log.Tracef("updating configuration: %v", message)
	var errors error
	for _, observer := range m.observers {
		err := observer.Update(message)
		if err != nil {
			errors = multierror.Append(errors, fmt.Errorf("running config observer failed: %s", err))
		}
	}

	file, err := json.MarshalIndent(message, "", " ")
	if err != nil {
		errors = multierror.Append(errors, fmt.Errorf("cannot unmarshal message JSON: %s", err))
		return errors
	}

	log.Tracef("writing config to %s: %v", m.deviceConfigFile, file)
	err = ioutil.WriteFile(m.deviceConfigFile, file, 0640)
	if err != nil {
		errors = multierror.Append(fmt.Errorf("cannot write device config file '%s': %s", m.deviceConfigFile, err))
		return errors
	}

	m.deviceConfiguration = &message
	m.initialConfig.Store(false)

	return errors
}

func (m *Manager) GetDataTransferInterval() time.Duration {
	return time.Second * 15
}

func (m *Manager) GetConfigurationVersion() string {
	version := m.deviceConfiguration.Version
	log.Tracef("configuration version: %v", version)
	return version
}

func (m *Manager) IsInitialConfig() bool {
	return m.initialConfig.Load().(bool)
}

func (m *Manager) Deregister() error {
	log.Infof("removing device config file: %s", m.deviceConfigFile)
	err := os.Remove(m.deviceConfigFile)
	if err != nil {
		log.Error(err)
		return err
	}
	return nil
}

func isEqualUnorderedSecretLists(x models.SecretList, y models.SecretList) bool {
	// nil and empty lists are considered equal. it's the contents that we care about
	if len(x) != len(y) {
		return false
	}
	secretsMap := map[string]*models.Secret{}
	for _, secret := range x {
		secretsMap[secret.Name] = secret
	}
	for _, secret := range y {
		otherSecret, ok := secretsMap[secret.Name]
		if !ok || !reflect.DeepEqual(secret, otherSecret) {
			return false
		}
	}
	return true
}
