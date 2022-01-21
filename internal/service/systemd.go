package service

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"sync"

	"git.sr.ht/~spc/go-log"
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
	Name          string            `json:"name"`
	RestartSec    int               `json:"restartSec"`
	Path          string            `json:"path"`
	Units         []string          `json:"units"`
	UnitsContent  map[string]string `json:"-"`
	servicePrefix string
}

//go:generate mockgen -package=service -destination=mock_systemd_manager.go . SystemdManager
type SystemdManager interface {
	Add(svc Service) error
	Get(name string) Service
	Remove(svc Service) error
}

type systemdManager struct {
	svcFilePath string
	lock        sync.RWMutex
	services    map[string]Service
}

func NewSystemdManager(configDir string) (SystemdManager, error) {
	services := make(map[string]Service)

	servicePath := path.Join(configDir, "services.json")
	servicesJson, err := ioutil.ReadFile(servicePath)
	if err == nil {
		err := json.Unmarshal(servicesJson, &services)
		if err != nil {
			return nil, err
		}
	}
	return &systemdManager{svcFilePath: servicePath, services: services, lock: sync.RWMutex{}}, nil
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
	err = ioutil.WriteFile(mgr.svcFilePath, svcJson, 0640)
	if err != nil {
		return err
	}
	return nil
}

func NewSystemd(name string, serviceNamePrefix string, units map[string]string) (Service, error) {
	path, err := exec.LookPath("systemctl")
	if err != nil {
		return nil, err
	}

	var unitNames []string
	for unit := range units {
		unitNames = append(unitNames, unit)
	}

	return &systemd{
		Name:          name,
		RestartSec:    DefaultRestartTimeout,
		Path:          path,
		Units:         unitNames,
		UnitsContent:  units,
		servicePrefix: serviceNamePrefix,
	}, nil
}

func (s *systemd) Add() error {
	for unit, content := range s.UnitsContent {
		err := os.WriteFile(path.Join(DefaultUnitsPath, unit+ServiceSuffix), []byte(content), 0644)
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

func (s *systemd) Start() error {
	return s.run([]string{"start", s.serviceName(s.Name)})
}

func (s *systemd) Stop() error {
	return s.run([]string{"stop", s.serviceName(s.Name)})
}

func (s *systemd) Enable() error {
	return s.run([]string{"enable", s.serviceName(s.Name)})
}

func (s *systemd) Disable() error {
	return s.run([]string{"disable", s.serviceName(s.Name)})
}

func (s *systemd) serviceName(serviceName string) string {
	return s.servicePrefix + serviceName + ServiceSuffix
}

func (s *systemd) reload() error {
	if err := s.run([]string{"daemon-reload"}); err != nil {
		return err
	}

	if err := s.run([]string{"reset-failed"}); err != nil {
		return err
	}

	return nil
}

func (s *systemd) run(args []string) error {
	args = append([]string{s.Path}, args...)
	cmd := exec.Cmd{
		Path: s.Path,
		Args: args,
	}

	log.Infof("Executing systemd command %v ", cmd)
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
