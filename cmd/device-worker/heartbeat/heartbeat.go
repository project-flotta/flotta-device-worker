package heartbeat

import (
	"context"
	"encoding/json"
	"git.sr.ht/~spc/go-log"
	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/jakub-dzon/k4e-device-worker/cmd/device-worker/configuration"
	"github.com/jakub-dzon/k4e-operator/models"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"time"
)

type Service struct {
	ticker           *time.Ticker
	dispatcherClient pb.DispatcherClient
	configManager    *configuration.Manager
}

func NewHeartbeatService(dispatcherClient pb.DispatcherClient, configManager *configuration.Manager) *Service {
	return &Service{
		dispatcherClient: dispatcherClient,
		configManager:    configManager,
	}
}

func (s *Service) Start() {
	config := s.configManager.GetDeviceConfiguration()
	s.initTicker(config.Heartbeat.PeriodSeconds)
}

func (s *Service) initTicker(periodSeconds int64) {
	ticker := time.NewTicker(time.Second * time.Duration(periodSeconds))
	s.ticker = ticker
	go func() {
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			// Create a data message to send back to the dispatcher.
			heartbeatInfo := models.Heartbeat{
				Status: models.HeartbeatStatusUp,
				Time:   strfmt.DateTime(time.Now()),
				Version: s.configManager.GetConfigurationVersion(),
			}

			content, err := json.Marshal(heartbeatInfo)
			if err != nil {
				log.Errorf("Cannot marshal heartbeat info: %v", err)
			}
			data := &pb.Data{
				MessageId: uuid.New().String(),
				Content:   content,
				Directive: "heartbeat",
			}

			// Call "Send"
			if _, err := s.dispatcherClient.Send(ctx, data); err != nil {
				log.Error(err)
			}
		}
	}()
}

func (s *Service) Update(config models.DeviceConfigurationMessage) error {
	periodSeconds := config.Configuration.Heartbeat.PeriodSeconds
	log.Infof("Reconfiguring ticker with interval: %v", periodSeconds)
	s.ticker.Stop()
	s.initTicker(periodSeconds)
	return nil
}
