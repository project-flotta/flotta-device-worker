package podman

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/blang/semver"
	"github.com/go-openapi/swag"
	"github.com/project-flotta/flotta-device-worker/internal/service"
	api "github.com/project-flotta/flotta-device-worker/internal/workload/api"

	"github.com/containers/podman/v4/pkg/bindings"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/bindings/generate"
	"github.com/containers/podman/v4/pkg/bindings/play"
	"github.com/containers/podman/v4/pkg/bindings/pods"
	"github.com/containers/podman/v4/pkg/bindings/secrets"
	"github.com/containers/podman/v4/pkg/bindings/system"
	"github.com/containers/podman/v4/pkg/domain/entities"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

const (
	StoppedContainer = "stoppedContainer"
	StartedContainer = "startedContainer"

	DefaultNetworkName = "podman"

	podmanBinary                  = "/usr/bin/podman"
	autoUpdateServiceUnitTemplate = `[Unit]
Description=Podman {{ .PodName }}.service
Wants=network-online.target
After=network-online.target

[Service]
Restart=on-failure
RestartSec=15
ExecStart={{ .PodmanBinary }} play kube --replace {{ .ManifestPath }}
ExecStop={{ .PodmanBinary }} play kube --down {{ .ManifestPath }}
Type=forking

[Install]
WantedBy=default.target
`
)

var (
	logTailLines = "1"
	boolTrue     = true
	boolFalse    = false
)

type AutoUpdateUnit struct {
	ManifestPath string
	PodName      string
	PodmanBinary string
}

//go:generate mockgen -package=podman -destination=mock_podman.go . Podman
type Podman interface {
	List() ([]api.WorkloadInfo, error)
	Remove(workloadId string) error
	Run(manifestPath, authFilePath string, annotations map[string]string) ([]*PodReport, error)
	Start(workloadId string) error
	Stop(workloadId string) error
	ListSecrets() (map[string]struct{}, error)
	RemoveSecret(name string) error
	CreateSecret(name, data string) error
	UpdateSecret(name, data string) error
	Exists(workloadId string) (bool, error)
	GenerateSystemdService(workload *v1.Pod, manifestPath string, monitoringInterval uint) (service.Service, error)
	Logs(podID string, res io.Writer) (context.CancelFunc, error)
	GetPodReportForPodName(podName string) (*PodReport, error)
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

const DefaultTimeoutForStoppingInSeconds int = 5

type podman struct {
	podmanConnection   context.Context
	timeoutForStopping int
	eventCh            chan service.Event
	cancel             chan bool
}

func NewPodman() (*podman, error) {
	podmanConnection, err := podmanConnection()
	if err != nil {
		return nil, err
	}
	p := &podman{
		podmanConnection:   podmanConnection,
		timeoutForStopping: DefaultTimeoutForStoppingInSeconds,
		eventCh:            make(chan service.Event, 1000),
		cancel:             make(chan bool),
	}
	err = p.MinVersion()
	if err != nil {
		return nil, err
	}

	return p, nil
}

func podmanConnection() (context.Context, error) {
	podmanConnection, err := bindings.NewConnection(context.Background(), fmt.Sprintf("unix:%s/podman/podman.sock", os.Getenv("FLOTTA_XDG_RUNTIME_DIR")))
	if err != nil {
		return nil, err
	}

	return podmanConnection, err
}

func (p *podman) MinVersion() error {
	version, err := system.Info(p.podmanConnection, nil)
	if err != nil {
		return fmt.Errorf("cannot get podman version: %v", err)
	}
	v := semver.MustParse(string(version.Version.Version))
	if v.Major > 4 {
		return nil
	}
	if v.Major == 4 && v.Minor >= 2 {
		return nil
	}
	return fmt.Errorf("podman version '%s' is not supported, needs >= v4.2", version.Version.Version)
}

func (p *podman) List() ([]api.WorkloadInfo, error) {
	podList, err := pods.List(p.podmanConnection, nil)
	if err != nil {
		return nil, err
	}
	var workloads []api.WorkloadInfo
	for _, pod := range podList {
		wi := api.WorkloadInfo{
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
	network, ok := data.NetworkSettings.Networks[DefaultNetworkName]
	if !ok {
		return nil, fmt.Errorf("cannot retrieve container '%s' network information", containerId)
	}

	return &ContainerReport{
		IPAddress: network.InspectBasicNetworkConfig.IPAddress,
		Id:        containerId,
		Name:      data.Name,
	}, nil
}

func (p *podman) GetPodReportForPodName(podID string) (*PodReport, error) {
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

func (p *podman) Stop(workloadId string) error {
	exists, err := pods.Exists(p.podmanConnection, workloadId, nil)
	if err != nil {
		return err
	}
	if exists {
		_, err := pods.Stop(p.podmanConnection, workloadId, &pods.StopOptions{Timeout: &p.timeoutForStopping})
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *podman) Run(manifestPath, authFilePath string, annotations map[string]string) ([]*PodReport, error) {
	network := []string{DefaultNetworkName}
	options := play.KubeOptions{
		Authfile:    &authFilePath,
		Network:     &network,
		Annotations: annotations,
		Start:       &boolFalse,
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

func hasAutoUpdateEnabled(labels map[string]string) bool {
	for label := range labels {
		if strings.Contains(label, "io.containers.autoupdate") {
			return true
		}
	}

	return false
}

func (p *podman) GenerateSystemdService(workload *v1.Pod, manifestPath string, monitoringInterval uint) (service.Service, error) {
	var svc service.Service
	podName := workload.Name

	// Since podman don't support generation of the systemd services that re-creates the pod, instead of restarting it,
	// for pods/containers created via the API, we must create the custom systemd service for pods, which has enabled
	// autoupdate feature, because in case the image of the containers is updated we need to re-create the containers.
	if !hasAutoUpdateEnabled(workload.Labels) {
		useName := true

		var report *entities.GenerateSystemdReport
		var err error

		report, err = generate.Systemd(p.podmanConnection, podName, &generate.SystemdOptions{PodPrefix: swag.String(""), RestartSec: &monitoringInterval, UseName: &useName})
		if err != nil {
			return nil, err
		}

		svc, err = service.NewSystemd(podName, report.Units, service.UserBus, p.eventCh)
		if err != nil {
			return nil, err
		}
	} else {
		var unit bytes.Buffer

		tmp := template.New("unit")
		t, err := tmp.Parse(autoUpdateServiceUnitTemplate)
		if err != nil {
			return nil, err
		}
		err = t.Execute(&unit, AutoUpdateUnit{ManifestPath: manifestPath, PodmanBinary: podmanBinary, PodName: podName})
		if err != nil {
			return nil, err
		}
		units := map[string]string{podName: unit.String()}
		svc, err = service.NewSystemd(podName, units, service.UserBus, p.eventCh)
		if err != nil {
			return nil, err
		}
	}

	return svc, nil
}

// Retrieve all pods logs and send that to the given io.Writer
func (p *podman) Logs(podID string, res io.Writer) (context.CancelFunc, error) {
	podInfo, err := pods.Inspect(p.podmanConnection, podID, nil)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(p.podmanConnection)
	readers := map[string]*bufio.Reader{}
	for _, container := range podInfo.Containers {
		res := &bytes.Buffer{}
		err := p.containerLog(ctx, container.ID, res)

		if err != nil {
			cancel()
			return nil, err
		}
		readers[container.ID] = bufio.NewReader(res)
	}

	go func() {
		for {
			hadLines := false // did we read any lines in this iteration?
			for containerID, buffer := range readers {
				line, err := buffer.ReadString('\n')
				if err != nil && !errors.Is(err, io.EOF) {
					log.Errorf("Cannot read container log line from buffer: %v podID=%s containerID=%s", err, podID, containerID)
					continue
				}

				if len(line) == 0 {
					continue
				}

				hadLines = true
				_, err = res.Write([]byte(fmt.Sprintf("%s: %s", containerID, line)))
				if err != nil {
					log.Errorf("Cannot write container log line: %v podID=%s containerID=%s", err, podID, containerID)
				}
			}

			if hadLines {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			} else {
				select {
				case <-ctx.Done():
					return
				case <-time.NewTimer(100 * time.Millisecond).C:
					continue
				}
			}
		}
	}()
	return cancel, nil
}

func (p *podman) containerLog(ctx context.Context, containerID string, res io.Writer) error {
	opts := containers.LogOptions{
		Follow: &boolTrue,
		Stderr: &boolTrue,
		Stdout: &boolTrue,
		Tail:   &logTailLines,
	}
	stdoutCh := make(chan string, 100)
	stderrCh := make(chan string, 100)

	go func() {
		err := containers.Logs(ctx, containerID, &opts, stdoutCh, stderrCh)
		if err != nil {
			log.Errorf("cannot get logs for container '%s': %v", containerID, err)
		}
	}()

	go func() {
		for {
			select {
			case line := <-stdoutCh:
				_, err := io.WriteString(res, line+"\n")
				if err != nil {
					log.Errorf("cannot write log line to io.Writer for container '%v': %v", containerID, err)
				}
			case line := <-stderrCh:
				_, err := io.WriteString(res, line+"\n")
				if err != nil {
					log.Errorf("cannot write log line to io.Writer for container '%v': %v", containerID, err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}
