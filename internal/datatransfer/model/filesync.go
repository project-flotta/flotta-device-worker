package model

//go:generate mockgen -package=datatransfer -destination=../mock_monitor.go . FileSync
type FileSync interface {
	Connect() error
	SyncPath(sourcePath, targetPath string) error
	Disconnect()
	GetStatistics() DataSyncStatistics
}

type DataSyncStatistics struct {
	FilesTransmitted   float64
	BytesTransmitted   float64
	DeletedRemoteFiles float64
}
