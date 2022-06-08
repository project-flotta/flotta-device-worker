package datatransfer

import (
	"log"
	"net/http"

	"github.com/project-flotta/flotta-device-worker/internal/datatransfer/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	DataSyncTransferMetricsAddress = "localhost:9101"
	DataSyncTransferMetricsPath    = "/metrics"
	MetricsEndpoint                = "http://" + DataSyncTransferMetricsAddress + DataSyncTransferMetricsPath
)

var (
	registry = prometheus.NewRegistry()
	factory  = promauto.With(registry)

	FilesTransferredCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flotta_agent_datasync_files_transferred_counter",
			Help: "The total of files uploaded for a workload",
		}, []string{"workload_name"})
	BytesTransferredCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flotta_agent_datasync_bytes_transferred_counter",
			Help: "The total number of bytes uploaded for a workload",
		}, []string{"workload_name"})
	TimeTransferredCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flotta_agent_datasync_time_transferred_counter",
			Help: "The total amount of time spent transferring bytes for a workload",
		}, []string{"workload_name"})
	DeletedRemoteFilesTransferredCounter = factory.NewCounterVec(
		prometheus.CounterOpts{
			Name: "flotta_agent_datasync_deleted_files_transferred_counter",
			Help: "The total number of files deleted from the target destination",
		}, []string{"workload_name"})
)

func StartMetrics() {
	http.Handle(DataSyncTransferMetricsPath, promhttp.Handler())
	err := http.ListenAndServe(DataSyncTransferMetricsAddress, nil)
	if err != nil {
		log.Fatalf("unable to start data transfer metrics server %s", err)
	}
}

func reportMetrics(workloadName string, stats model.DataSyncStatistics) {
	FilesTransferredCounter.WithLabelValues(workloadName).Add(stats.FilesTransmitted)
	BytesTransferredCounter.WithLabelValues(workloadName).Add(stats.BytesTransmitted)
	TimeTransferredCounter.WithLabelValues(workloadName).Add(stats.TransmissionTime)
	DeletedRemoteFilesTransferredCounter.WithLabelValues(workloadName).Add(stats.DeletedRemoteFiles)
}
