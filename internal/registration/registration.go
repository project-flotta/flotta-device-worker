package registration

import (
	"context"
	"encoding/json"
	"git.sr.ht/~spc/go-log"
	"github.com/google/uuid"
	"github.com/jakub-dzon/k4e-device-worker/internal/configuration"
	hardware2 "github.com/jakub-dzon/k4e-device-worker/internal/hardware"
	os2 "github.com/jakub-dzon/k4e-device-worker/internal/os"
	"github.com/jakub-dzon/k4e-operator/models"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload"
	"github.com/jakub-dzon/k4e-device-worker/internal/heartbeat"
	"github.com/jakub-dzon/k4e-device-worker/internal/datatransfer"
	"time"
)

const (
	retryAfter = 10
)

type Registration struct {
	hardware         *hardware2.Hardware
	os               *os2.OS
	dispatcherClient pb.DispatcherClient
	config           *configuration.Manager
	RetryAfter       int64
	heartbeat        *heartbeat.Heartbeat
	workloads        *workload.WorkloadManager
	monitor          *datatransfer.Monitor
}

func NewRegistration(hardware *hardware2.Hardware, os *os2.OS, dispatcherClient pb.DispatcherClient, config *configuration.Manager, heartbeatManager *heartbeat.Heartbeat, workloadsManager *workload.WorkloadManager, monitorManager *datatransfer.Monitor) *Registration {
	return &Registration{
		hardware:         hardware,
		os:               os,
		dispatcherClient: dispatcherClient,
		config:           config,
		RetryAfter:       retryAfter,
		heartbeat:        heartbeatManager,
		workloads:        workloadsManager,
		monitor:          monitorManager,
	}
}

func (r *Registration) RegisterDevice() {
	err := r.registerDeviceOnce()
	if err != nil {
		log.Error(err)
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
		log.Infof("Configuration has not been initialized yet. Sending registration request.")
		err := r.registerDeviceOnce()
		if err != nil {
			log.Error(err)
		}
	}
}

func (r *Registration) registerDeviceOnce() error {
	hardwareInformation, err := r.hardware.GetHardwareInformation()
	if err != nil {
		return err
	}
	registrationInfo := models.RegistrationInfo{
		Hardware:  hardwareInformation,
		OsImageID: r.os.GetOsImageId(),
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
	return nil
}

func (r *Registration) Deregister() error {
	err := r.workloads.Deregister()
	if err != nil {
		log.Errorf("failed to deregister workloads: %v", err)
	}

	err = r.config.Deregister()
	if err != nil {
		log.Errorf("failed to deregister configuration: %v", err)
	}

	err = r.heartbeat.Deregister()
	if err != nil {
		log.Errorf("failed to deregister heartbeat: %v", err)
	}

	err = r.monitor.Deregister()
	if err != nil {
		log.Errorf("failed to deregister monitor: %v", err)
	}
	return nil
}
