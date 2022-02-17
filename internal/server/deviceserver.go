package server

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/apenella/go-ansible/pkg/options"
	"github.com/apenella/go-ansible/pkg/playbook"
	"github.com/project-flotta/flotta-device-worker/internal/ansible"
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
	ansibleManager       *ansible.AnsibleManager
}

func NewDeviceServer(configManager *configuration2.Manager, registrationManager *registration2.Registration, ansibleManager *ansible.AnsibleManager) *deviceServer {
	server := &deviceServer{
		deviceID:             configManager.GetDeviceID(),
		registrationManager:  registrationManager,
		configurationUpdates: make(chan models.DeviceConfigurationMessage),
		ansibleManager:      ansibleManager,
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
		//check if it is an ansible playbook message

		if x, found := d.GetMetadata()["ansible-playbook"]; found && x == "true" {
			log.Debugf("Received message %s with 'ansible-playbook' metadata", d.MessageId)

			// defined how to connect to hosts
			ansiblePlaybookConnectionOptions := &options.AnsibleConnectionOptions{
				Connection: "local",
			}
			// defined which should be the ansible-playbook execution behavior and where to find execution configuration.
			ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{
				Inventory: "127.0.0.1,",
			}

			playbookCmd := &playbook.AnsiblePlaybookCmd{
				ConnectionOptions: ansiblePlaybookConnectionOptions,
				Options:           ansiblePlaybookOptions,
			}

			timeout := getTimeout(d.GetMetadata())
			err := s.ansibleManager.HandlePlaybook(playbookCmd, d, timeout)

			if err != nil {
				log.Warnf("cannot handle ansible playbook. Error: %v", err)
			}
		}

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

func getTimeout(metadata map[string]string) time.Duration {
	timeout := 300 * time.Second // Deafult timeout
	if timeoutStr, found := metadata["ansible-playbook-timeout"]; found {
		timeoutVal, err := strconv.Atoi(timeoutStr)
		if err != nil {
			return timeout
		}
		timeout = time.Duration(timeoutVal) * time.Second
	}
	return timeout
}
