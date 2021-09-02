package datatransfer

import (
	"git.sr.ht/~spc/go-log"
	"github.com/jakub-dzon/k4e-device-worker/internal/configuration"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload"
	"github.com/jakub-dzon/k4e-operator/models"
	"path"
)

type Monitor struct {
	workloads *workload.WorkloadManager
	config    *configuration.Manager
}

func (m *Monitor) syncPaths() error {
	workloads, err := m.workloads.ListWorkloads()
	if err != nil {
		return err
	}
	workloadToDataPaths := make(map[string][]*models.DataPath)
	for _, wd := range m.config.GetWorkloads() {
		workloadToDataPaths[wd.Name] = wd.Data.Paths
	}

	// Monitor actual workloads and not ones expected by the configuration
	for _, wd := range workloads {
		hostPath := m.workloads.GetExportedHostPath(wd.Name)
		dataPaths := workloadToDataPaths[wd.Name]
		for _, dp := range dataPaths {
			source := path.Join(hostPath, dp.Source)
			target := dp.Source
			// TODO: Actual sync
			log.Infof("Synchronizing [device]%s => [remote]%s", source, target)
		}
	}
	return nil
}
