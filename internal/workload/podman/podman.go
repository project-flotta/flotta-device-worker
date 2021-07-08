package podman

import (
	"context"
	"github.com/containers/podman/v2/pkg/bindings"
	"github.com/containers/podman/v2/pkg/bindings/play"
	"github.com/containers/podman/v2/pkg/bindings/pods"
	"github.com/containers/podman/v2/pkg/domain/entities"
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
	podList, err := pods.List(p.podmanConnection, map[string][]string{})
	if err != nil {
		return nil, err
	}
	var workloads []api2.WorkloadInfo
	for _, pod := range podList {
		wi := api2.WorkloadInfo{
			Name:   pod.Name,
			Status: pod.Status,
		}
		workloads = append(workloads, wi)
	}
	return workloads, nil
}

func (p *Podman) Remove(workloadName string) error {
	exists, err := pods.Exists(p.podmanConnection, workloadName)
	if err != nil {
		return err
	}
	if exists {
		force := true
		_, err := pods.Remove(p.podmanConnection, workloadName, &force)
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Podman) Run(manifestPath string) error {
	_, err := play.Kube(p.podmanConnection, manifestPath, entities.PlayKubeOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (p *Podman) Start(workloadName string) error {
	_, err := pods.Start(p.podmanConnection, workloadName)
	if err != nil {
		return err
	}
	return nil
}
