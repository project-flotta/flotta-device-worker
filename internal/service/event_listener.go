package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.sr.ht/~spc/go-log"
	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/fsnotify/fsnotify"
)

const (
	EventStarted EventType = "started"
	EventStopped EventType = "stopped"

	systemdUserServicesDir         = ".config/systemd/user"
	defaultTargetWantsRelativePath = "default.target.wants/"

	// To avoid confusion, we match the action verb tense coming from systemd sub state, even though it is not consistent
	// The only addition to the list is the "removed" case, which we use when a service is removed
	// from the system and we receive an empty unitStatus object.

	//The service is running
	running unitSubState = "running"
	//The service is starting, probably due to a restart.
	//In this case we have to notify the observers that the service is not yet up and is loading. A subsequent 'running' event will proceed.
	start unitSubState = "start"
	//The service has been stopped, due to the container being killed. Systemd will restart the service.
	stop unitSubState = "stop"
	//The service is dead, due to the container being stopped for instance.
	dead unitSubState = "dead"
	//The service failed to start
	failed unitSubState = "failed"
	//The service has been removed, no unit state is returned from systemd
	removed unitSubState = "removed"
)

var (
	SystemdUserServicesFullPath = filepath.Join(os.Getenv("HOME"), systemdUserServicesDir)
	enabledServicesFullPath     = filepath.Join(SystemdUserServicesFullPath, defaultTargetWantsRelativePath)
)

type unitSubState string

type EventType string

type Event struct {
	WorkloadName string
	Type         EventType
}

type DBusEventListener struct {
	observerCh chan *Event
	set        *dbus.SubscriptionSet
	dbusCh     <-chan map[string]*dbus.UnitStatus
	dbusErrCh  <-chan error
	fsWatcher  *fsnotify.Watcher
}

func NewDBusEventListener() *DBusEventListener {
	return &DBusEventListener{}
}

func (e *DBusEventListener) Connect() (<-chan *Event, error) {
	conn, err := newDbusConnection(UserBus)
	if err != nil {
		return nil, err
	}
	e.set = conn.NewSubscriptionSet()
	if err := e.initializeSubscriptionSet(); err != nil {
		return nil, err
	}
	e.observerCh = make(chan *Event, 1000)
	return e.observerCh, nil
}

func (e *DBusEventListener) initializeSubscriptionSet() error {
	_, err := os.Stat(enabledServicesFullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return e.watchServiceDirectory()
		}
		return err
	}
	files, err := os.ReadDir(enabledServicesFullPath)
	if err != nil {
		return err
	}
	log.Infof("List of services to be monitored in %s:%s", enabledServicesFullPath, files)
	for _, fd := range files {
		f := filepath.Join(enabledServicesFullPath, fd.Name())
		log.Debugf("Detected service file %s", f)
		if !fileExist(f) {
			log.Errorf("Hard link %s does not exist or broken link", fd.Name())
			continue
		}
		e.Add(getServiceFileName(fd.Name()))
	}
	return e.watchServiceDirectory()
}

func (e *DBusEventListener) watchServiceDirectory() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	e.fsWatcher = watcher

	_, err = os.Stat(SystemdUserServicesFullPath)
	if err != nil {
		return fmt.Errorf("systemd user directory not found: %s", err)
	}

	go e.watchFSEvents()
	return e.fsWatcher.Add(SystemdUserServicesFullPath)
}

func (e *DBusEventListener) Listen() {
	e.dbusCh, e.dbusErrCh = e.set.Subscribe()
	for {
		select {
		case msg := <-e.dbusCh:
			for name, unit := range msg {
				log.Debugf("Captured DBus event for %s: %v+", name, unit)
				n, err := extractWorkloadName(name)
				if err != nil {
					log.Error(err)
					continue
				}
				state := translateUnitSubStatus(unit)
				log.Debugf("Systemd service for workload %s transitioned to sub state %s\n", n, state)
				switch state {
				case running:
					log.Debugf("Sending start event to observer channel for workload %s", n)
					e.observerCh <- &Event{WorkloadName: n, Type: EventStarted}
				case removed:
					// We remove the service from the filter here to avoid a potential race condition where the fs notify routine removes it before the systemd event is received.
					log.Infof("Service %s disabled or removed", name)
					e.Remove(name)
					fallthrough
				case stop, dead, failed, start:
					log.Debugf("Sending stop event to observer channel for workload %s", n)
					e.observerCh <- &Event{WorkloadName: n, Type: EventStopped}
				default:
					log.Debugf("Ignoring unit sub state for service %s: %s", name, unit.SubState)
				}
			}
		case msg := <-e.dbusErrCh:
			log.Errorf("Error while parsing dbus event: %v", msg)
		}
	}

}

func (e *DBusEventListener) watchFSEvents() {
	for {
		select {
		case event, ok := <-e.fsWatcher.Events:
			if !ok {
				log.Errorf("Error while watching for file system events at %s:%s", SystemdUserServicesFullPath, event)
				continue
			}
			log.Debugf("Captured file system event:%s", event)
			if !isAnEnabledService(event.Name) {
				continue
			}
			svcName := getServiceFileName(event.Name)
			switch {
			case event.Op&fsnotify.Create == fsnotify.Create:
				log.Infof("New systemd service unit file %s detected", event.Name)
				e.Add(svcName)
			case event.Op&fsnotify.Remove == fsnotify.Remove:
				log.Debugf("Systemd service unit file %s has been removed. Waiting for the systemd dbus remove event to elimitate it from the service watch filter", event.Name)
			}
		case err, ok := <-e.fsWatcher.Errors:
			if !ok {
				log.Errorf("Error detected on file system watcher: %s", err)
				return
			}
		}
	}
}

func translateUnitSubStatus(unit *dbus.UnitStatus) unitSubState {
	if unit == nil {
		return removed
	}
	return unitSubState(unit.SubState)
}

func (e *DBusEventListener) Add(serviceName string) {
	log.Debugf("Adding service for event listener %s", serviceName)
	e.set.Add(serviceName)
}

func (e *DBusEventListener) Remove(serviceName string) {
	log.Debugf("Removing service from event listener %s", serviceName)
	e.set.Remove(serviceName)
}

func extractWorkloadName(serviceName string) (string, error) {
	if filepath.Ext(serviceName) != ServiceSuffix {
		return "", fmt.Errorf("invalid file name or not a service %s", serviceName)
	}
	return strings.TrimSuffix(filepath.Base(serviceName), filepath.Ext(serviceName)), nil
}

func fileExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isAnEnabledService(fullPath string) bool {
	return filepath.Ext(fullPath) == ServiceSuffix && filepath.Dir(fullPath) == enabledServicesFullPath
}
func getServiceFileName(fullPath string) string {
	_, filename := filepath.Split(fullPath)
	return filename
}
