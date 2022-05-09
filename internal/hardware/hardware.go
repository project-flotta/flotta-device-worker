package hardware

import (
	"reflect"

	"github.com/openshift/assisted-installer-agent/src/inventory"
	"github.com/openshift/assisted-installer-agent/src/util"
	"github.com/project-flotta/flotta-operator/models"
)

//go:generate mockgen -package=hardware -destination=mock_hardware.go . Hardware
type Hardware interface {
	GetHardwareInformation() (*models.HardwareInfo, error)
	GetHardwareImmutableInformation(hardwareInfo *models.HardwareInfo) error
	CreateHardwareMutableInformation() *models.HardwareInfo
	GetMutableHardwareInfoDelta(hardwareMutableInfoPrevious models.HardwareInfo, hardwareMutableInfoNew models.HardwareInfo) *models.HardwareInfo
}

type HardwareInfo struct {
	Dependencies util.IDependencies
}

func (s *HardwareInfo) GetHardwareInformation() (*models.HardwareInfo, error) {
	s.init()
	hardwareInfo := models.HardwareInfo{}
	err := s.GetHardwareImmutableInformation(&hardwareInfo)
	if err != nil {
		return nil, err
	}
	s.getHardwareMutableInformation(&hardwareInfo)

	return &hardwareInfo, nil
}

func (s *HardwareInfo) GetHardwareImmutableInformation(hardwareInfo *models.HardwareInfo) error {
	s.init()
	cpu := inventory.GetCPU(s.Dependencies)
	systemVendor := inventory.GetVendor(s.Dependencies)

	hardwareInfo.CPU = &models.CPU{
		Architecture: cpu.Architecture,
		ModelName:    cpu.ModelName,
		Flags:        []string{},
	}
	hardwareInfo.SystemVendor = (*models.SystemVendor)(systemVendor)

	return nil
}

func (s *HardwareInfo) CreateHardwareMutableInformation() *models.HardwareInfo {
	hardwareInfo := models.HardwareInfo{}
	s.getHardwareMutableInformation(&hardwareInfo)
	return &hardwareInfo
}

func (s *HardwareInfo) getHardwareMutableInformation(hardwareInfo *models.HardwareInfo) {
	s.init()
	hostname := inventory.GetHostname(s.Dependencies)
	interfaces := inventory.GetInterfaces(s.Dependencies)

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
}

func (s *HardwareInfo) init() {
	if s.Dependencies == nil {
		s.Dependencies = util.NewDependencies("/")
	}
}

func (s *HardwareInfo) GetMutableHardwareInfoDelta(hardwareMutableInfoPrevious models.HardwareInfo, hardwareMutableInfoNew models.HardwareInfo) *models.HardwareInfo {
	return GetMutableHardwareInfoDelta(hardwareMutableInfoPrevious, hardwareMutableInfoNew)
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
