package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sync"

	"git.sr.ht/~spc/go-log"
	"github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"
	"github.com/pkg/errors"
)

const (
	DefaultRestartTimeout = 15
	PodSuffix             = "_pod"
	ServicePrefix         = "pod-"
	ServiceSuffix         = ".service"
	TimerSuffix           = ".timer"
)

var (
	DefaultUnitsPath = path.Join(os.Getenv("HOME"), ".config/systemd/user/")
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
	Units         []string          `json:"units"`
	UnitsContent  map[string]string `json:"-"`
	Rootless      bool              `json:"rootless"`
	servicePrefix string
	suffix        string
	podSuffix     string
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

func NewSystemd(name string, serviceNamePrefix string, units map[string]string) Service {
	return NewSystemdWithSuffix(name, serviceNamePrefix, PodSuffix, ServiceSuffix, units, true)
}

func newDbusConnection(rootless bool) (*dbus.Conn, error) {
	if rootless {
		return dbus.NewConnection(func() (*godbus.Conn, error) {
			uid := path.Base(os.Getenv("FLOTTA_XDG_RUNTIME_DIR"))
			path := filepath.Join(os.Getenv("FLOTTA_XDG_RUNTIME_DIR"), "systemd/private")
			conn, err := godbus.Dial(fmt.Sprintf("unix:path=%s", path))
			if err != nil {
				return nil, err
			}

			methods := []godbus.Auth{godbus.AuthExternal(uid)}

			err = conn.Auth(methods)
			if err != nil {
				if err = conn.Close(); err != nil {
					return nil, err
				}
				return nil, err
			}

			return conn, nil
		})
	} else {
		return dbus.NewSystemdConnectionContext(context.TODO())
	}
}

func NewSystemdWithSuffix(name string, serviceNamePrefix string, serviceNameSuffix string, serviceSuffix string, units map[string]string, rootless bool) Service {

	var unitNames []string
	for unit := range units {
		unitNames = append(unitNames, unit)
	}

	return &systemd{
		Name:          name,
		RestartSec:    DefaultRestartTimeout,
		Units:         unitNames,
		UnitsContent:  units,
		Rootless:      rootless,
		servicePrefix: serviceNamePrefix,
		suffix:        serviceSuffix,
		podSuffix:     serviceNameSuffix,
	}
}

func (s *systemd) Add() error {
	for unit, content := range s.UnitsContent {
		err := os.WriteFile(path.Join(DefaultUnitsPath, unit+s.suffix), []byte(content), 0644) //#nosec
		if err != nil {
			return err
		}
	}
	return s.reload()
}

func (s *systemd) Remove() error {
	for _, unit := range s.Units {
		err := os.Remove(path.Join(DefaultUnitsPath, unit+s.suffix))
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
	conn, err := newDbusConnection(s.Rootless)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.ReloadContext(context.Background())
}

func (s *systemd) Start() error {
	conn, err := newDbusConnection(s.Rootless)
	if err != nil {
		return err
	}
	defer conn.Close()
	startChan := make(chan string)
	if _, err := conn.StartUnitContext(context.Background(), s.serviceName(s.Name), "replace", startChan); err != nil {
		return err
	}

	result := <-startChan
	switch result {
	case "done":
		return nil
	default:
		return errors.Errorf("Failed[%s] to start systemd service %s", result, s.serviceName(s.Name))
	}
}

func (s *systemd) Stop() error {
	conn, err := newDbusConnection(s.Rootless)
	if err != nil {
		return err
	}
	defer conn.Close()
	stopChan := make(chan string)
	if _, err := conn.StopUnitContext(context.Background(), s.serviceName(s.Name), "replace", stopChan); err != nil {
		return err
	}

	result := <-stopChan
	switch result {
	case "done":
		return nil
	default:
		return errors.Errorf("Failed[%s] to stop systemd service %s", result, s.serviceName(s.Name))
	}
}

func (s *systemd) Enable() error {
	conn, err := newDbusConnection(s.Rootless)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, _, err = conn.EnableUnitFilesContext(context.Background(), []string{s.serviceName(s.Name)}, false, true)
	return err
}

func (s *systemd) Disable() error {
	conn, err := newDbusConnection(s.Rootless)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.DisableUnitFilesContext(context.Background(), []string{s.serviceName(s.Name)}, false)
	return err
}

func (s *systemd) serviceName(serviceName string) string {
	return s.servicePrefix + serviceName + s.podSuffix + s.suffix
}

func DefaultServiceName(serviceName string) string {
	return ServicePrefix + serviceName + PodSuffix + ServiceSuffix
}
