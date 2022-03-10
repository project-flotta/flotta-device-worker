package server

import (
	"context"
	"encoding/json"

	"git.sr.ht/~spc/go-log"
	configuration2 "github.com/project-flotta/flotta-device-worker/internal/configuration"
	registration2 "github.com/project-flotta/flotta-device-worker/internal/registration"
	"github.com/project-flotta/flotta-operator/models"
	pb "github.com/redhatinsights/yggdrasil/protocol"
)

type deviceServer struct {
	pb.UnimplementedWorkerServer
	deviceID             string
	registrationManager  *registration2.Registration
	configurationUpdates chan models.DeviceConfigurationMessage
}

func NewDeviceServer(configManager *configuration2.Manager, registrationManager *registration2.Registration) *deviceServer {
	server := &deviceServer{
		deviceID:             configManager.GetDeviceID(),
		registrationManager:  registrationManager,
		configurationUpdates: make(chan models.DeviceConfigurationMessage),
	}

	go func() {
		for deviceConfigurationMessage := range server.configurationUpdates {
			err := configManager.Update(deviceConfigurationMessage)
			if err != nil {
				log.Warnf("failed to process message. DeviceID: %s; err: %v", server.deviceID, err)
			}
		}
	}()
	return server
}

// Send implements the "Send" method of the Worker gRPC service.
func (s *deviceServer) Send(_ context.Context, d *pb.Data) (*pb.Response, error) {
	go func() {
		deviceConfigurationMessage := models.DeviceConfigurationMessage{}
		err := json.Unmarshal(d.Content, &deviceConfigurationMessage)
		if err != nil {
			log.Warnf("cannot unmarshal message. DeviceID: %s; err: %v", s.deviceID, err)
		}
		s.configurationUpdates <- deviceConfigurationMessage
	}()

	// Respond to the start request that the work was accepted.
	return &pb.Response{}, nil
}

// NotifyEvent implements the "NotifyEvent" method of the Worker gRPC service.
func (s *deviceServer) NotifyEvent(ctx context.Context, in *pb.EventNotification) (*pb.EventReceipt, error) {
	log.Infof("received worker event. DeviceID: %s; Event: %s", s.deviceID, in.Name)

	if in.Name == pb.Event_RECEIVED_DISCONNECT {
		log.Infof("Starting unregisting DeviceID: %s", s.deviceID)
		err := s.registrationManager.Deregister()
		if err != nil {
			log.Warnf("cannot disconnect. DeviceID: %s; err: %v", s.deviceID, err)
		}
	}
	return &pb.EventReceipt{}, nil
}
