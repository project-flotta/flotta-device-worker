package podman

import (
	"context"
	"fmt"
	"strings"

	"git.sr.ht/~spc/go-log"
	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/containers"
	"github.com/containers/podman/v3/pkg/bindings/generate"
	"github.com/containers/podman/v3/pkg/bindings/play"
	"github.com/containers/podman/v3/pkg/bindings/pods"
	"github.com/containers/podman/v3/pkg/bindings/secrets"
	api2 "github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
)

//go:generate mockgen -package=podman -destination=mock_podman.go . Podman
type Podman interface {
	List() ([]api2.WorkloadInfo, error)
	Remove(workloadId string) error
	Run(manifestPath, authFilePath string) ([]*PodReport, error)
	Start(workloadId string) error
	ListSecrets() (map[string]struct{}, error)
	RemoveSecret(name string) error
	CreateSecret(name, data string) error
	UpdateSecret(name, data string) error
	Exists(workloadId string) (bool, error)
}

type podman struct {
	podmanConnection context.Context
}

type ContainerReport struct {
	IPAddress string
	Id        string
	Name      string
}

type PodReport struct {
	Id         string
	Name       string
	Containers []*ContainerReport
}

func (p *PodReport) AppendContainer(c *ContainerReport) {
	p.Containers = append(p.Containers, c)
}

func NewPodman() (*podman, error) {
	podmanConnection, err := bindings.NewConnection(context.Background(), "unix://run/podman/podman.sock")
	if err != nil {
		return nil, err
	}
	return &podman{
		podmanConnection: podmanConnection,
	}, nil
}

func (p *podman) List() ([]api2.WorkloadInfo, error) {
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

func (p *podman) Exists(workloadId string) (bool, error) {
	exists, err := pods.Exists(p.podmanConnection, workloadId, nil)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (p *podman) Remove(workloadId string) error {
	exists, err := p.Exists(workloadId)
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

// getContainerDetails returns some basic information about a container
func (p *podman) getContainerDetails(containerId string) (*ContainerReport, error) {
	data, err := containers.Inspect(p.podmanConnection, containerId, nil)
	if err != nil {
		return nil, err
	}
	// this is the default one
	network, ok := data.NetworkSettings.Networks["podman"]
	if !ok {
		return nil, fmt.Errorf("cannot retrieve container '%s' network information", containerId)
	}

	return &ContainerReport{
		IPAddress: network.InspectBasicNetworkConfig.IPAddress,
		Id:        containerId,
		Name:      data.Name,
	}, nil
}

func (p *podman) Run(manifestPath, authFilePath string) ([]*PodReport, error) {
	options := play.KubeOptions{
		Authfile: &authFilePath,
	}
	report, err := play.Kube(p.podmanConnection, manifestPath, &options)
	if err != nil {
		return nil, err
	}

	var podIds = make([]*PodReport, len(report.Pods))
	for i, pod := range report.Pods {
		report := &PodReport{Id: pod.ID}
		for _, container := range pod.Containers {
			c, err := p.getContainerDetails(container)
			if err != nil {
				log.Errorf("cannot get container information: %v", err)
				continue
			}
			report.AppendContainer(c)
		}
		podIds[i] = report
	}
	return podIds, nil
}

func (p *podman) Start(workloadId string) error {
	_, err := pods.Start(p.podmanConnection, workloadId, nil)
	if err != nil {
		return err
	}
	return nil
}

func (p *podman) ListSecrets() (map[string]struct{}, error) {
	result := map[string]struct{}{}
	listResult, err := secrets.List(p.podmanConnection, nil)
	if err != nil {
		return nil, err
	}
	for _, secret := range listResult {
		result[secret.Spec.Name] = struct{}{}
	}
	return result, nil
}

func (p *podman) RemoveSecret(name string) error {
	return secrets.Remove(p.podmanConnection, name)
}

func (p *podman) CreateSecret(name, data string) error {
	_, err := secrets.Create(p.podmanConnection, strings.NewReader(data), &secrets.CreateOptions{Name: &name})
	return err
}

func (p *podman) UpdateSecret(name, data string) error {
	err := p.RemoveSecret(name)
	if err != nil {
		return err
	}
	return p.CreateSecret(name, data)
}

func (p *Podman) GenerateSystemdFiles(podName string, monitoringInterval uint) (map[string]string, error) {
	useName := true
	report, err := generate.Systemd(p.podmanConnection, podName, &generate.SystemdOptions{UseName: &useName, RestartSec: &monitoringInterval})
	if err != nil {
		return nil, err
	}
	return report.Units, nil
}
