package datatransfer

import (
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	DataSyncTransferMetricsAddress           = "localhost:9101"
	DataSyncTransferMetricsPath              = "/metrics"
	MetricsEndpoint                          = "http://" + DataSyncTransferMetricsAddress + DataSyncTransferMetricsPath
	egress                         direction = "egress"
	ingress                        direction = "ingress"
)

type direction string

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

func StartMetrics() {
	http.Handle(DataSyncTransferMetricsPath, promhttp.Handler())
	err := http.ListenAndServe(DataSyncTransferMetricsAddress, nil)
	if err != nil {
		log.Fatalf("unable to start data transfer metrics server %s", err)
	}
}

func reportMetrics(workloadName string, dir direction, bytesTransmitted, filesTransmitted, deletedFiles float64, syncTime int64) {
	FilesTransferredCounter.WithLabelValues(workloadName, string(dir)).Add(filesTransmitted)
	BytesTransferredCounter.WithLabelValues(workloadName, string(dir)).Add(bytesTransmitted)
	DeletedRemoteFilesTransferredCounter.WithLabelValues(workloadName, string(dir)).Add(deletedFiles)
	TimeTransferredCounter.WithLabelValues(workloadName, string(dir)).Add(float64(syncTime))
}
