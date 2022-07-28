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

	"github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	DefaultRestartTimeout = 15
	TimerSuffix           = ".timer"
	ServiceSuffix         = ".service"
	DefaultNameSeparator  = "-"
)

var (
	DefaultUnitsPath = path.Join(os.Getenv("HOME"), ".config/systemd/user/")
)

type BusType string

const (
	UserBus   BusType = "user"
	SystemBus BusType = "system"
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
	dbusConnection *dbus.Conn        `json:"-"`
	BusType        BusType           `json:"busType"`
	eventCh        chan Event        `json:"-"`
}

//go:generate mockgen -package=service -destination=mock_systemd_manager.go . SystemdManager
type SystemdManager interface {
	Add(svc Service) error
	Get(name string) Service
	Remove(svc Service) error
	RemoveServicesFile() error
}

type systemdManager struct {
	svcFilePath   string
	lock          sync.RWMutex
	services      map[string]Service
	eventListener *EventListener
}

func NewSystemdManager(configDir string, observerCh chan *Event) (SystemdManager, error) {
	services := make(map[string]*systemd)
	servicePath := path.Join(configDir, "services.json")
	servicesJson, err := os.ReadFile(servicePath) //#nosec
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
	listener := NewEventListener(observerCh)
	err = listener.Connect()
	if err != nil {
		return nil, err
	}
	for name := range services {
		listener.Add(name)
	}
	listener.Listen()
	return &systemdManager{svcFilePath: servicePath, services: systemdSVC, lock: sync.RWMutex{}, eventListener: listener}, nil
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
	err := mgr.write()
	if err != nil {
		return err
	}
	mgr.eventListener.Add(svc.GetName())
	return nil
}

func (mgr *systemdManager) Get(name string) Service {
	mgr.lock.RLock()
	defer mgr.lock.RUnlock()

	return mgr.services[name]
}

func (mgr *systemdManager) Remove(svc Service) error {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()
	err := mgr.write()
	if err != nil {
		return err
	}
	delete(mgr.services, svc.GetName())
	mgr.eventListener.Remove(svc.GetName())
	return nil
}

func (mgr *systemdManager) write() error {
	svcJson, err := json.Marshal(mgr.services)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(mgr.svcFilePath, svcJson, 0640) //#nosec
}

func newDbusConnection(busType BusType) (*dbus.Conn, error) {
	if busType == UserBus {
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

func NewSystemd(name string, units map[string]string, busType BusType, eventCh chan Event) (Service, error) {
	var err error
	var conn *dbus.Conn

	conn, err = newDbusConnection(busType)
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
		BusType:        busType,
		UnitsContent:   units,
		eventCh:        eventCh,
	}, nil
}

func (s *systemd) Add() error {
	if len(s.UnitsContent) == 0 {
		log.Infof("calling systemd add service for '%s' with no units available", s.Name)
	}

	for unit, content := range s.UnitsContent {
		targetPath := path.Join(DefaultUnitsPath, DefaultServiceName(unit))
		err := os.WriteFile(targetPath, []byte(content), 0644) //#nosec
		if err != nil {
			return err
		}
		log.Infof("writing new systemd file for '%s' on '%s'", unit, targetPath)
	}
	return s.reload()
}

func (s *systemd) Remove() error {
	for _, unit := range s.Units {
		err := os.Remove(path.Join(DefaultUnitsPath, DefaultServiceName(unit)))
		if err != nil {
			return err
		}
	}
	s.eventCh <- Event{WorkloadName: s.Name, Type: EventStopped}
	return s.reload()
}

func (s *systemd) GetName() string {
	return s.Name
}

func (s *systemd) reload() error {
	conn, err := newDbusConnection(s.BusType)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.ReloadContext(context.Background())
}

func (s *systemd) Start() error {
	conn, err := newDbusConnection(s.BusType)
	if err != nil {
		return err
	}
	defer conn.Close()
	startChan := make(chan string)
	if _, err := conn.StartUnitContext(context.Background(), DefaultServiceName(s.Name), "replace", startChan); err != nil {
		return err
	}

	result := <-startChan
	switch result {
	case "done":
		return nil
	default:
		return errors.Errorf("Failed[%s] to start systemd service %s", result, DefaultServiceName(s.Name))
	}
}

func (s *systemd) Stop() error {
	conn, err := newDbusConnection(s.BusType)
	if err != nil {
		return err
	}
	defer conn.Close()
	stopChan := make(chan string)
	if _, err := conn.StopUnitContext(context.Background(), DefaultServiceName(s.Name), "replace", stopChan); err != nil {
		return err
	}

	result := <-stopChan
	switch result {
	case "done":
		return nil
	default:
		return errors.Errorf("Failed[%s] to stop systemd service %s", result, DefaultServiceName(s.Name))
	}
}

func (s *systemd) Enable() error {
	conn, err := newDbusConnection(s.BusType)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, _, err = conn.EnableUnitFilesContext(context.Background(), []string{DefaultServiceName(s.Name)}, false, true)
	return err
}

func (s *systemd) Disable() error {
	conn, err := newDbusConnection(s.BusType)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.DisableUnitFilesContext(context.Background(), []string{DefaultServiceName(s.Name)}, false)
	return err
}

func DefaultServiceName(serviceName string) string {
	return serviceName + ServiceSuffix
}
