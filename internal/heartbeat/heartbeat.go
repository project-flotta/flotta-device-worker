package heartbeat

import (
	"context"
	"encoding/json"
	"git.sr.ht/~spc/go-log"
	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	cfg "github.com/jakub-dzon/k4e-device-worker/internal/configuration"
	"github.com/jakub-dzon/k4e-device-worker/internal/datatransfer"
	hw "github.com/jakub-dzon/k4e-device-worker/internal/hardware"
	workld "github.com/jakub-dzon/k4e-device-worker/internal/workload"
	"github.com/jakub-dzon/k4e-operator/models"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"time"
)

type Heartbeat struct {
	ticker           *time.Ticker
	dispatcherClient pb.DispatcherClient
	configManager    *cfg.Manager
	workloadManager  *workld.WorkloadManager
	dataMonitor      *datatransfer.Monitor
	hardware         *hw.Hardware
}

func NewHeartbeatService(dispatcherClient pb.DispatcherClient, configManager *cfg.Manager,
	workloadManager *workld.WorkloadManager, hardware *hw.Hardware, dataMonitor *datatransfer.Monitor) *Heartbeat {
	return &Heartbeat{
		dispatcherClient: dispatcherClient,
		configManager:    configManager,
		workloadManager:  workloadManager,
		hardware:         hardware,
		dataMonitor:      dataMonitor,
	}
}

func (s *Heartbeat) Start() {
	s.initTicker(s.getInterval(s.configManager.GetDeviceConfiguration()))
}

func (s *Heartbeat) getInterval(config models.DeviceConfiguration) int64 {
	interval := config.Heartbeat.PeriodSeconds
	if interval <= 0 {
		interval = 60
	}
	return interval
}

func (s *Heartbeat) initTicker(periodSeconds int64) {
	ticker := time.NewTicker(time.Second * time.Duration(periodSeconds))
	s.ticker = ticker
	go func() {
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			// Create a data message to send back to the dispatcher.
			heartbeatInfo := s.getHeartbeatInfo()

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

func (s *Heartbeat) getHeartbeatInfo() models.Heartbeat {
	var workloadStatuses []*models.WorkloadStatus
	workloads, err := s.workloadManager.ListWorkloads()
	for _, info := range workloads {
		workloadStatus := models.WorkloadStatus{
			Name:   info.Name,
			Status: info.Status,
		}
		if lastSyncTime := s.dataMonitor.GetLastSuccessfulSyncTime(info.Name); lastSyncTime != nil {
			workloadStatus.LastDataUpload = strfmt.DateTime(*lastSyncTime)
		}
		workloadStatuses = append(workloadStatuses, &workloadStatus)
	}
	if err != nil {
		log.Errorf("Cannot get workload information: %v", err)
	}

	config := s.configManager.GetDeviceConfiguration()
	var hardwareInfo *models.HardwareInfo
	if config.Heartbeat.HardwareProfile.Include {
		hardwareInfo, err = s.hardware.GetHardwareInformation()
		if err != nil {
			log.Errorf("Can't get hardware information: %v", err)
		}
	}

	heartbeatInfo := models.Heartbeat{
		Status:    models.HeartbeatStatusUp,
		Time:      strfmt.DateTime(time.Now()),
		Version:   s.configManager.GetConfigurationVersion(),
		Workloads: workloadStatuses,
		Hardware:  hardwareInfo,
	}
	return heartbeatInfo
}

func (s *Heartbeat) Update(config models.DeviceConfigurationMessage) error {
	periodSeconds := s.getInterval(*config.Configuration)
	log.Infof("Reconfiguring ticker with interval: %v", periodSeconds)
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.initTicker(periodSeconds)
	return nil
}
