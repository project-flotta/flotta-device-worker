package service

import (
	"fmt"

	"git.sr.ht/~spc/go-log"
	"github.com/coreos/go-systemd/v22/dbus"
)

const (
	// servicePostfixLength is the length of the string ".service". This value is used to extract the workload name from the service
	// coming from systemd
	servicePostfixLength = 8

	EventStarted EventType = "started"
	EventStopped EventType = "stopped"

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

type unitSubState string

type EventType string

type Event struct {
	WorkloadName string
	Type         EventType
}

type EventListener struct {
	observerCh chan *Event
	set        *dbus.SubscriptionSet
	dbusCh     <-chan map[string]*dbus.UnitStatus
	dbusErrCh  <-chan error
}

func NewEventListener(observerCh chan *Event) *EventListener {
	return &EventListener{observerCh: observerCh}
}

func (e *EventListener) Connect() error {
	conn, err := newDbusConnection(true)
	if err != nil {
		return err
	}
	e.set = conn.NewSubscriptionSet()
	return nil
}

func (e *EventListener) Listen() {
	e.dbusCh, e.dbusErrCh = e.set.Subscribe()
	go func() {
		for {
			select {
			case msg := <-e.dbusCh:
				for name, unit := range msg {
					log.Debugf("Captured event for %s: %v+", name, unit)
					n := extractWorkloadName(name)
					state := translateUnitSubStatus(unit)
					log.Infof("Service %s for workload transitioned to sub state %s\n", n, state)
					switch state {
					case running:
						e.observerCh <- &Event{WorkloadName: n, Type: EventStarted}
					case removed, stop, dead, failed, start:
						e.observerCh <- &Event{WorkloadName: n, Type: EventStopped}
					default:
						log.Infof("Ignoring unit sub state for service %s: %s", name, unit.SubState)
					}
				}
			case msg := <-e.dbusErrCh:
				log.Errorf("Error while parsing dbus event: %v", msg)
			}
		}
	}()
}

func translateUnitSubStatus(unit *dbus.UnitStatus) unitSubState {
	if unit == nil {
		return removed
	}
	return unitSubState(unit.SubState)
}

func (e *EventListener) Add(workloadName string) {
	name := fmt.Sprintf("%s.service", workloadName)
	log.Debugf("Adding service for events %s", name)
	if !e.set.Contains(name) {
		e.set.Add(name)
	}
}

func (e *EventListener) Remove(workloadName string) {
	name := fmt.Sprintf("%s.service", workloadName)
	log.Debugf("Removing service for events %s", name)
	e.set.Remove(name)
}

func extractWorkloadName(serviceName string) string {
	return serviceName[:len(serviceName)-servicePostfixLength]
}
