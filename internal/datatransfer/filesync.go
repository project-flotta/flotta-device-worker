package datatransfer

//go:generate mockgen -package=datatransfer -destination=mock_monitor.go . FileSync
type FileSync interface {
	Connect() error
	SyncPath(sourcePath, targetPath string) error
}
