package mount_test

import (
	"fmt"
	"os"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/assisted-installer-agent/src/util"
	mm "github.com/project-flotta/flotta-device-worker/internal/mount"
	"github.com/project-flotta/flotta-operator/models"
	"golang.org/x/sys/unix"
)

var _ = Describe("Mount", func() {
	var (
		depMock          *util.MockIDependencies
		blockDevicePath  string
		otherBlockDevice string
		charDevicePath   string
		devFolder        string
	)

	Context("Mount block device", func() {
		BeforeEach(func() {
			// SkipIfRootless("need root permission")
			depMock = &util.MockIDependencies{}
			devFolder = "/dev/foodev"
			Expect(os.MkdirAll(devFolder, os.ModePerm)).To(BeNil())

			blockDevicePath = fmt.Sprintf("%s/blockdevice", devFolder)
			Expect(unix.Mknod(blockDevicePath, syscall.S_IFBLK|uint32(os.FileMode(0660)), int(unix.Mkdev(7, 0)))).To(BeNil())

			otherBlockDevice = fmt.Sprintf("%s/otherblockdevice", devFolder)
			Expect(unix.Mknod(otherBlockDevice, syscall.S_IFBLK|uint32(os.FileMode(0660)), int(unix.Mkdev(7, 0)))).To(BeNil())

			charDevicePath = fmt.Sprintf("%s/chardevice", devFolder)
			Expect(unix.Mknod(charDevicePath, syscall.S_IFCHR|uint32(os.FileMode(0660)), int(unix.Mkdev(3, 1)))).To(BeNil())
		})
		AfterEach(func() {
			depMock.AssertExpectations(GinkgoT())

			Expect(os.RemoveAll(devFolder)).To(BeNil())
		})

		It("mount block device without error", func() {
			// given
			dep := util.NewDependencies("/")
			mount := models.Mount{
				Device:    blockDevicePath,
				Directory: "/mnt",
				Type:      "ext4",
			}

			configuration := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Mounts: []*models.Mount{&mount},
				},
			}

			mountManager, err := mm.New()
			Expect(err).To(BeNil())

			// when
			Expect(mountManager.Update(configuration)).To(BeNil())
			defer func() {
				Expect(unix.Unmount(mount.Device, 0)).To(BeNil())
			}()

			// then
			_, mounts, err := mm.GetMounts(dep)
			Expect(err).To(BeNil())

			_, found := mounts[mount.Directory]
			Expect(found).To(BeTrue())
		})

		It("Don't try to mount char devices", func() {
			// given
			dep := util.NewDependencies("/")
			mount := models.Mount{
				Device:    charDevicePath,
				Directory: "/mnt",
				Type:      "ext4",
			}

			configuration := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Mounts: []*models.Mount{&mount},
				},
			}

			mountManager, err := mm.New()
			Expect(err).To(BeNil())

			// when
			Expect(mountManager.Update(configuration)).To(BeNil())

			// then
			_, mounts, err := mm.GetMounts(dep)
			Expect(err).To(BeNil())

			// we expect not to found the mount here
			_, found := mounts[mount.Directory]
			Expect(found).To(BeFalse())
		})

		It("Unmount a device before mounting again", func() {
			// given
			dep := util.NewDependencies("/")
			mount := models.Mount{
				Device:    blockDevicePath,
				Directory: "/mnt",
				Type:      "ext4",
			}

			configuration := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Mounts: []*models.Mount{&mount},
				},
			}

			mountManager, err := mm.New()
			Expect(err).To(BeNil())

			// when
			Expect(mountManager.Update(configuration)).To(BeNil())

			// try to mount the other block device on the same folder
			mount = models.Mount{
				Device:    otherBlockDevice,
				Directory: "/mnt",
				Type:      "ext4",
			}

			// when
			Expect(mountManager.Update(configuration)).To(BeNil())

			// then
			_, mounts, err := mm.GetMounts(dep)
			Expect(err).To(BeNil())

			// we expect not to found the mount here
			m, found := mounts[mount.Directory]
			Expect(found).To(BeFalse())
			Expect(m.Device).To(Equal(otherBlockDevice))
		})

		It("Cannot mount on a folder twice", func() {
			// given
			dep := util.NewDependencies("/")
			mounts := []*models.Mount{
				{
					Device:    blockDevicePath,
					Directory: "/mnt",
					Type:      "ext4",
				},
				{
					Device:    otherBlockDevice,
					Directory: "/mnt",
					Type:      "ext4",
				},
			}

			configuration := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Mounts: mounts,
				},
			}

			mountManager, err := mm.New()
			Expect(err).To(BeNil())

			// when
			Expect(mountManager.Update(configuration)).To(BeNil())

			// then
			_, mountMap, err := mm.GetMounts(dep)
			Expect(err).To(BeNil())

			// we expect not to found the mount here
			m, found := mountMap["/mnt"]
			Expect(found).To(BeFalse())
			Expect(m.Device).To(Equal(blockDevicePath))
		})

		It("Ignore non valid mount configuration", func() {
			// given
			dep := util.NewDependencies("/")
			mounts := []*models.Mount{
				{
					Device:    charDevicePath,
					Directory: "/mnt",
					Type:      "ext4",
				},
				{
					Device:    otherBlockDevice,
					Directory: "/mnt",
					Type:      "ext4",
				},
			}

			configuration := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Mounts: mounts,
				},
			}

			mountManager, err := mm.New()
			Expect(err).To(BeNil())

			// when
			Expect(mountManager.Update(configuration)).To(BeNil())

			// then
			_, mountMap, err := mm.GetMounts(dep)
			Expect(err).To(BeNil())

			// we expect not to found the mount here
			m, found := mountMap["/mnt"]
			Expect(found).To(BeFalse())
			Expect(m.Device).To(Equal(otherBlockDevice))
		})
	})
})
