package heartbeat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"

	ansible "github.com/project-flotta/flotta-device-worker/internal/ansible"
	cfg "github.com/project-flotta/flotta-device-worker/internal/configuration"
	"github.com/project-flotta/flotta-device-worker/internal/datatransfer"
	hw "github.com/project-flotta/flotta-device-worker/internal/hardware"
	os2 "github.com/project-flotta/flotta-device-worker/internal/os"
	"github.com/project-flotta/flotta-device-worker/internal/registration"
	workld "github.com/project-flotta/flotta-device-worker/internal/workload"
	"github.com/project-flotta/flotta-operator/models"

	pb "github.com/redhatinsights/yggdrasil/protocol"
)

const (
	ScopeDelta = "delta"
	ScopeFull  = "full"
)

type HeartbeatData struct {
	configManager                   *cfg.Manager
	workloadManager                 *workld.WorkloadManager
	ansibleManager                  *ansible.Manager
	dataMonitor                     *datatransfer.Monitor
	hardware                        hw.Hardware
	osInfo                          *os2.OS
	previousMutableHardwareInfo     *models.HardwareInfo
	previousMutableHardwareInfoLock sync.RWMutex
}

func NewHeartbeatData(configManager *cfg.Manager,
	workloadManager *workld.WorkloadManager, ansibleManager *ansible.Manager, hardware hw.Hardware, dataMonitor *datatransfer.Monitor, deviceOs *os2.OS) *HeartbeatData {

	return &HeartbeatData{
		configManager:               configManager,
		workloadManager:             workloadManager,
		ansibleManager:              ansibleManager,
		hardware:                    hardware,
		dataMonitor:                 dataMonitor,
		osInfo:                      deviceOs,
		previousMutableHardwareInfo: nil,
	}
}

func (s *HeartbeatData) RetrieveInfo() models.Heartbeat {

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
		log.Errorf("cannot get workload information. DeviceID: %s; err: %v", s.workloadManager.GetDeviceID(), err)
	}

	config := s.configManager.GetDeviceConfiguration()
	var hardwareInfo *models.HardwareInfo
	if config.Heartbeat.HardwareProfile.Include {
		hardwareInfo = s.buildHardwareInfo()
	}
	ansibleEvents := []*models.EventInfo{}
	if s.ansibleManager != nil {
		ansibleEvents = s.ansibleManager.PopEvents()
	}
	heartbeatInfo := models.Heartbeat{
		Status:    models.HeartbeatStatusUp,
		Version:   s.configManager.GetConfigurationVersion(),
		Workloads: workloadStatuses,
		Hardware:  hardwareInfo,
		Events:    append(s.workloadManager.PopEvents(), ansibleEvents...),
		Upgrade:   s.osInfo.GetUpgradeStatus(),
	}
	return heartbeatInfo
}

func (s *HeartbeatData) GetPreviousHardwareInfo() *models.HardwareInfo {
	s.previousMutableHardwareInfoLock.RLock()
	defer s.previousMutableHardwareInfoLock.RUnlock()
	return s.previousMutableHardwareInfo
}

func (s *HeartbeatData) SetPreviousHardwareInfo(previousHardwareInfo *models.HardwareInfo) {
	s.previousMutableHardwareInfoLock.Lock()
	defer s.previousMutableHardwareInfoLock.Unlock()
	s.previousMutableHardwareInfo = previousHardwareInfo
}

func (s *HeartbeatData) buildHardwareInfo() *models.HardwareInfo {
	currentMutableHwInfo, err := s.hardware.CreateHardwareMutableInformation()
	if err != nil {
		log.Errorf("cannot create hardware mutable information. DeviceID: %s; err: %v", s.workloadManager.GetDeviceID(), err)
		return nil
	}
	hardwareInfo := s.getMutableHardwareInfoDelta(*currentMutableHwInfo)

	if s.GetPreviousHardwareInfo() == nil {
		var err error
		// Only send all Hw info (mutable + immutable) for the 1st heartbeat, then send only mutable hw info
		hardwareInfo, err = s.hardware.GetHardwareInformation()
		if err != nil {
			log.Errorf("cannot get full hardware information. DeviceID: %s; err: %v", s.workloadManager.GetDeviceID(), err)
		}
	}
	s.SetPreviousHardwareInfo(currentMutableHwInfo)

	return hardwareInfo
}

func (s *HeartbeatData) getMutableHardwareInfoDelta(currentMutableHwInfo models.HardwareInfo) *models.HardwareInfo {
	hardwareInfo := &currentMutableHwInfo
	if s.configManager.GetDeviceConfiguration().Heartbeat.HardwareProfile.Scope == ScopeDelta {
		log.Debugf("Checking if mutable hardware information change between heartbeat (scope = delta). DeviceID: %s", s.workloadManager.GetDeviceID())
		previousMutableHardwareInfo := s.GetPreviousHardwareInfo()
		if previousMutableHardwareInfo != nil {
			hardwareInfo = s.hardware.GetMutableHardwareInfoDelta(*previousMutableHardwareInfo, *hardwareInfo)
		}
	}

	return hardwareInfo
}

type Heartbeat struct {
	ticker                *time.Ticker
	dispatcherClient      pb.DispatcherClient
	data                  *HeartbeatData
	reg                   *registration.Registration
	firstHearbeat         bool
	previousPeriodSeconds int64
	sendLock              sync.Mutex
	tickerLock            sync.RWMutex
	pushInfoLock          sync.RWMutex
	log                   log.Logger
}

func NewHeartbeatService(dispatcherClient pb.DispatcherClient, configManager *cfg.Manager,
	workloadManager *workld.WorkloadManager, hardware hw.Hardware,
	dataMonitor *datatransfer.Monitor, osInfo *os2.OS,
	reg registration.RegistrationWrapper) *Heartbeat {
	return &Heartbeat{
		ticker:           nil,
		dispatcherClient: dispatcherClient,
		data: &HeartbeatData{
			configManager:   configManager,
			workloadManager: workloadManager,
			hardware:        hardware,
			dataMonitor:     dataMonitor,
			osInfo:          osInfo,
		},
		firstHearbeat:         true,
		previousPeriodSeconds: -1,
		log:                   *log.New(os.Stderr, log.Prefix(), log.Flags(), log.CurrentLevel()),
	}
}

func (s *Heartbeat) SetLogger(logger log.Logger) {
	s.log = logger
}

func (s *Heartbeat) send(data *pb.Data) error {
	s.sendLock.Lock()
	defer s.sendLock.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	log.Debugf("Heartbeat send: Sending data: %+v; Device ID: %s", data, s.data.workloadManager.GetDeviceID())
	response, err := s.dispatcherClient.Send(ctx, data)
	log.Debugf("Heartbeat send: Response: %+v, err: %+v; Device ID: %s", response, err, s.data.workloadManager.GetDeviceID())
	if err != nil {
		return err
	}

	if isResponseEmpty(response) {
		return fmt.Errorf("empty response received, host may not be reachable; reponse: %+v, response.Response: %s, Device ID: %s", response, response.Response, s.data.workloadManager.GetDeviceID())
	}

	parsedResponse, err := registration.NewYGGDResponse(response.Response)
	if err != nil {
		return err
	}

	// If it's already expired the cert need to be renewed
	if parsedResponse.StatusCode != http.StatusUnauthorized {
		return err
	}
	s.reg.RegisterDevice()
	// Sending again the heartbeat info with the right info.
	_, err = s.dispatcherClient.Send(ctx, data)

	return err
}

func isResponseEmpty(response *pb.Response) bool {
	return response == nil || len(response.Response) == 0
}

func (s *Heartbeat) String() string {
	return "heartbeat"
}

func (s *Heartbeat) Start() {
	s.previousPeriodSeconds = s.getInterval(s.data.configManager.GetDeviceConfiguration())
	s.initTicker(s.getInterval(s.data.configManager.GetDeviceConfiguration()))
}

func (s *Heartbeat) HasStarted() bool {
	s.tickerLock.RLock()
	defer s.tickerLock.RUnlock()
	return s.ticker != nil
}

// Init no-op due to we need to an update from the source of truth in this
// case(API)
func (s *Heartbeat) Init(config models.DeviceConfigurationMessage) error {
	return nil
}

func (s *Heartbeat) Update(config models.DeviceConfigurationMessage) error {
	periodSeconds := s.getInterval(*config.Configuration)
	previousPeriodSeconds := atomic.LoadInt64(&s.previousPeriodSeconds)
	if previousPeriodSeconds <= 0 || previousPeriodSeconds != periodSeconds {
		s.log.Debugf("Heartbeat configuration update: periodSeconds changed from %d to %d; Device ID: %s", previousPeriodSeconds, periodSeconds, s.data.workloadManager.GetDeviceID())
		s.log.Infof("reconfiguring ticker with interval: %v. DeviceID: %s", periodSeconds, s.data.workloadManager.GetDeviceID())
		s.stopTicker()

		atomic.StoreInt64(&s.previousPeriodSeconds, periodSeconds)

		s.initTicker(periodSeconds)
		return nil
	}
	return nil
}

func (s *Heartbeat) getInterval(config models.DeviceConfiguration) int64 {
	var interval int64 = 60

	if config.Heartbeat != nil {
		interval = config.Heartbeat.PeriodSeconds
	}
	if interval <= 0 {
		interval = 60
	}
	return interval
}

func (s *Heartbeat) pushInformation() error {
	// Create a data message to send back to the dispatcher.
	heartbeatInfo := s.data.RetrieveInfo()
	deviceId := s.data.workloadManager.GetDeviceID()
	log.Debugf("pushInformation: Heartbeat info: %+v; DeviceID: %s;", heartbeatInfo, deviceId)
	content, err := json.Marshal(heartbeatInfo)
	if err != nil {
		return err
	}

	data := &pb.Data{
		MessageId: uuid.New().String(),
		Content:   content,
		Directive: "heartbeat",
	}
	log.Debugf("pushInformation: sending content %+v; DeviceID: %s;", content, deviceId)
	err = s.send(data)
	log.Debugf("pushInformation: sending content results %s; DeviceID: %s;", err, deviceId)

	if err != nil {
		s.pushInfoLock.RLock()
		defer s.pushInfoLock.RUnlock()
		if s.firstHearbeat {
			s.data.SetPreviousHardwareInfo(nil)
		}
		return err
	}
	s.pushInfoLock.Lock()
	defer s.pushInfoLock.Unlock()
	s.firstHearbeat = false

	return nil
}

func (s *Heartbeat) initTicker(periodSeconds int64) {
	ticker := time.NewTicker(time.Second * time.Duration(periodSeconds))
	s.tickerLock.Lock()
	defer s.tickerLock.Unlock()
	s.ticker = ticker
	go func() {
		for range ticker.C {
			err := s.pushInformation()
			if err != nil {
				s.log.Errorf("heartbeat interval cannot send the data. DeviceID: %s; err: %s", s.data.workloadManager.GetDeviceID(), err)
			}
		}
	}()

	s.log.Infof("the heartbeat was started. DeviceID: %s", s.data.workloadManager.GetDeviceID())
}

func (s *Heartbeat) Deregister() error {
	s.log.Infof("stopping heartbeat ticker. DeviceID: %s", s.data.workloadManager.GetDeviceID())
	s.stopTicker()
	return nil
}

func (s *Heartbeat) stopTicker() {
	if s.HasStarted() {
		s.tickerLock.RLock()
		defer s.tickerLock.RUnlock()
		s.ticker.Stop()
	}
}
