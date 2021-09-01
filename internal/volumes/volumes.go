package volumes

import (
	v1 "k8s.io/api/core/v1"
	"path"
)

func HostPathVolume(volumesDir string, workloadName string) v1.Volume {
	directoryOrCreate := v1.HostPathDirectoryOrCreate
	return v1.Volume{
		Name: "export-" + workloadName,
		VolumeSource: v1.VolumeSource{
			HostPath: &v1.HostPathVolumeSource{
				Type: &directoryOrCreate,
				Path: HostPathVolumePath(volumesDir, workloadName),
			},
		},
	}
}

func HostPathVolumePath(volumesDir string, workloadName string) string {
	return path.Join(volumesDir, workloadName)
}
