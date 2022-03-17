package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"

	"git.sr.ht/~spc/go-log"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/pkg/errors"
)

const (
	DefaultUnitsPath      = "/etc/systemd/system/"
	DefaultRestartTimeout = 15
	ServiceSuffix         = ".service"
)

//go:generate mockgen -package=service -destination=mock_systemd.go . Service
type Service interface {
	GetName() string
	Add() error
	Remove() error
	Start() error
	Stop() error
	Enable() error
	Disable() error
}

type systemd struct {
	Name           string            `json:"name"`
	RestartSec     int               `json:"restartSec"`
	Units          []string          `json:"units"`
	UnitsContent   map[string]string `json:"-"`
	servicePrefix  string
	dbusConnection *dbus.Conn `json:"-"`
}

//go:generate mockgen -package=service -destination=mock_systemd_manager.go . SystemdManager
type SystemdManager interface {
	Add(svc Service) error
	Get(name string) Service
	Remove(svc Service) error
	RemoveServicesFile() error
}

type systemdManager struct {
	svcFilePath string
	lock        sync.RWMutex
	services    map[string]Service
}

func NewSystemdManager(configDir string) (SystemdManager, error) {

	services := make(map[string]*systemd)

	servicePath := path.Join(configDir, "services.json")
	servicesJson, err := ioutil.ReadFile(servicePath) //#nosec
	if err == nil {
		err := json.Unmarshal(servicesJson, &services)
		if err != nil {
			return nil, fmt.Errorf("cannot unmarshal %v: %w", servicePath, err)
		}
	}

	systemdSVC := make(map[string]Service)
	for k, v := range services {
		systemdSVC[k] = v
	}

	return &systemdManager{svcFilePath: servicePath, services: systemdSVC, lock: sync.RWMutex{}}, nil
}

func (mgr *systemdManager) RemoveServicesFile() error {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	log.Infof("deleting %s file", mgr.svcFilePath)
	err := os.RemoveAll(mgr.svcFilePath)
	if err != nil {
		log.Errorf("failed to delete %s: %v", mgr.svcFilePath, err)
		return err
	}

	return nil
}

func (mgr *systemdManager) Add(svc Service) error {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	mgr.services[svc.GetName()] = svc

	return mgr.write()
}

func (mgr *systemdManager) Get(name string) Service {
	mgr.lock.RLock()
	defer mgr.lock.RUnlock()

	return mgr.services[name]
}

func (mgr *systemdManager) Remove(svc Service) error {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	delete(mgr.services, svc.GetName())

	return mgr.write()
}

func (mgr *systemdManager) write() error {
	svcJson, err := json.Marshal(mgr.services)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(mgr.svcFilePath, svcJson, 0640) //#nosec
	if err != nil {
		return err
	}
	return nil
}

func NewSystemd(name string, serviceNamePrefix string, units map[string]string) (Service, error) {
	conn, err := dbus.NewSystemdConnectionContext(context.TODO())
	if err != nil {
		return nil, err
	}

	var unitNames []string
	for unit := range units {
		unitNames = append(unitNames, unit)
	}

	return &systemd{
		Name:           name,
		RestartSec:     DefaultRestartTimeout,
		dbusConnection: conn,
		Units:          unitNames,
		UnitsContent:   units,
		servicePrefix:  serviceNamePrefix,
	}, nil
}

func (s *systemd) Add() error {
	for unit, content := range s.UnitsContent {
		err := os.WriteFile(path.Join(DefaultUnitsPath, unit+ServiceSuffix), []byte(content), 0644) //#nosec
		if err != nil {
			return err
		}
	}
	return s.reload()
}

func (s *systemd) Remove() error {
	for _, unit := range s.Units {
		err := os.Remove(path.Join(DefaultUnitsPath, unit+ServiceSuffix))
		if err != nil {
			return err
		}
	}

	return s.reload()
}

func (s *systemd) GetName() string {
	return s.Name
}

func (s *systemd) reload() error {
	return s.dbusConnection.ReloadContext(context.Background())
}

func (s *systemd) Start() error {
	startChan := make(chan string)
	if _, err := s.dbusConnection.StartUnitContext(context.Background(), s.serviceName(s.Name), "replace", startChan); err != nil {
		return err
	}

	result := <-startChan
	switch result {
	case "done":
		return nil
	default:
		return errors.Errorf("Failed to start systemd service %s", s.serviceName(s.Name))
	}
}

func (s *systemd) Stop() error {
	stopChan := make(chan string)
	if _, err := s.dbusConnection.StopUnitContext(context.Background(), s.serviceName(s.Name), "replace", stopChan); err != nil {
		return err
	}

	result := <-stopChan
	switch result {
	case "done":
		return nil
	default:
		return errors.Errorf("Failed to stop systemd service %s", s.serviceName(s.Name))
	}
}

func (s *systemd) Enable() error {
	_, _, err := s.dbusConnection.EnableUnitFilesContext(context.Background(), []string{s.serviceName(s.Name)}, false, true)
	return err
}

func (s *systemd) Disable() error {
	_, err := s.dbusConnection.DisableUnitFilesContext(context.Background(), []string{s.serviceName(s.Name)}, false)
	return err
}

func (s *systemd) serviceName(serviceName string) string {
	return s.servicePrefix + serviceName + ServiceSuffix
}
