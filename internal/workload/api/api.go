package api

type WorkloadInfo struct {
	Name   string
	Status string
}

type WorkloadAPI interface {
	List() ([]WorkloadInfo, error)
	Remove(workloadName string) error
	Run(manifestPath string) error
	Start(workloadName string) error
}
