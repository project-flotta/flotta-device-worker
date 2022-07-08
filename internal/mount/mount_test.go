package mount_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"github.com/openshift/assisted-installer-agent/src/util"
	"github.com/project-flotta/flotta-device-worker/internal/common"
	mm "github.com/project-flotta/flotta-device-worker/internal/mount"
	"github.com/project-flotta/flotta-operator/models"
	"golang.org/x/sys/unix"
)

var _ = Describe("Mount", Ordered, Label("root"), func() {
	var (
		depMock        *util.MockIDependencies
		charDevicePath string
		devFolder      string
		tmpFolder      string
		err            error

		deviceFolder string
		loopDevices  []string

		mountManager *mm.Manager

		dep util.IDependencies
	)

	execCommand := func(cmd string) string {
		split := strings.Split(cmd, " ")
		command := exec.Command(split[0], split[1:]...)
		session, err := gexec.Start(command, new(bytes.Buffer), new(bytes.Buffer))
		Expect(err).ShouldNot(HaveOccurred())
		EventuallyWithOffset(1, session).Should(gexec.Exit(0), fmt.Sprintf("Command: %v failed with %s", cmd, session.Out.Contents()))
		return string(session.Out.Contents())
	}

	BeforeAll(func() {
		common.SkipIfRootless("Mount needs root access to run it")
		createFilesystemfile()
		deviceFolder, err = ioutil.TempDir("/tmp/", "lopdevices")
		Expect(err).NotTo(HaveOccurred())

		execCommand(fmt.Sprintf("dd if=/dev/zero of=/%s/image bs=1M count=128", deviceFolder))
		execCommand(fmt.Sprintf("mkfs.ext4 /%s/image", deviceFolder))

		deviceID := execCommand(fmt.Sprintf("losetup -f --show %s/image", deviceFolder))
		loopDevices = append(loopDevices, strings.Replace(deviceID, "\n", "", -1))

		secondDeviceID := execCommand(fmt.Sprintf("losetup -f --show %s/image", deviceFolder))
		loopDevices = append(loopDevices, strings.Replace(secondDeviceID, "\n", "", -1))
	})

	AfterAll(func() {
		for _, device := range loopDevices {
			execCommand(fmt.Sprintf("losetup -d %s", device))
		}
		Expect(os.RemoveAll(tmpFolder)).ShouldNot(HaveOccurred())
	})

	Context("Mount block device", func() {
		BeforeEach(func() {

			dep = util.NewDependencies("/")

			mountManager, err = mm.New()
			Expect(err).NotTo(HaveOccurred())

			tmpFolder, err = ioutil.TempDir("/tmp/", "mountfile")
			Expect(err).NotTo(HaveOccurred())

			depMock = &util.MockIDependencies{}

			devFolder, err = ioutil.TempDir("/tmp/", "dev")
			Expect(err).NotTo(HaveOccurred())

			Expect(os.MkdirAll(devFolder, os.ModePerm)).NotTo(HaveOccurred())

			charDevicePath = fmt.Sprintf("%s/chardevice", devFolder)
			Expect(unix.Mknod(charDevicePath, syscall.S_IFCHR|uint32(os.FileMode(0660)), int(unix.Mkdev(4, 1)))).
				NotTo(HaveOccurred())
		})

		AfterEach(func() {
			// We check if tmpFolder is mount and if so, we force unmount it
			_, mountMap, err := mm.GetMounts(dep)
			Expect(err).NotTo(HaveOccurred())
			if mountVal, found := mountMap[tmpFolder]; found {
				err = unix.Unmount(mountVal.Directory, unix.MNT_FORCE)
				Expect(err).NotTo(HaveOccurred())
			}

			depMock.AssertExpectations(GinkgoT())

			Expect(os.RemoveAll(tmpFolder)).NotTo(HaveOccurred())
		})

		It("mount block device without error", func() {
			// given
			mount := models.Mount{
				Device:    loopDevices[0],
				Directory: tmpFolder,
				Type:      "ext4",
			}

			configuration := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Mounts: []*models.Mount{&mount},
				},
			}

			// when
			Expect(mountManager.Update(configuration)).NotTo(HaveOccurred())

			// then
			_, mounts, err := mm.GetMounts(dep)
			Expect(err).NotTo(HaveOccurred())

			Expect(mounts).To(HaveKey(mount.Directory))
		})

		It("Don't try to mount char devices", func() {
			// given
			mount := models.Mount{
				Device:    charDevicePath,
				Directory: tmpFolder,
				Type:      "ext4",
			}

			configuration := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Mounts: []*models.Mount{&mount},
				},
			}

			// when
			Expect(mountManager.Update(configuration)).To(HaveOccurred())

			// then
			_, mounts, err := mm.GetMounts(dep)
			Expect(err).NotTo(HaveOccurred())

			// we expect not to found the mount here
			_, found := mounts[mount.Directory]
			Expect(found).To(BeFalse())
		})

		It("Unmount a device before mounting again", func() {
			// given
			mount := models.Mount{
				Device:    loopDevices[0],
				Directory: tmpFolder,
				Type:      "ext4",
			}

			configuration := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Mounts: []*models.Mount{&mount},
				},
			}

			// when
			Expect(mountManager.Update(configuration)).NotTo(HaveOccurred())

			// try to mount the other block device on the same folder
			mount = models.Mount{
				Device:    loopDevices[1],
				Directory: tmpFolder,
				Type:      "ext4",
			}

			// when
			Expect(mountManager.Update(configuration)).NotTo(HaveOccurred())

			// then
			_, mounts, err := mm.GetMounts(dep)
			Expect(err).NotTo(HaveOccurred())

			// we expect not to found the mount here

			Expect(mounts).To(HaveKey(mount.Directory))
			m := mounts[mount.Directory]
			Expect(m.Device).To(Equal(loopDevices[1]))
		})

		It("Cannot mount on a folder twice", func() {
			// given
			mounts := []*models.Mount{
				{
					Device:    loopDevices[0],
					Directory: tmpFolder,
					Type:      "ext4",
				},
				{
					Device:    loopDevices[1],
					Directory: tmpFolder,
					Type:      "ext4",
				},
			}

			configuration := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Mounts: mounts,
				},
			}

			// when
			Expect(mountManager.Update(configuration)).To(HaveOccurred())

			// then
			_, mountMap, err := mm.GetMounts(dep)
			Expect(err).NotTo(HaveOccurred())

			Expect(mountMap).To(HaveKey(tmpFolder))

			m, found := mountMap[tmpFolder]
			Expect(found).To(BeTrue())
			Expect(m.Device).To(Equal(loopDevices[0]))
		})

		It("Ignore non valid mount configuration", func() {
			// given
			mounts := []*models.Mount{
				{
					Device:    charDevicePath,
					Directory: tmpFolder,
					Type:      "ext4",
				},
				{
					Device:    loopDevices[0],
					Directory: tmpFolder,
					Type:      "ext4",
				},
			}

			configuration := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					Mounts: mounts,
				},
			}

			// when
			Expect(mountManager.Update(configuration)).To(HaveOccurred())

			// then
			_, mountMap, err := mm.GetMounts(dep)
			Expect(err).NotTo(HaveOccurred())
			Expect(mountMap).To(HaveKey(tmpFolder))

			// we expect not to found the mount here
			m, found := mountMap[tmpFolder]
			Expect(found).To(BeTrue())
			Expect(m.Device).To(Equal(loopDevices[0]))
		})
	})
})

func createFilesystemfile() {
	//GH actions does not have this file at all.
	path := "/etc/filesystems"
	_, err := os.Stat(path)
	if err == nil {
		return
	}
	if !errors.Is(err, os.ErrNotExist) {
		return
	}
	content := `
ext4
ext3
ext2
nodev proc
nodev devpts
iso9660
vfat
hfs
hfsplus
*
`
	err = os.WriteFile(path, []byte(content), 0666)
	Expect(err).NotTo(HaveOccurred())
}
