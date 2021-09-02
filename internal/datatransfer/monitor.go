package datatransfer

import (
	"encoding/base64"
	"git.sr.ht/~spc/go-log"
	"github.com/jakub-dzon/k4e-device-worker/internal/configuration"
	"github.com/jakub-dzon/k4e-device-worker/internal/datatransfer/s3"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload"
	"github.com/jakub-dzon/k4e-operator/models"
	"path"
	"time"
)

type Monitor struct {
	workloads *workload.WorkloadManager
	config    *configuration.Manager
	ticker    *time.Ticker
}

func NewMonitor(workloadsManager *workload.WorkloadManager, configManager *configuration.Manager) *Monitor {
	ticker := time.NewTicker(configManager.GetDataTransferInterval())
	monitor := Monitor{
		workloads: workloadsManager,
		config:    configManager,
		ticker:    ticker,
	}
	return &monitor
}

func (m *Monitor) Start() {
	go func() {
		for range m.ticker.C {
			m.syncPaths()
		}
	}()
}

func (m *Monitor) syncPaths() {
	workloads, err := m.workloads.ListWorkloads()
	if err != nil {
		log.Errorf("Can't get the list of workloads: %v", err)
	}
	if len(workloads) == 0 {
		return
	}
	storage := m.config.GetDeviceConfiguration().Storage
	if storage != nil && storage.S3 != nil {
		workloadToDataPaths := make(map[string][]*models.DataPath)
		for _, wd := range m.config.GetWorkloads() {
			if wd.Data != nil && len(wd.Data.Paths) > 0 {
				workloadToDataPaths[wd.Name] = wd.Data.Paths
			}
		}

		s3Config := storage.S3
		accessKeyBytes, err := base64.StdEncoding.DecodeString(s3Config.AwsAccessKeyID)
		if err != nil {
			log.Errorf("Can't decode AWS Access Key: %v", err)
		}
		secretKeyBytes, err := base64.StdEncoding.DecodeString(s3Config.AwsSecretAccessKey)
		if err != nil {
			log.Errorf("Can't decode AWS Access Key: %v", err)
		}
		sync := s3.NewSync(s3Config.BucketHost, s3Config.BucketPort, string(accessKeyBytes), string(secretKeyBytes), s3Config.BucketName)

		// Monitor actual workloads and not ones expected by the configuration
		for _, wd := range workloads {
			hostPath := m.workloads.GetExportedHostPath(wd.Name)
			dataPaths := workloadToDataPaths[wd.Name]

			for _, dp := range dataPaths {
				source := path.Join(hostPath, dp.Source)
				target := dp.Target

				log.Infof("Synchronizing [device]%s => [remote]%s", source, target)
				if err := sync.SyncPath(source, target); err != nil {
					log.Errorf("Error while synchronizing [device]%s => [remote]%s: %v", source, target, err)
				}
			}
		}
	}
}
