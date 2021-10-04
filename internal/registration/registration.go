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
}

func NewRegistration(hardware *hardware2.Hardware, os *os2.OS, dispatcherClient pb.DispatcherClient, config *configuration.Manager) *Registration {
	return &Registration{
		hardware:         hardware,
		os:               os,
		dispatcherClient: dispatcherClient,
		config:           config,
		RetryAfter:       retryAfter,
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
