package api

import v1 "k8s.io/api/core/v1"

type WorkloadInfo struct {
	Name   string
	Status string
}

type WorkloadAPI interface {
	List() ([]WorkloadInfo, error)
	Remove(workloadName string) error
	Run(workload *v1.Pod, manifestPath string) error
	Start(workload *v1.Pod) error
}
