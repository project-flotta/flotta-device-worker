package registration

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	hardware2 "github.com/jakub-dzon/k4e-device-worker/internal/hardware"
	os2 "github.com/jakub-dzon/k4e-device-worker/internal/os"
	"github.com/jakub-dzon/k4e-operator/models"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"time"
)

type Registration struct {
	hardware         *hardware2.Hardware
	os               *os2.OS
	dispatcherClient pb.DispatcherClient
}

func NewRegistration(hardware *hardware2.Hardware, os *os2.OS, dispatcherClient pb.DispatcherClient) *Registration {
	return &Registration{
		hardware:         hardware,
		os:               os,
		dispatcherClient: dispatcherClient,
	}
}

func (r *Registration) RegisterDevice() error {
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
