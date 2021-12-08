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
	DefaultServiceSuffix  = ".service"
	DefaultServicePrefix  = "pod-"
)

type Systemd struct {
	Name         string            `json:"name"`
	RestartSec   int               `json:"restartSec"`
	Path         string            `json:"path"`
	Units        []string          `json:"units"`
	UnitsContent map[string]string `json:"-"`
}

type SystemdManager struct {
	svcFilePath string
	lock        sync.RWMutex
	services    map[string]*Systemd
}

func NewSystemdManager(configDir string) (*SystemdManager, error) {
	services := make(map[string]*Systemd)

	servicePath := path.Join(configDir, "services.json")
	servicesJson, err := ioutil.ReadFile(servicePath)
	if err == nil {
		err := json.Unmarshal(servicesJson, &services)
		if err != nil {
			return nil, err
		}
	}
	return &SystemdManager{svcFilePath: servicePath, services: services, lock: sync.RWMutex{}}, nil
}

func (mgr *SystemdManager) Add(svc *Systemd) error {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	mgr.services[svc.Name] = svc

	return mgr.write()
}

func (mgr *SystemdManager) Get(name string) *Systemd {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	return mgr.services[name]
}

func (mgr *SystemdManager) Remove(svc *Systemd) error {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	delete(mgr.services, svc.Name)

	return mgr.write()
}

func (mgr *SystemdManager) write() error {
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

func NewSystemd(name string, units map[string]string) (*Systemd, error) {
	path, err := exec.LookPath("systemctl")
	if err != nil {
		return nil, err
	}

	var unitNames []string
	for unit := range units {
		unitNames = append(unitNames, unit)
	}

	return &Systemd{
		Name:         name,
		RestartSec:   DefaultRestartTimeout,
		Path:         path,
		Units:        unitNames,
		UnitsContent: units,
	}, nil
}

func (svc *Systemd) Add() error {
	for unit, content := range svc.UnitsContent {
		err := os.WriteFile(path.Join(DefaultUnitsPath, unit+DefaultServiceSuffix), []byte(content), 0644)
		if err != nil {
			return err
		}
	}
	return svc.reload()
}

func (svc *Systemd) Remove() error {
	for _, unit := range svc.Units {
		err := os.Remove(path.Join(DefaultUnitsPath, unit+DefaultServiceSuffix))
		if err != nil {
			return err
		}
	}

	return svc.reload()
}

func (s *Systemd) Start() error {
	return s.run([]string{"start", serviceName(s.Name)})
}

func (s *Systemd) Stop() error {
	return s.run([]string{"stop", serviceName(s.Name)})
}

func (s *Systemd) Enable() error {
	return s.run([]string{"enable", serviceName(s.Name)})
}

func serviceName(serviceName string) string {
	return DefaultServicePrefix + serviceName + DefaultServiceSuffix
}

func (s *Systemd) reload() error {
	if err := s.run([]string{"daemon-reload"}); err != nil {
		return err
	}

	if err := s.run([]string{"reset-failed"}); err != nil {
		return err
	}

	return nil
}

func (s *Systemd) run(args []string) error {
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
