package registration

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jakub-dzon/k4e-device-worker/cmd/device-worker/hardware"
	"github.com/jakub-dzon/k4e-device-worker/cmd/device-worker/os"
	"github.com/jakub-dzon/k4e-operator/models"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"time"
)

type Registration struct {
	hardware         *hardware.Hardware
	os               *os.OS
	dispatcherClient pb.DispatcherClient
}

func NewRegistration(hardware *hardware.Hardware, os *os.OS, dispatcherClient pb.DispatcherClient) *Registration {
	return &Registration{
		hardware:         hardware,
		os:               os,
		dispatcherClient: dispatcherClient,
	}
}

func (r *Registration) RegisterDevice() error {
	hardwareInformation := r.hardware.GetHardwareInformation()
	registrationInfo := models.RegistrationInfo{
		Hardware:  &hardwareInformation,
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
