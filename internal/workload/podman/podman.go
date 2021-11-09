package podman

import (
	"context"

	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/play"
	"github.com/containers/podman/v3/pkg/bindings/pods"
	api2 "github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
)

type Podman struct {
	podmanConnection context.Context
}

func NewPodman() (*Podman, error) {
	podmanConnection, err := bindings.NewConnection(context.Background(), "unix://run/podman/podman.sock")
	if err != nil {
		return nil, err
	}
	return &Podman{
		podmanConnection: podmanConnection,
	}, nil
}

func (p *Podman) List() ([]api2.WorkloadInfo, error) {
	podList, err := pods.List(p.podmanConnection, nil)
	if err != nil {
		return nil, err
	}
	var workloads []api2.WorkloadInfo
	for _, pod := range podList {
		wi := api2.WorkloadInfo{
			Id:     pod.Id,
			Name:   pod.Name,
			Status: pod.Status,
		}
		workloads = append(workloads, wi)
	}
	return workloads, nil
}

func (p *Podman) Remove(workloadId string) error {
	exists, err := pods.Exists(p.podmanConnection, workloadId, nil)
	if err != nil {
		return err
	}
	if exists {
		force := true
		_, err := pods.Remove(p.podmanConnection, workloadId, &pods.RemoveOptions{Force: &force})
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Podman) Run(manifestPath string, configMaps []string) ([]string, error) {
	options := play.KubeOptions{
		ConfigMaps: &configMaps,
	}
	report, err := play.Kube(p.podmanConnection, manifestPath, &options)
	if err != nil {
		return nil, err
	}
	var podIds []string
	for _, pod := range report.Pods {
		podIds = append(podIds, pod.ID)
	}
	return podIds, nil
}

func (p *Podman) Start(workloadId string) error {
	_, err := pods.Start(p.podmanConnection, workloadId, nil)
	if err != nil {
		return err
	}
	return nil
}
