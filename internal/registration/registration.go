package registration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/project-flotta/flotta-device-worker/internal/configuration"
	hardware2 "github.com/project-flotta/flotta-device-worker/internal/hardware"
	"github.com/project-flotta/flotta-device-worker/internal/workload"
	"github.com/project-flotta/flotta-operator/models"
	pb "github.com/redhatinsights/yggdrasil/protocol"
)

const (
	retryAfter = 10
)

//go:generate mockgen -package=registration -destination=mock_deregistrable.go . Deregistrable
type Deregistrable interface {
	fmt.Stringer
	Deregister() error
}

type Registration struct {
	hardware         *hardware2.Hardware
	workloads        *workload.WorkloadManager
	dispatcherClient pb.DispatcherClient
	config           *configuration.Manager
	registered       bool
	RetryAfter       int64
	deviceID         string
	lock             sync.RWMutex
	deregistrables   []Deregistrable
}

func NewRegistration(deviceID string, hardware *hardware2.Hardware, dispatcherClient DispatcherClient,
	config *configuration.Manager, workloadsManager *workload.WorkloadManager) *Registration {
	return &Registration{
		deviceID:         deviceID,
		hardware:         hardware,
		dispatcherClient: dispatcherClient,
		config:           config,
		RetryAfter:       retryAfter,
		workloads:        workloadsManager,
		lock:             sync.RWMutex{},
	}
}

func (r *Registration) DeregisterLater(deregistrables ...Deregistrable) {
	r.deregistrables = append(r.deregistrables, deregistrables...)
}

func (r *Registration) RegisterDevice() {
	err := r.registerDeviceOnce()

	if err != nil {
		log.Errorf("cannot register device. DeviceID: %s; err: %v", r.deviceID, err)
	}

	go r.registerDeviceWithRetries(r.RetryAfter)
}

func (r *Registration) registerDeviceWithRetries(interval int64) {
	ticker := time.NewTicker(time.Second * time.Duration(interval))
	for range ticker.C {
		if !r.config.IsInitialConfig() {
			ticker.Stop()
			break
		}
		log.Infof("configuration has not been initialized yet. Sending registration request. DeviceID: %s;", r.deviceID)
		err := r.registerDeviceOnce()
		if err != nil {
			log.Errorf("cannot register device. DeviceID: %s; err: %v", r.deviceID, err)
		}
	}
}

func (r *Registration) registerDeviceOnce() error {
	hardwareInformation, err := r.hardware.GetHardwareInformation()
	if err != nil {
		return err
	}
	registrationInfo := models.RegistrationInfo{
		Hardware: hardwareInformation,
	}
	content, err := json.Marshal(registrationInfo)
	if err != nil {
		return err
	}
	data := &pb.Data{
		MessageId: uuid.New().String(),
		Content:   content,
		Directive: "registration",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Call "Send"
	if _, err := r.dispatcherClient.Send(ctx, data); err != nil {
		return err
	}
	r.lock.Lock()
	r.registered = true
	r.lock.Unlock()
	return nil
}

func (r *Registration) IsRegistered() bool {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return r.registered
}

func (r *Registration) Deregister() error {
	var errors error
	for _, closer := range r.deregistrables {
		err := closer.Deregister()
		if err != nil {
			errors = multierror.Append(errors, fmt.Errorf("failed to deregister %s: %v", closer, err))
			log.Errorf("failed to deregister %s. DeviceID: %s; err: %v", closer, r.deviceID, err)
		}
	}

	r.registered = false
	return errors
}
