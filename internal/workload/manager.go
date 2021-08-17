package workload

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"git.sr.ht/~spc/go-log"
	api2 "github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
	"github.com/jakub-dzon/k4e-operator/models"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

type WorkloadManager struct {
	manifestsDir string
	workloads    *workloadWrapper
}

type podAndPath struct {
	pod          v1.Pod
	manifestPath string
}

func NewWorkloadManager(configDir string, podmanSocketName string) (*WorkloadManager, error) {
	manifestsDir := path.Join(configDir, "manifests")
	if err := os.MkdirAll(manifestsDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create directory: %w", err)
	}
	wrapper, err := newWorkloadWrapper(configDir, podmanSocketName)
	if err != nil {
		return nil, err
	}
	manager := WorkloadManager{
		manifestsDir: manifestsDir,
		workloads:    wrapper,
	}
	if err := manager.workloads.Init(); err != nil {
		return nil, err
	}
	go func() {
		for {
			err := manager.ensureWorkloadsFromManifestsAreRunning()
			if err != nil {
				log.Error(err)
			}
			time.Sleep(time.Second * 15)
		}
	}()

	return &manager, nil
}

func (w *WorkloadManager) ListWorkloads() ([]api2.WorkloadInfo, error) {
	return w.workloads.List()
}

func (w *WorkloadManager) Update(configuration models.DeviceConfigurationMessage) error {
	workloads := configuration.Workloads
	if len(workloads) == 0 {
		log.Trace("No workloads")

		// Purge all the workloads
		err := w.purgeWorkloads()
		if err != nil {
			return err
		}
		// Remove manifests
		err = w.removeManifests()
		if err != nil {
			return err
		}
		return nil
	}

	for _, workload := range workloads {
		log.Tracef("Deploying workload: %s", workload.Name)
		// TODO: change error handling from fail fast to best effort (deploy as many workloads as possible)
		pod, err := w.toPod(workload)
		if err != nil {
			return err
		}
		manifestPath, err := w.storeManifest(pod)
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
	return nil
}

func (w *WorkloadManager) purgeWorkloads() error {
	podList, err := w.workloads.List()
	if err != nil {
		log.Errorf("Cannot list workloads: %v", err)
		return err
	}
	for _, podReport := range podList {
		err := w.workloads.Remove(podReport.Name)
		if err != nil {
			log.Errorf("Error removing workload: %v", err)
			return err
		}
	}
	return nil
}

func (w *WorkloadManager) removeManifests() error {
	manifestInfo, err := ioutil.ReadDir(w.manifestsDir)
	if err != nil {
		return err
	}
	for _, fi := range manifestInfo {
		filePath := path.Join(w.manifestsDir, fi.Name())
		err := os.Remove(filePath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *WorkloadManager) storeManifest(pod *v1.Pod) (string, error) {
	podYaml, err := yaml.Marshal(pod)
	if err != nil {
		return "", err
	}
	fileName := strings.ReplaceAll(pod.Name, " ", "-") + ".yaml"
	filePath := path.Join(w.manifestsDir, fileName)
	err = ioutil.WriteFile(filePath, podYaml, 0640)
	if err != nil {
		return "", err
	}
	return filePath, nil
}

func (w *WorkloadManager) ensureWorkloadsFromManifestsAreRunning() error {
	manifestInfo, err := ioutil.ReadDir(w.manifestsDir)
	if err != nil {
		return err
	}
	workloads, err := w.workloads.List()
	if err != nil {
		return err
	}
	nameToWorkload := make(map[string]api2.WorkloadInfo)
	for _, workload := range workloads {
		nameToWorkload[workload.Name] = workload
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
	return &pod, nil
}
