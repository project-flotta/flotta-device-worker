package service

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"git.sr.ht/~spc/go-log"
	"github.com/coreos/go-systemd/v22/dbus"
	godbus "github.com/godbus/dbus/v5"
	"github.com/pkg/errors"
)

const (
	DefaultRestartTimeout = 15
	TimerSuffix           = ".timer"
	ServiceSuffix         = ".service"
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
	ServiceExists() (bool, error)
}

type systemd struct {
	name         string
	Units        []string
	UnitsContent map[string]string
	BusType      BusType
}

//go:generate mockgen -package=service -destination=mock_systemd_manager.go . SystemdManager
type SystemdManager interface {
	Add(svc Service) error
	Get(name string) Service
	Remove(svc Service) error
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

func NewSystemd(name string, units map[string]string, busType BusType) Service {
	var unitNames []string
	for unit := range units {
		unitNames = append(unitNames, unit)
	}

	return &systemd{
		name: name,

		Units:        unitNames,
		BusType:      busType,
		UnitsContent: units,
	}

}

func (s *systemd) Add() error {
	if len(s.UnitsContent) == 0 {
		log.Infof("calling systemd add service for '%s' with no units available", s.GetName())
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

	conn, err := newDbusConnection(s.BusType)
	if err != nil {
		return err
	}
	// Retrieve dependency/bound services to the workload. It will return a slice with the services that are bound to this one
	p, err := conn.GetUnitPropertyContext(context.Background(), DefaultServiceName(s.GetName()), "BoundBy")
	if err != nil {
		return err
	}
	log.Debugf("List of dependent services to %s: %s", s.GetName(), p)
	v, ok := p.Value.Value().([]string)
	if !ok {
		return fmt.Errorf("invalid value %s for property BoundBy in service %s", p.Value.Value(), s.GetName())
	}
	// Delete the files that are bound to this service
	for _, unit := range v {
		log.Tracef("Deleting service unit configuration at %s", path.Join(DefaultUnitsPath, unit))
		err := os.Remove(path.Join(DefaultUnitsPath, unit))
		if err != nil {
			return err
		}
	}
	// Finally, remove the service file
	err = os.Remove(path.Join(DefaultUnitsPath, DefaultServiceName(s.GetName())))
	if err != nil {
		return err
	}
	return s.reload()
}

func (s *systemd) GetName() string {
	return s.name
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
	log.Debugf("Starting service %s", s.GetName())
	conn, err := newDbusConnection(s.BusType)
	if err != nil {
		return err
	}
	defer conn.Close()
	startChan := make(chan string)
	if _, err := conn.StartUnitContext(context.Background(), DefaultServiceName(s.GetName()), "replace", startChan); err != nil {
		return err
	}

	result := <-startChan
	switch result {
	case "done":
		return nil
	default:
		return errors.Errorf("Failed[%s] to start systemd service %s", result, DefaultServiceName(s.GetName()))
	}
}

func (s *systemd) Stop() error {

	conn, err := newDbusConnection(s.BusType)
	if err != nil {
		return err
	}
	defer conn.Close()
	stopChan := make(chan string)
	if _, err := conn.StopUnitContext(context.Background(), DefaultServiceName(s.GetName()), "replace", stopChan); err != nil {
		return err
	}

	result := <-stopChan
	switch result {
	case "done":
		return nil
	default:
		return errors.Errorf("Failed[%s] to stop systemd service %s", result, DefaultServiceName(s.GetName()))
	}
}

func (s *systemd) Enable() error {
	log.Debugf("Enabling service %s", s.GetName())
	conn, err := newDbusConnection(s.BusType)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, _, err = conn.EnableUnitFilesContext(context.Background(), []string{DefaultServiceName(s.GetName())}, false, true)
	return err
}

func (s *systemd) Disable() error {
	log.Debugf("Disabling service %s", s.GetName())
	conn, err := newDbusConnection(s.BusType)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.DisableUnitFilesContext(context.Background(), []string{DefaultServiceName(s.GetName())}, false)
	return err
}

func DefaultServiceName(serviceName string) string {
	return serviceName + ServiceSuffix
}

func (s *systemd) ServiceExists() (bool, error) {
	conn, err := newDbusConnection(s.BusType)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	units, err := conn.ListUnitsByNamesContext(context.Background(), []string{DefaultServiceName(s.GetName())})
	if err != nil {
		return false, err
	}
	log.Tracef("Number of matching units %d:%+v", len(units), units)
	exists := len(units) == 1 && units[0].LoadState != "not-found"
	if !exists {
		log.Tracef("Service %s not found ", s.GetName())
	}
	return exists, nil
}
