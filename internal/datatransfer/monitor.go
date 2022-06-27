package datatransfer

import (
	"fmt"
	"path"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/hashicorp/go-multierror"
	"github.com/project-flotta/flotta-device-worker/internal/configuration"
	"github.com/project-flotta/flotta-device-worker/internal/datatransfer/model"
	"github.com/project-flotta/flotta-device-worker/internal/datatransfer/s3"
	"github.com/project-flotta/flotta-device-worker/internal/workload"
	"github.com/project-flotta/flotta-device-worker/internal/workload/podman"
	"github.com/project-flotta/flotta-operator/models"
)

type Monitor struct {
	workloads                   *workload.WorkloadManager
	config                      *configuration.Manager
	ticker                      *time.Ticker
	lastSuccessfulSyncTimes     map[string]time.Time
	lastSuccessfulSyncTimesLock sync.RWMutex
	fsSync                      model.FileSync
	syncMutex                   sync.RWMutex
}

func NewMonitor(workloadsManager *workload.WorkloadManager, configManager *configuration.Manager) *Monitor {
	ticker := time.NewTicker(configManager.GetDataTransferInterval())

	monitor := Monitor{
		workloads:                   workloadsManager,
		config:                      configManager,
		ticker:                      ticker,
		lastSuccessfulSyncTimes:     make(map[string]time.Time),
		lastSuccessfulSyncTimesLock: sync.RWMutex{},
	}
	return &monitor
}

func (m *Monitor) String() string {
	return "data transfer"
}

func (m *Monitor) Start() {
	go func() {
		for range m.ticker.C {
			err := m.syncPaths()
			if err != nil {
				log.Error("Cannot sync paths: ", err)
			}
		}
		log.Infof("the monitor was stopped. DeviceID: %s;", m.workloads.GetDeviceID())
	}()
}

func (m *Monitor) Deregister() error {
	log.Infof("stopping monitor ticker. DeviceID: %s;", m.workloads.GetDeviceID())
	if m.ticker != nil {
		m.ticker.Stop()
	}
	return nil
}

func (m *Monitor) GetLastSuccessfulSyncTime(workloadName string) *time.Time {
	m.lastSuccessfulSyncTimesLock.RLock()
	defer m.lastSuccessfulSyncTimesLock.RUnlock()
	if t, ok := m.lastSuccessfulSyncTimes[workloadName]; ok {
		return &t
	}
	return nil
}

// WorkloadStarted is defined to satisfied the workload.Observer Interface, do
// nothing.
func (m *Monitor) WorkloadStarted(workloadName string, report []*podman.PodReport) {
}

func (m *Monitor) WorkloadRemoved(workloadName string) {
	m.syncPathsWorkload(workloadName)

	m.lastSuccessfulSyncTimesLock.Lock()
	defer m.lastSuccessfulSyncTimesLock.Unlock()
	delete(m.lastSuccessfulSyncTimes, workloadName)
}

func (m *Monitor) syncPathsWorkload(workloadName string) {
	if !m.HasStorageDefined() {
		return
	}

	dataConfig := m.getDataConfigOfWorkload(workloadName)
	if !containsDataPaths(dataConfig) {
		return
	}

	hostPath := m.workloads.GetExportedHostPath(workloadName)
	var errors error
	err := m.syncDataPaths(dataConfig.Egress, hostPath, workloadName, egress)
	if err != nil {
		errors = multierror.Append(errors, err)
	}
	err = m.syncDataPaths(dataConfig.Ingress, hostPath, workloadName, ingress)
	if err != nil {
		errors = multierror.Append(errors, err)
	}
	if errors == nil {
		m.storeLastUpdateTime(workloadName)
	}

}

func (m *Monitor) getDataConfigOfWorkload(workloadName string) *models.DataConfiguration {
	for _, wd := range m.config.GetWorkloads() {
		if wd.Name == workloadName {
			return wd.Data
		}
	}
	return nil
}

func (m *Monitor) ForceSync() error {
	return m.syncPaths()
}

func (m *Monitor) HasStorageDefined() bool {
	storage := m.config.GetDeviceConfiguration().Storage
	if storage == nil {
		return false
	}

	return storage.S3 != nil
}

func (m *Monitor) SetStorage(fs model.FileSync) {
	m.syncMutex.Lock()
	defer m.syncMutex.Unlock()
	m.fsSync = fs
}

func (m *Monitor) getFsSync() (model.FileSync, error) {
	m.syncMutex.Lock()
	defer m.syncMutex.Unlock()
	if m.fsSync == nil {
		return nil, fmt.Errorf("cannot get filesync")
	}
	// Copy here to be able to always use that pointer meanwhile update the
	// config. Related to:
	// https://github.com/project-flotta/flotta-device-worker/pull/38#discussion_r735290220
	res := m.fsSync
	return res, nil
}

func (m *Monitor) syncPaths() error {

	workloads, err := m.workloads.ListWorkloads()
	if err != nil {
		log.Errorf("cannot get the list of workloads. DeviceID: %s; err: %v", m.workloads.GetDeviceID(), err)
		return err
	}

	if len(workloads) == 0 {
		log.Tracef("no workloads to return. DeviceID: %s;", m.workloads.GetDeviceID())
		return nil
	}

	if !m.HasStorageDefined() {
		log.Tracef("monitor does not have storage defined. DeviceID: %s;", m.workloads.GetDeviceID())
		return nil
	}

	workloadToDataPaths := make(map[string]*models.DataConfiguration)
	for _, wd := range m.config.GetWorkloads() {
		if containsDataPaths(wd.Data) {
			workloadToDataPaths[wd.Name] = wd.Data
		}
	}

	var errors error
	// Monitor actual workloads and not ones expected by the configuration

	for _, wd := range workloads {
		dataConfig := workloadToDataPaths[wd.Name]
		if dataConfig == nil {
			log.Infof("workload %s not found in configuration", wd.Name)
			continue
		}
		hostPath := m.workloads.GetExportedHostPath(wd.Name)
		err := m.syncDataPaths(dataConfig.Egress, hostPath, wd.Name, egress)
		if err != nil {
			errors = multierror.Append(errors, err)
			continue
		}
		err = m.syncDataPaths(dataConfig.Ingress, hostPath, wd.Name, ingress)
		if err != nil {
			errors = multierror.Append(errors, err)
			continue
		}
		m.storeLastUpdateTime(wd.Name)
	}
	return errors
}

func (m *Monitor) storeLastUpdateTime(workloadName string) {
	m.lastSuccessfulSyncTimesLock.Lock()
	defer m.lastSuccessfulSyncTimesLock.Unlock()
	m.lastSuccessfulSyncTimes[workloadName] = time.Now()
}

func (m *Monitor) Init(configuration models.DeviceConfigurationMessage) error {
	return m.Update(configuration)
}

func (m *Monitor) Update(configuration models.DeviceConfigurationMessage) error {
	if configuration.Configuration == nil || configuration.Configuration.Storage == nil || configuration.Configuration.Storage.S3 == nil {
		return nil
	}

	s3sync, err := s3.NewSync(*configuration.Configuration.Storage.S3)
	if err != nil {
		return fmt.Errorf("observer update failed. DeviceID: %s; err: %s", m.workloads.GetDeviceID(), err)
	}
	m.SetStorage(s3sync)
	return nil
}

func containsDataPaths(dc *models.DataConfiguration) bool {
	return dc != nil &&
		(dc.Egress != nil && len(dc.Egress) > 0 ||
			dc.Ingress != nil && len(dc.Ingress) > 0)
}

func (m *Monitor) syncDataPaths(dataPaths []*models.DataPath, hostPath, workloadName string, dir direction) error {
	syncWrapper, err := m.getFsSync()
	if err != nil {
		return err
	}
	err = syncWrapper.Connect()
	if err != nil {
		return err
	}
	defer syncWrapper.Disconnect()
	var errors error
	// For Ingress
	// TODO: Identify how much disk space is required for complete ingress sync before pulling remote data onto the device storage.
	// a simple check of a diff in disk usage between remote and local will suffice.
	if dataPaths != nil {
		startSync := time.Now().UnixMilli()
		for _, dp := range dataPaths {
			source, target := resolvePaths(dp.Source, dp.Target, hostPath, dir)
			err := m.syncPath(source, target)
			if err != nil {
				errors = multierror.Append(errors, err)
			}
		}
		syncTime := time.Now().UnixMilli() - startSync
		stats := syncWrapper.GetStatistics()
		reportMetrics(workloadName, dir, stats.BytesTransmitted, stats.FilesTransmitted, stats.DeletedRemoteFiles, syncTime)
	}
	return errors
}

func resolvePaths(source, target, hostPath string, dir direction) (string, string) {
	if dir == egress {
		return path.Join(hostPath, source), target
	}
	return source, path.Join(hostPath, target)
}

func (m *Monitor) syncPath(source, target string) error {
	logMessage := fmt.Sprintf("synchronizing [source]%s => [target]%s", source, target)
	log.Debug(logMessage)
	err := m.fsSync.SyncPath(source, target)
	if err != nil {
		log.Errorf("error while %s", logMessage)
		return fmt.Errorf("error while %s", logMessage)
	}
	return nil
}
