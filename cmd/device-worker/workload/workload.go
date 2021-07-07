package workload

import (
	"context"
	"fmt"
	"git.sr.ht/~spc/go-log"
	"github.com/containers/podman/v2/pkg/bindings"
	"github.com/containers/podman/v2/pkg/bindings/play"
	"github.com/containers/podman/v2/pkg/bindings/pods"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/jakub-dzon/k4e-operator/models"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	"os"
	"path"
	"sigs.k8s.io/yaml"
	"strings"
)

type Workload struct {
	manifestsDir string
}

func NewWorkload(configDir string) (*Workload, error) {
	manifestsDir := path.Join(configDir, "manifests")
	if err := os.MkdirAll(manifestsDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create directory: %w", err)
	}
	return &Workload{
		manifestsDir: manifestsDir,
	}, nil
}

func (w *Workload) Update(configuration models.DeviceConfigurationMessage) error {
	workloads := configuration.Workloads
	if len(workloads) == 0 {
		log.Trace("No workloads")

		// Stop all the workloads
		podmanConnection, err := bindings.NewConnection(context.Background(), "unix://run/podman/podman.sock")
		if err != nil {
			log.Errorf("Cannot connect to podman: %v", err)
			return err
		}
		podList, err := pods.List(podmanConnection, map[string][]string{})
		if err != nil {
			log.Errorf("Cannot list pods: %v", err)
			return err
		}
		for _, podReport := range podList {
			err := w.removePod(podmanConnection, podReport.Name)
			if err != nil {
				log.Errorf("Error removing pod: %v", err)
				return err
			}
		}
		// Remove manifests
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

	for _, workload := range workloads {
		podName := workload.Name
		log.Tracef("Deploying workload: %s", podName)
		// TODO: change error handling from fail fast to best effort (deploy as many workloads as possible)
		manifestPath, err := w.storeManifest(workload)
		if err != nil {
			return err
		}
		podmanConnection, err := bindings.NewConnection(context.Background(), "unix://run/podman/podman.sock")
		if err != nil {
			log.Errorf("Cannot connect to podman: %v", err)
			return err
		}
		err = w.removePod(podmanConnection, podName)
		if err != nil {
			log.Errorf("Error removing pod: %v", err)
			return err
		}
		report, err := play.Kube(podmanConnection, manifestPath, entities.PlayKubeOptions{})
		if err != nil {
			log.Errorf("Cannot execute Podman Play Kube: %v", err)
			return err
		}
		log.Infof("Pod report: %v", report)
	}
	return nil
}

func (w *Workload) removePod(podmanConnection context.Context, podName string) error {
	exists, err := pods.Exists(podmanConnection, podName)
	if err != nil {
		log.Errorf("Cannot check pod existence: %v", err)
		return err
	}
	if exists {
		log.Infof("Pod %s exists. Removing.", podName)
		force := true
		_, err := pods.Remove(podmanConnection, podName, &force)
		if err != nil {
			log.Errorf("Cannot remove pod: %v", err)
		}
	}
	return nil
}

func (w *Workload) storeManifest(workload *models.Workload) (string, error) {
	podYaml, err := w.toPodYaml(workload)
	if err != nil {
		return "", err
	}
	fileName := strings.ReplaceAll(workload.Name, " ", "-") + ".yaml"
	filePath := path.Join(w.manifestsDir, fileName)
	err = ioutil.WriteFile(filePath, podYaml, 0640)
	if err != nil {
		return "", err
	}
	return filePath, nil
}

func (w *Workload) toPodYaml(workload *models.Workload) ([]byte, error) {
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
	return yaml.Marshal(pod)
}
