package workload

import (
	"context"
	"fmt"

	"io"
	"sync"

	"github.com/project-flotta/flotta-device-worker/internal/service"

	"git.sr.ht/~spc/go-log"
	"github.com/project-flotta/flotta-device-worker/internal/workload/api"
	"github.com/project-flotta/flotta-device-worker/internal/workload/mapping"
	"github.com/project-flotta/flotta-device-worker/internal/workload/network"
	"github.com/project-flotta/flotta-device-worker/internal/workload/podman"
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
	Logs(podID string, res io.Writer) (context.CancelFunc, error)
	Remove(string) error
	Stop(string) error
	Run(*v1.Pod, string, string, map[string]string) error
	Start(*v1.Pod) error
	PersistConfiguration() error
	RemoveTable() error
	RemoveMappingFile() error
	RemoveServicesFile() error
	ListSecrets() (map[string]struct{}, error)
	RemoveSecret(string) error
	CreateSecret(string, string) error
	UpdateSecret(string, string) error
	ListenServiceEvents()
}

// Workload manages the workload and its configuration on the device
type Workload struct {
	podManager         podman.Podman
	netfilter          network.Netfilter
	mappingRepository  mapping.MappingRepository
	observers          []Observer
	serviceManager     service.SystemdManager
	monitoringInterval uint
	lock               sync.RWMutex
	systemdEventCh     <-chan *service.Event
}

func NewWorkload(p podman.Podman, n network.Netfilter, m mapping.MappingRepository, s service.SystemdManager, monitoringInterval uint, systemdEventCh <-chan *service.Event) *Workload {
	return &Workload{
		podManager:         p,
		netfilter:          n,
		mappingRepository:  m,
		serviceManager:     s,
		monitoringInterval: monitoringInterval,
		systemdEventCh:     systemdEventCh,
	}
}

func newWorkloadInstance(configDir string, monitoringInterval uint, systemdEventCh <-chan *service.Event) (*Workload, error) {
	newPodman, err := podman.NewPodman()
	if err != nil {
		return nil, fmt.Errorf("workload cannot initialize podman manager: %w", err)
	}
	netfilter, err := network.NewNetfilter()
	if err != nil {
		return nil, fmt.Errorf("workload cannot initialize netfilter manager: %w", err)
	}
	mappingRepository, err := mapping.NewMappingRepository(configDir)
	if err != nil {
		return nil, fmt.Errorf("workload cannot initialize mapping repository: %w", err)
	}

	serviceManager, err := service.NewSystemdManager(configDir)
	if err != nil {
		return nil, fmt.Errorf("workload cannot initialize systemd manager: %w", err)
	}

	ww := &Workload{
		podManager:         newPodman,
		netfilter:          netfilter,
		mappingRepository:  mappingRepository,
		serviceManager:     serviceManager,
		monitoringInterval: monitoringInterval,
		systemdEventCh:     systemdEventCh,
	}

	return ww, nil
}

func (ww *Workload) RegisterObserver(observer Observer) {
	ww.lock.Lock()
	defer ww.lock.Unlock()
	ww.observers = append(ww.observers, observer)
}

func (ww *Workload) Init() error {
	// Enable auto-update podman timer:
	svc, err := service.NewSystemd("podman-auto-update", nil, service.UserBus)
	if err != nil {
		return err
	}
	if err := svc.Start(); err != nil {
		return err
	}

	// Init netfiliter table:
	return ww.netfilter.AddTable(nfTableName)
}

func (ww *Workload) List() ([]api.WorkloadInfo, error) {
	infos, err := ww.podManager.List()
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

func (ww *Workload) Logs(podID string, res io.Writer) (context.CancelFunc, error) {
	return ww.podManager.Logs(podID, res)
}

func (ww *Workload) Remove(workloadName string) error {
	id := ww.mappingRepository.GetId(workloadName)
	if id == "" {
		id = workloadName
	}

	// Remove the service configuration from the system:
	if err := ww.removeService(workloadName); err != nil {
		return err
	}

	if err := ww.podManager.Remove(id); err != nil {
		return err
	}
	if err := ww.netfilter.DeleteChain(nfTableName, workloadName); err != nil {
		log.Errorf("failed to delete chain '%[1]s' from table '%[2]s' for workload '%[1]s': %[3]v", workloadName, nfTableName, err)
	}
	if err := ww.mappingRepository.Remove(workloadName); err != nil {
		return err
	}
	return nil
}

func (ww *Workload) Stop(workloadName string) error {
	id := ww.mappingRepository.GetId(workloadName)
	if id == "" {
		id = workloadName
	}
	err := ww.podManager.Stop(id)
	return err
}

func (ww *Workload) RemoveTable() error {
	log.Infof("deleting table %s", nfTableName)
	if err := ww.netfilter.DeleteTable(nfTableName); err != nil {
		log.Errorf("failed to delete table %s: %v", nfTableName, err)
		return err
	}
	return nil
}

func (ww *Workload) RemoveServicesFile() error {
	log.Infof("deleting services file")
	if err := ww.serviceManager.RemoveServicesFile(); err != nil {
		log.Errorf("failed to remove services file: %v", err)
		return err
	}
	return nil
}

func (ww *Workload) RemoveMappingFile() error {
	log.Infof("deleting mapping file")
	if err := ww.mappingRepository.RemoveMappingFile(); err != nil {
		log.Errorf("failed to remove mapping file: %v", err)
		return err
	}
	return nil
}

func (ww *Workload) Run(workload *v1.Pod, manifestPath string, authFilePath string, podmanAnnotations map[string]string) error {
	if err := ww.applyNetworkConfiguration(workload); err != nil {
		return err
	}
	podIds, err := ww.podManager.Run(manifestPath, authFilePath, podmanAnnotations)
	if err != nil {
		return err
	}

	// Must be called before GenerateSystemdService:
	if err = ww.mappingRepository.Add(workload.Name, podIds[0].Id); err != nil {
		return err
	}

	// Create the system service to manage the pod:
	svc, err := ww.podManager.GenerateSystemdService(workload, manifestPath, ww.monitoringInterval)
	if err != nil {
		return fmt.Errorf("error while generating systemd service: %v", err)
	}

	log.Infof("Creating service for %s", workload.Name)
	err = ww.createService(svc)
	if err != nil {
		return fmt.Errorf("error while starting service: %v", err)
	}
	log.Infof("Registering service %s", workload.Name)
	err = ww.serviceManager.Add(svc)
	if err != nil {
		return fmt.Errorf("error while updating service manager: %v", err)
	}
	return nil
}

func (ww *Workload) removeService(workloadName string) error {
	svc := ww.serviceManager.Get(workloadName)
	if svc == nil {
		return nil
	}

	// Ignore stop failure:
	err := svc.Stop()
	if err != nil {
		return fmt.Errorf("unable to stop service %s:%s", workloadName, err)
	}

	// Disable the service from the system:
	if err := svc.Disable(); err != nil {
		return fmt.Errorf("unable to disable systemd service for '%s': %s", workloadName, err)
	}

	// Remove the service from the system:
	if err := svc.Remove(); err != nil {
		return fmt.Errorf("unable to remove systemd service for '%s': %s", workloadName, err)
	}

	err = ww.serviceManager.Remove(svc)
	if err != nil {
		log.Errorf("Unable to remove service from serviceManager %s:%s", workloadName, err)
	}
	return nil
}

func (ww *Workload) createService(svc service.Service) error {
	if err := svc.Add(); err != nil {
		return fmt.Errorf("cannot add systemd service '%s': %v", svc.GetName(), err)
	}
	if err := svc.Enable(); err != nil {
		return fmt.Errorf("cannot enable systemd service '%s': %v", svc.GetName(), err)
	}

	err := svc.Start()
	if err != nil {
		// A service maybe is already in place, and started, so no returning an
		// error here.
		log.Errorf("cannot start systemd service '%s': %v", svc.GetName(), err)
	}

	return nil
}

func (ww *Workload) applyNetworkConfiguration(workload *v1.Pod) error {
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

func (ww *Workload) Start(workload *v1.Pod) error {
	_ = ww.netfilter.DeleteChain(nfTableName, workload.Name)
	if err := ww.applyNetworkConfiguration(workload); err != nil {
		return err
	}

	podId := ww.mappingRepository.GetId(workload.Name)
	if err := ww.podManager.Start(podId); err != nil {
		return err
	}
	return nil
}

func (ww *Workload) PersistConfiguration() error {
	return ww.mappingRepository.Persist()
}

func (ww *Workload) ListSecrets() (map[string]struct{}, error) {
	return ww.podManager.ListSecrets()
}

func (ww *Workload) RemoveSecret(name string) error {
	return ww.podManager.RemoveSecret(name)
}

func (ww *Workload) CreateSecret(name, data string) error {
	return ww.podManager.CreateSecret(name, data)
}

func (ww *Workload) UpdateSecret(name, data string) error {
	return ww.podManager.UpdateSecret(name, data)
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

func (w *Workload) ListenServiceEvents() {
	log.Debug("Starting routine to listen for service events")
	for {
		event := <-w.systemdEventCh
		log.Debugf("Event received: %s", string(event.Type))
		w.lock.Lock()
		observers := w.observers
		w.lock.Unlock()
		log.Debugf("Number of observers %d: %+v", len(observers), observers)
		switch event.Type {
		case service.EventStarted:
			log.Infof("Service for workload %s started", event.WorkloadName)
			report, err := w.podManager.GetPodReportForPodName(event.WorkloadName)
			if err != nil {
				log.Errorf("unable to get pod report for workload %s:%v", event.WorkloadName, err)
			}
			for _, observer := range observers {
				log.Debugf("Triggering WorkloadStarted in observer '%s' for workload %s", observer, event.WorkloadName)
				observer.WorkloadStarted(event.WorkloadName, []*podman.PodReport{report})
			}
		case service.EventStopped:
			log.Infof("Service for workload %s stopped", event.WorkloadName)
			for _, observer := range observers {
				log.Debugf("Triggering WorkloadRemoved in observer '%s' for workload %s", observer, event.WorkloadName)
				observer.WorkloadRemoved(event.WorkloadName)
			}
		default:
			log.Errorf("Unknown event %s", event.Type)
		}
	}
}
