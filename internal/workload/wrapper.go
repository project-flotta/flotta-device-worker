package workload

import (
	"fmt"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/mapping"

	"git.sr.ht/~spc/go-log"
	api2 "github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/network"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/podman"
	v1 "k8s.io/api/core/v1"
)

const nfTableName string = "edge"

// workloadWrapper manages the workload and its configuration on the device
type workloadWrapper struct {
	workloads         *podman.Podman
	netfilter         *network.Netfilter
	mappingRepository *mapping.MappingRepository
}

func newWorkloadWrapper(configDir string) (*workloadWrapper, error) {
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
	return &workloadWrapper{
		workloads:         newPodman,
		netfilter:         netfilter,
		mappingRepository: mappingRepository,
	}, nil
}

func (ww workloadWrapper) Init() error {
	return ww.netfilter.AddTable(nfTableName)
}

func (ww workloadWrapper) List() ([]api2.WorkloadInfo, error) {
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

func (ww workloadWrapper) Remove(workloadName string) error {
	id := ww.mappingRepository.GetId(workloadName)
	if id == "" {
		id = workloadName
	}
	if err := ww.workloads.Remove(id); err != nil {
		return err
	}
	if err := ww.netfilter.DeleteChain(nfTableName, workloadName); err != nil {
		log.Errorf("failed to delete chain %[1]s from %s table for workload %[1]s: %v", workloadName, nfTableName, err)
	}
	if err := ww.mappingRepository.Remove(workloadName); err != nil {
		return err
	}
	return nil
}

func (ww workloadWrapper) Run(workload *v1.Pod, manifestPath string) error {
	if err := ww.applyNetworkConfiguration(workload); err != nil {
		return err
	}
	podIds, err := ww.workloads.Run(manifestPath)
	if err != nil {
		return err
	}
	return ww.mappingRepository.Add(workload.Name, podIds[0])
}

func (ww workloadWrapper) applyNetworkConfiguration(workload *v1.Pod) error {
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

func (ww workloadWrapper) Start(workload *v1.Pod) error {
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

func (ww workloadWrapper) PersistConfiguration() error{
	return ww.mappingRepository.Persist()
}

func getHostPorts(workload *v1.Pod) ([]int32, error) {
	hostPorts := []int32{}
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
