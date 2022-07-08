package hardware

import (
	"errors"
	"fmt"
	"reflect"

	"git.sr.ht/~spc/go-log"
	runc "github.com/opencontainers/runc/libcontainer/devices"
	"github.com/openshift/assisted-installer-agent/src/inventory"
	"github.com/openshift/assisted-installer-agent/src/util"
	"github.com/project-flotta/flotta-device-worker/internal/mount"
	"github.com/project-flotta/flotta-operator/models"
)

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

	mounts, _, err := mount.GetMounts(s.dependencies)
	if err != nil {
		return fmt.Errorf("cannot list mounts: %s", err)
	}

	hardwareInfo.Mounts = mounts

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
	runcDevices, err := runc.HostDevices()
	if err != nil {
		return []*models.HostDevice{}, err
	}

	devices := make([]*models.HostDevice, 0, len(runcDevices))
	for _, d := range runcDevices {
		device := models.HostDevice{
			Path:       d.Path,
			DeviceType: string(d.Type),
			Major:      d.Major,
			Minor:      d.Minor,
			UID:        int64(d.Uid),
			Gid:        int64(d.Gid),
		}

		devices = append(devices, &device)
	}

	return devices, nil
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
