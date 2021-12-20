package workload

import (
	"fmt"

	"github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/mapping"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/service"

	"git.sr.ht/~spc/go-log"
	api2 "github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/network"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/podman"
	v1 "k8s.io/api/core/v1"

	_ "github.com/golang/mock/mockgen/model"
)

const nfTableName string = "edge"

type Observer interface {
	WorkloadRemoved(workloadName string)
	WorkloadStarted(workloadName string, report []*podman.PodReport)
}

//go:generate mockgen -package=workload -destination=mock_wrapper.go . WorkloadWrapper
type WorkloadWrapper interface {
	Init() error
	RegisterObserver(Observer)
	List() ([]api.WorkloadInfo, error)
	Remove(string) error
	Run(*v1.Pod, string, string) error
	Start(*v1.Pod) error
	PersistConfiguration() error
	RemoveTable() error
	RemoveMappingFile() error
	ListSecrets() (map[string]struct{}, error)
	RemoveSecret(string) error
	CreateSecret(string, string) error
	UpdateSecret(string, string) error
}

// Workload manages the workload and its configuration on the device
type Workload struct {
	workloads          podman.Podman
	netfilter          network.Netfilter
	mappingRepository  mapping.MappingRepository
	observers          []Observer
	serviceManager     *service.SystemdManager
	monitoringInterval int64
}

func newWorkloadInstance(configDir string, monitoringInterval int64) (*Workload, error) {
	newPodman, err := podman.NewPodman()
	if err != nil {
		return nil, err
	}
	netfilter, err := network.NewNetfilter()
	if err != nil {
		return nil, err
	}
	mappingRepository, err := mapping.NewMappingRepository(configDir)
	if err != nil {
		return nil, err
	}
	serviceManager, err := service.NewSystemdManager(configDir)
	if err != nil {
		return nil, err
	}
	return &Workload{
		workloads:          newPodman,
		netfilter:          netfilter,
		mappingRepository:  mappingRepository,
		serviceManager:     serviceManager,
		monitoringInterval: monitoringInterval,
	}, nil
}

func (ww *Workload) RegisterObserver(observer Observer) {
	ww.observers = append(ww.observers, observer)
}

func (ww Workload) Init() error {
	return ww.netfilter.AddTable(nfTableName)
}

func (ww Workload) List() ([]api2.WorkloadInfo, error) {
	infos, err := ww.workloads.List()
	if err != nil {
		return nil, err
	}
	for i := range infos {
		mappedName := ww.mappingRepository.GetName(infos[i].Id)
		if mappedName != "" {
			infos[i].Name = mappedName
		}
	}
	return infos, err
}

func (ww Workload) Remove(workloadName string) error {
	id := ww.mappingRepository.GetId(workloadName)
	if id == "" {
		id = workloadName
	}

	// Remove the service configuration from the system:
	if err := ww.removeService(workloadName); err != nil {
		return err
	}

	if err := ww.workloads.Remove(id); err != nil {
		return err
	}
	if err := ww.netfilter.DeleteChain(nfTableName, workloadName); err != nil {
		log.Errorf("failed to delete chain '%[1]s' from table '%[2]s' for workload '%[1]s': %[3]v", workloadName, nfTableName, err)
	}
	if err := ww.mappingRepository.Remove(workloadName); err != nil {
		return err
	}
	for _, observer := range ww.observers {
		observer.WorkloadRemoved(workloadName)
	}
	return nil
}

func (ww Workload) RemoveTable() error {
	log.Infof("deleting table %s", nfTableName)
	if err := ww.netfilter.DeleteTable(nfTableName); err != nil {
		log.Errorf("failed to delete table %s: %v", nfTableName, err)
		return err
	}
	return nil
}
func (ww Workload) RemoveMappingFile() error {
	log.Infof("deleting table %s", nfTableName)
	if err := ww.mappingRepository.RemoveMappingFile(); err != nil {
		log.Errorf("failed to remove mapping file: %v", err)
		return err
	}
	return nil
}

func (ww Workload) Run(workload *v1.Pod, manifestPath string, authFilePath string) error {
	if err := ww.applyNetworkConfiguration(workload); err != nil {
		return err
	}
	podIds, err := ww.workloads.Run(manifestPath, authFilePath)
	if err != nil {
		return err
	}

	// Create the system service to manage the pod:
	svc, err := ww.createService(workload)
	if err != nil {
		return err
	}
	err = ww.serviceManager.Add(svc)
	if err != nil {
		return err
	}

	return ww.mappingRepository.Add(workload.Name, podIds[0].Id)
}

func (ww Workload) removeService(workloadName string) error {
	svc := ww.serviceManager.Get(workloadServiceName(workloadName))
	if svc == nil {
		return nil
	}

	// Ignore stop failure:
	svc.Stop()

	// Remove the service from the system:
	if err := svc.Remove(); err != nil {
		return err
	}

	err := ww.serviceManager.Remove(svc)
	if err != nil {
		return nil
	}

	return nil
}

func (ww Workload) createService(workload *v1.Pod) (*service.Systemd, error) {
	units, err := ww.workloads.GenerateSystemdFiles(workloadServiceName(workload.GetName()), ww.monitoringInterval)
	if err != nil {
		return nil, err
	}

	svc, err := service.NewSystemd(workloadServiceName(workload.GetName()), units)
	if err != nil {
		return nil, err
	}
	if err = svc.Add(); err != nil {
		return nil, err
	}
	if err = svc.Enable(); err != nil {
		return nil, err
	}
	if err = svc.Start(); err != nil {
		return nil, err
	}

	return svc, nil
}

func workloadServiceName(workloadName string) string {
	return workloadName + "_pod"
}

func (ww Workload) applyNetworkConfiguration(workload *v1.Pod) error {
	hostPorts, err := getHostPorts(workload)
	if err != nil {
		log.Error(err)
		return err
	}
	if len(hostPorts) == 0 {
		return nil
	}
	// skip existence check since chain is not changed if already exists
	if err := ww.netfilter.AddChain(nfTableName, workload.Name); err != nil {
		return fmt.Errorf("failed to create chain for workload %s: %v", workload.Name, err)
	}

	// for workloads, a port will be opened for the pod based on hostPort
	for _, p := range hostPorts {
		rule := fmt.Sprintf("tcp dport %d ct state new,established counter accept", p)
		if err := ww.netfilter.AddRule(nfTableName, workload.Name, rule); err != nil {
			return fmt.Errorf("failed to add rule %s for workload %s: %v", rule, workload.Name, err)
		}
	}
	return nil
}

func (ww Workload) Start(workload *v1.Pod) error {
	ww.netfilter.DeleteChain(nfTableName, workload.Name)
	if err := ww.applyNetworkConfiguration(workload); err != nil {
		return err
	}

	podId := ww.mappingRepository.GetId(workload.Name)
	if err := ww.workloads.Start(podId); err != nil {
		return err
	}
	return nil
}

func (ww Workload) PersistConfiguration() error {
	return ww.mappingRepository.Persist()
}

func (ww Workload) ListSecrets() (map[string]struct{}, error) {
	return ww.workloads.ListSecrets()
}

func (ww Workload) RemoveSecret(name string) error {
	return ww.workloads.RemoveSecret(name)
}

func (ww Workload) CreateSecret(name, data string) error {
	return ww.workloads.CreateSecret(name, data)
}

func (ww Workload) UpdateSecret(name, data string) error {
	return ww.workloads.UpdateSecret(name, data)
}

func getHostPorts(workload *v1.Pod) ([]int32, error) {
	var hostPorts []int32
	for _, c := range workload.Spec.Containers {
		for _, p := range c.Ports {
			if p.HostPort > 0 && p.HostPort < 65536 {
				hostPorts = append(hostPorts, p.HostPort)
			} else {
				return nil, fmt.Errorf("illegal host port number %d for container %s in workload %s", p.HostPort, c.Name, workload.Name)
			}
		}
	}
	return hostPorts, nil
}
