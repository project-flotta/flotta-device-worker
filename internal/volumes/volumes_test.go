package volumes_test

import (
	"github.com/project-flotta/flotta-device-worker/internal/volumes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("Volumes", func() {
	It("Should generate HostPath Volume", func() {
		// given
		var (
			volumesDir   = "/some/path"
			workloadName = "a-workload"
		)

		// when
		volume := volumes.HostPathVolume(volumesDir, workloadName)

		// then
		Expect(volume).ToNot(BeNil())
		Expect(volume.Name).To(ContainSubstring(workloadName))
		Expect(volume.VolumeSource.HostPath).ToNot(BeNil())
		Expect(volume.VolumeSource.HostPath.Type).ToNot(BeNil())
		Expect(*volume.VolumeSource.HostPath.Type).To(BeEquivalentTo(v1.HostPathDirectoryOrCreate))
		Expect(volume.VolumeSource.HostPath.Path).To(BeEquivalentTo(volumesDir + "/" + workloadName))

	})
})
