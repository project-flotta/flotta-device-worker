package datatransfer

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type direction string

const (
	egress  direction = "egress"
	ingress direction = "ingress"
)

var (
	registry = prometheus.NewRegistry()
	factory  = promauto.With(registry)

	FilesTransferredCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flotta_agent_datasync_files_transferred_counter",
			Help: "The total of files uploaded for a workload",
		}, []string{"workload_name", "direction"})
	BytesTransferredCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flotta_agent_datasync_bytes_transferred_counter",
			Help: "The total number of bytes uploaded for a workload",
		}, []string{"workload_name", "direction"})
	TimeTransferredCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flotta_agent_datasync_time_transferred_counter",
			Help: "The total amount of time spent transferring bytes for a workload",
		}, []string{"workload_name", "direction"})
	DeletedRemoteFilesTransferredCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flotta_agent_datasync_deleted_files_transferred_counter",
			Help: "The total number of files deleted from the target destination",
		}, []string{"workload_name", "direction"})
)

func GetRegistry() prometheus.Gatherer {

	return registry
}

func reportMetrics(workloadName string, dir direction, bytesTransmitted, filesTransmitted, deletedFiles float64, syncTime int64) {
	FilesTransferredCounter.WithLabelValues(workloadName, string(dir)).Add(filesTransmitted)
	BytesTransferredCounter.WithLabelValues(workloadName, string(dir)).Add(bytesTransmitted)
	DeletedRemoteFilesTransferredCounter.WithLabelValues(workloadName, string(dir)).Add(deletedFiles)
	TimeTransferredCounter.WithLabelValues(workloadName, string(dir)).Add(float64(syncTime))
}
