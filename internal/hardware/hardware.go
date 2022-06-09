package hardware

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"

	"git.sr.ht/~spc/go-log"
	"github.com/openshift/assisted-installer-agent/src/inventory"
	"github.com/openshift/assisted-installer-agent/src/util"
	"github.com/project-flotta/flotta-operator/models"
	"golang.org/x/sys/unix"
)

// errNotADevice means that the file is not a device node.
var errNotADevice = errors.New("not a device node")

//go:generate mockgen -package=hardware -destination=mock_hardware.go . Hardware
type Hardware interface {
	GetHardwareInformation() (*models.HardwareInfo, error)
	GetHardwareImmutableInformation(hardwareInfo *models.HardwareInfo) error
	CreateHardwareMutableInformation() (*models.HardwareInfo, error)
	GetMutableHardwareInfoDelta(hardwareMutableInfoPrevious models.HardwareInfo, hardwareMutableInfoNew models.HardwareInfo) *models.HardwareInfo
}

type HardwareInfo struct {
	dependencies util.IDependencies
}

func (s *HardwareInfo) GetHardwareInformation() (*models.HardwareInfo, error) {
	hardwareInfo := models.HardwareInfo{}
	err := s.GetHardwareImmutableInformation(&hardwareInfo)
	if err != nil {
		return nil, err
	}
	err = s.getHardwareMutableInformation(&hardwareInfo)

	return &hardwareInfo, err
}

func (s *HardwareInfo) GetHardwareImmutableInformation(hardwareInfo *models.HardwareInfo) error {
	if !s.isDependenciesSet() {
		return errors.New("HardwareInfo object has not been initialized")
	}
	cpu := inventory.GetCPU(s.dependencies)
	systemVendor := inventory.GetVendor(s.dependencies)

	hardwareInfo.CPU = &models.CPU{
		Architecture: cpu.Architecture,
		ModelName:    cpu.ModelName,
		Flags:        []string{},
	}
	hardwareInfo.SystemVendor = (*models.SystemVendor)(systemVendor)

	if hostDevices, err := s.getHostDevices(); err != nil {
		log.Warnf("failed to list host devices: %v", err)
	} else {
		hardwareInfo.HostDevices = hostDevices
	}

	return nil
}

func (s *HardwareInfo) CreateHardwareMutableInformation() (*models.HardwareInfo, error) {
	hardwareInfo := models.HardwareInfo{}
	err := s.getHardwareMutableInformation(&hardwareInfo)
	if err != nil {
		return nil, err
	}
	return &hardwareInfo, nil
}

func (s *HardwareInfo) getHardwareMutableInformation(hardwareInfo *models.HardwareInfo) error {
	if !s.isDependenciesSet() {
		return errors.New("HardwareInfo object has not been initialized")
	}
	hostname := inventory.GetHostname(s.dependencies)
	interfaces := inventory.GetInterfaces(s.dependencies)

	hardwareInfo.Hostname = hostname
	for _, currInterface := range interfaces {
		if len(currInterface.IPV4Addresses) == 0 && len(currInterface.IPV6Addresses) == 0 {
			continue
		}
		newInterface := &models.Interface{
			IPV4Addresses: currInterface.IPV4Addresses,
			IPV6Addresses: currInterface.IPV6Addresses,
			Flags:         []string{},
		}
		hardwareInfo.Interfaces = append(hardwareInfo.Interfaces, newInterface)
	}

	return nil
}

func (s *HardwareInfo) Init(dep util.IDependencies) {
	if dep == nil {
		s.dependencies = util.NewDependencies("/")
	} else {
		s.dependencies = dep
	}
}

func (s *HardwareInfo) isDependenciesSet() bool {
	return s.dependencies != nil
}

func (s *HardwareInfo) GetMutableHardwareInfoDelta(hardwareMutableInfoPrevious models.HardwareInfo, hardwareMutableInfoNew models.HardwareInfo) *models.HardwareInfo {
	return GetMutableHardwareInfoDelta(hardwareMutableInfoPrevious, hardwareMutableInfoNew)
}

func (s *HardwareInfo) getHostDevices() ([]*models.HostDevice, error) {
	return getDevices("/dev")
}

func GetMutableHardwareInfoDelta(hardwareMutableInfoPrevious models.HardwareInfo, hardwareMutableInfoNew models.HardwareInfo) *models.HardwareInfo {
	hardwareInfo := &models.HardwareInfo{}
	if hardwareMutableInfoPrevious.Hostname != hardwareMutableInfoNew.Hostname {
		hardwareInfo.Hostname = hardwareMutableInfoNew.Hostname
	}
	if !reflect.DeepEqual(hardwareMutableInfoPrevious.Interfaces, hardwareMutableInfoNew.Interfaces) {
		hardwareInfo.Interfaces = hardwareMutableInfoNew.Interfaces
	}

	return hardwareInfo
}

// GetDevices recursively traverses a directory specified by path
// and returns all devices found there.
// Shamelessly copied from 	https://github.com/opencontainers/runc/blob/main/libcontainer/devices/device_unix.go
func getDevices(path string) ([]*models.HostDevice, error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var out []*models.HostDevice
	for _, f := range files {
		switch {
		case f.IsDir():
			switch f.Name() {
			// ".lxc" & ".lxd-mounts" added to address https://github.com/lxc/lxd/issues/2825
			// ".udev" added to address https://github.com/opencontainers/runc/issues/2093
			case "pts", "shm", "fd", "mqueue", ".lxc", ".lxd-mounts", ".udev":
				continue
			default:
				sub, err := getDevices(filepath.Join(path, f.Name()))
				if err != nil {
					return nil, err
				}

				out = append(out, sub...)
				continue
			}
		case f.Name() == "console":
			continue
		}
		device, err := deviceFromPath(filepath.Join(path, f.Name()))
		if err != nil {
			if errors.Is(err, errNotADevice) {
				continue
			}
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if device.DeviceType == "FifoDevice" {
			continue
		}
		out = append(out, &device)
	}
	return out, nil
}

func deviceFromPath(path string) (models.HostDevice, error) {
	var (
		stat   unix.Stat_t
		device models.HostDevice
	)

	err := unix.Lstat(path, &stat)
	if err != nil {
		return models.HostDevice{}, err
	}

	devNumber := uint64(stat.Rdev)

	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFBLK:
		device.DeviceType = "BlockDevice"
	case unix.S_IFCHR:
		device.DeviceType = "CharacterDevice"
	case unix.S_IFIFO:
		device.DeviceType = "FifoDevice"
	default:
		return models.HostDevice{}, errNotADevice
	}

	device.Major = int64(unix.Major(devNumber))
	device.Minor = int64(unix.Minor(devNumber))
	device.Path = path
	device.UID = int64(stat.Uid)
	device.Gid = int64(stat.Gid)

	return device, nil
}
