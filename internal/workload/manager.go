package workload

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/jakub-dzon/k4e-device-worker/internal/volumes"

	"git.sr.ht/~spc/go-log"
	api2 "github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
	"github.com/jakub-dzon/k4e-operator/models"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	defaultWorkloadsMonitoringInterval = 15
)

type WorkloadManager struct {
	manifestsDir   string
	volumesDir     string
	workloads      *workloadWrapper
	managementLock sync.Locker
	ticker         *time.Ticker
}

type podAndPath struct {
	pod          v1.Pod
	manifestPath string
}

func NewWorkloadManager(dataDir string) (*WorkloadManager, error) {
	manifestsDir := path.Join(dataDir, "manifests")
	if err := os.MkdirAll(manifestsDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create directory: %w", err)
	}
	volumesDir := path.Join(dataDir, "volumes")
	if err := os.MkdirAll(volumesDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create directory: %w", err)
	}
	wrapper, err := newWorkloadWrapper(dataDir)
	if err != nil {
		return nil, err
	}
	manager := WorkloadManager{
		manifestsDir:   manifestsDir,
		volumesDir:     volumesDir,
		workloads:      wrapper,
		managementLock: &sync.Mutex{},
	}
	if err := manager.workloads.Init(); err != nil {
		return nil, err
	}

	manager.initTicker(defaultWorkloadsMonitoringInterval)
	return &manager, nil
}

func (w *WorkloadManager) ListWorkloads() ([]api2.WorkloadInfo, error) {
	return w.workloads.List()
}

func (w *WorkloadManager) GetExportedHostPath(workloadName string) string {
	return volumes.HostPathVolumePath(w.volumesDir, workloadName)
}

func (w *WorkloadManager) Update(configuration models.DeviceConfigurationMessage) error {
	w.managementLock.Lock()
	defer w.managementLock.Unlock()

	configuredWorkloadNameSet := make(map[string]struct{})
	for _, workload := range configuration.Workloads {
		log.Tracef("Deploying workload: %s", workload.Name)
		configuredWorkloadNameSet[workload.Name] = struct{}{}
		// TODO: change error handling from fail fast to best effort (deploy as many workloads as possible)
		pod, err := w.toPod(workload)
		if err != nil {
			return err
		}
		manifestPath := w.getManifestPath(pod.Name)
		podYaml, err := w.toPodYaml(pod)
		if err != nil {
			return nil
		}
		if !w.podModified(manifestPath, podYaml) {
			log.Tracef("Pod '%s' definition is unchanged (%s)", workload.Name, manifestPath)
			continue
		}
		err = w.storeManifest(manifestPath, podYaml)
		if err != nil {
			return err
		}

		err = w.workloads.Remove(workload.Name)
		if err != nil {
			log.Errorf("Error removing workload: %v", err)
			return err
		}
		err = w.workloads.Run(pod, manifestPath)
		if err != nil {
			log.Errorf("Cannot run workload: %v", err)
			return err
		}
	}

	deployedWorkloadByName, err := w.indexWorkloads()
	if err != nil {
		log.Errorf("Cannot get deployed workloads: %v", err)
		return err
	}
	// Remove any workloads that don't correspond to the configured ones
	for name := range deployedWorkloadByName {
		if _, ok := configuredWorkloadNameSet[name]; !ok {
			log.Infof("Workload not found: %s. Removing", name)
			manifestPath := w.getManifestPath(name)
			err := os.Remove(manifestPath)
			if err != nil {
				if !os.IsNotExist(err) {
					return err
				}
			}

			if err := w.workloads.Remove(name); err != nil {
				return err
			}
			log.Infof("Workload %s removed", name)
		}
	}
	// Reset the interval of the current monitoring routine
	if configuration.WorkloadsMonitoringInterval != nil {
		w.ticker.Reset(time.Duration(*configuration.WorkloadsMonitoringInterval))
	}
	return nil
}

func (w *WorkloadManager) initTicker(periodSeconds int64) {
	ticker := time.NewTicker(time.Second * time.Duration(periodSeconds))
	w.ticker = ticker
	go func() {
		for range ticker.C {
			err := w.ensureWorkloadsFromManifestsAreRunning()
			if err != nil {
				log.Error(err)
			}
		}
	}()
}

func (w *WorkloadManager) storeManifest(filePath string, podYaml []byte) error {
	return ioutil.WriteFile(filePath, podYaml, 0640)
}

func (w *WorkloadManager) getManifestPath(workloadName string) string {
	fileName := strings.ReplaceAll(workloadName, " ", "-") + ".yaml"
	return path.Join(w.manifestsDir, fileName)
}

func (w *WorkloadManager) toPodYaml(pod *v1.Pod) ([]byte, error) {
	podYaml, err := yaml.Marshal(pod)
	if err != nil {
		return nil, err
	}
	return podYaml, nil
}

func (w *WorkloadManager) ensureWorkloadsFromManifestsAreRunning() error {
	w.managementLock.Lock()
	defer w.managementLock.Unlock()
	nameToWorkload, err := w.indexWorkloads()
	if err != nil {
		return err
	}

	manifestInfo, err := ioutil.ReadDir(w.manifestsDir)
	if err != nil {
		return err
	}
	manifestNameToPodAndPath := make(map[string]podAndPath)
	for _, fi := range manifestInfo {
		filePath := path.Join(w.manifestsDir, fi.Name())
		manifest, err := ioutil.ReadFile(filePath)
		if err != nil {
			log.Error(err)
			continue
		}
		pod := v1.Pod{}
		err = yaml.Unmarshal(manifest, &pod)
		if err != nil {
			log.Error(err)
			continue
		}
		manifestNameToPodAndPath[pod.Name] = podAndPath{pod, filePath}
	}

	// Remove any workloads that don't correspond to stored manifests
	for name := range nameToWorkload {
		if _, ok := manifestNameToPodAndPath[name]; !ok {
			log.Infof("Workload not found: %s. Removing", name)
			if err := w.workloads.Remove(name); err != nil {
				log.Error(err)
			}
		}
	}

	for name, podWithPath := range manifestNameToPodAndPath {
		if workload, ok := nameToWorkload[name]; ok {
			if workload.Status != "Running" {
				// Workload is not running - start
				err = w.workloads.Start(&podWithPath.pod)
				if err != nil {
					log.Errorf("failed to start workload %s: %v", name, err)
				}
			}
			continue
		}
		// Workload is not present - run
		err = w.workloads.Run(&podWithPath.pod, podWithPath.manifestPath)
		if err != nil {
			log.Errorf("failed to run workload %s (manifest: %s): %v", name, podWithPath.manifestPath, err)
			continue
		}
	}
	if err = w.workloads.PersistConfiguration(); err != nil {
		log.Errorf("failed to persist workload configuration: %v", err)
	}
	return nil
}

func (w *WorkloadManager) indexWorkloads() (map[string]api2.WorkloadInfo, error) {
	workloads, err := w.workloads.List()
	if err != nil {
		return nil, err
	}
	nameToWorkload := make(map[string]api2.WorkloadInfo)
	for _, workload := range workloads {
		nameToWorkload[workload.Name] = workload
	}
	return nameToWorkload, nil
}

func (w *WorkloadManager) RegisterObserver(observer Observer) {
	w.workloads.RegisterObserver(observer)
}

func (w *WorkloadManager) toPod(workload *models.Workload) (*v1.Pod, error) {
	podSpec := v1.PodSpec{}
	err := yaml.Unmarshal([]byte(workload.Specification), &podSpec)
	if err != nil {
		return nil, err
	}
	pod := v1.Pod{
		Spec: podSpec,
	}
	pod.Kind = "Pod"
	pod.Name = workload.Name
	exportVolume := volumes.HostPathVolume(w.volumesDir, workload.Name)
	pod.Spec.Volumes = append(pod.Spec.Volumes, exportVolume)
	var containers []v1.Container
	for _, container := range pod.Spec.Containers {
		mount := v1.VolumeMount{
			Name:      exportVolume.Name,
			MountPath: "/export",
		}
		container.VolumeMounts = append(container.VolumeMounts, mount)
		containers = append(containers, container)
	}
	pod.Spec.Containers = containers
	return &pod, nil
}

func (w *WorkloadManager) podModified(manifestPath string, podYaml []byte) bool {
	file, err := ioutil.ReadFile(manifestPath)
	if err != nil {
		return true
	}
	return bytes.Compare(file, podYaml) != 0
}
