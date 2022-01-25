package podman

import (
	"context"
	"fmt"
	"strings"

	"github.com/jakub-dzon/k4e-device-worker/internal/service"

	"git.sr.ht/~spc/go-log"
	podmanEvents "github.com/containers/podman/v3/libpod/events"
	"github.com/containers/podman/v3/pkg/bindings"
	"github.com/containers/podman/v3/pkg/bindings/containers"
	"github.com/containers/podman/v3/pkg/bindings/generate"
	"github.com/containers/podman/v3/pkg/bindings/play"
	"github.com/containers/podman/v3/pkg/bindings/pods"
	"github.com/containers/podman/v3/pkg/bindings/secrets"
	"github.com/containers/podman/v3/pkg/bindings/system"
	"github.com/containers/podman/v3/pkg/domain/entities"
	api2 "github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
)

const (
	StoppedContainer = "stoppedContainer"
	StartedContainer = "startedContainer"

	podmanStart  = string(podmanEvents.Start)
	podmanRemove = string(podmanEvents.Remove)
	podmanStop   = string(podmanEvents.Stop)
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
	GenerateSystemdService(podName string, monitoringInterval uint) (service.Service, error)
}

type PodmanEvent struct {
	Event        string
	WorkloadName string
	Report       *PodReport
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

type podman struct {
	podmanConnection context.Context
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

func (p *podman) Events(events chan *PodmanEvent) {
	evchan := make(chan entities.Event, 1000)
	cancel := make(chan bool)
	booltrue := true

	workloads, err := p.List()
	if err != nil {
		log.Errorf("Cannot get the list of running pods: %v", err)
	}

	if workloads != nil {
		for _, wrk := range workloads {
			report, err := p.getPodReportforId(wrk.Id)
			if err != nil {
				log.Errorf("Cannot get pod report for pod '%v': ", wrk.Name, err)
				continue
			}
			event := &PodmanEvent{
				WorkloadName: wrk.Name,
				Event:        StartedContainer,
				Report:       report,
			}
			go func() { events <- event }()
		}
	}

	// subroutine that reads the event chan and sending proper messages to the
	// wrapper
	go func() {
		for {
			select {
			case msg := <-evchan:
				event := &PodmanEvent{
					WorkloadName: msg.Actor.Attributes["name"],
				}
				switch msg.Action {
				// create event is avoided because containers are not yet created and
				// the flow is created->started
				case podmanStart:
					event.Event = StartedContainer
					report, err := p.getPodReportforId(msg.ID)
					if err != nil {
						log.Error("cannot get current pod information on event: %v", err)
						continue
					}
					event.Report = report
				case podmanRemove, podmanStop:
					event.Event = StoppedContainer
				default:
					continue
				}
				events <- event
			}
		}
	}()

	// subroutine to track events
	go system.Events(p.podmanConnection, evchan, cancel, &system.EventsOptions{
		Filters: map[string][]string{
			"type": {"pod"},
		},
		Stream: &booltrue,
	})
}

func (p *podman) getPodReportforId(podID string) (*PodReport, error) {
	podInfo, err := pods.Inspect(p.podmanConnection, podID, nil)
	if err != nil {
		return nil, err
	}

	report := &PodReport{Id: podID, Name: podInfo.Name}
	for _, container := range podInfo.Containers {
		c, err := p.getContainerDetails(container.ID)
		if err != nil {
			log.Errorf("cannot get container information: %v", err)
			continue
		}
		report.AppendContainer(c)
	}
	return report, nil
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

func (p *podman) GenerateSystemdService(podId string, monitoringInterval uint) (service.Service, error) {
	report, err := generate.Systemd(p.podmanConnection, podId, &generate.SystemdOptions{RestartSec: &monitoringInterval})
	if err != nil {
		return nil, err
	}

	svc, err := service.NewSystemd(podId, "pod-", report.Units)
	if err != nil {
		return nil, err
	}

	return svc, nil
}
