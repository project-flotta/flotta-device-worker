package hardware

import (
	"reflect"

	"github.com/openshift/assisted-installer-agent/src/inventory"
	"github.com/openshift/assisted-installer-agent/src/util"
	"github.com/project-flotta/flotta-operator/models"
)

type Hardware struct {
	Dependencies util.IDependencies
}

func (s *Hardware) GetHardwareInformation() (*models.HardwareInfo, error) {
	s.Init()
	hardwareInfo := models.HardwareInfo{}
	err := s.GetHardwareImmutableInformation(&hardwareInfo)
	if err != nil {
		return nil, err
	}
	s.getHardwareMutableInformation(&hardwareInfo)

	return &hardwareInfo, nil
}

func (s *Hardware) GetHardwareImmutableInformation(hardwareInfo *models.HardwareInfo) error {
	s.Init()
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

func (s *Hardware) CreateHardwareMutableInformation() *models.HardwareInfo {
	hardwareInfo := models.HardwareInfo{}
	s.getHardwareMutableInformation(&hardwareInfo)
	return &hardwareInfo
}

func (s *Hardware) getHardwareMutableInformation(hardwareInfo *models.HardwareInfo) {
	s.Init()
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

func (s *Hardware) Init() {
	if s.Dependencies == nil {
		s.Dependencies = util.NewDependencies("/")
	}
}

func GetMutableHardwareInfoDelta(hardwareMutableInfoSource models.HardwareInfo, hardwareMutableInfoTarget models.HardwareInfo) *models.HardwareInfo {
	hardwareInfo := &models.HardwareInfo{}
	if hardwareMutableInfoSource.Hostname != hardwareMutableInfoTarget.Hostname {
		hardwareInfo.Hostname = hardwareMutableInfoTarget.Hostname
	}
	if !reflect.DeepEqual(hardwareMutableInfoSource.Interfaces, hardwareMutableInfoTarget.Interfaces) {
		hardwareInfo.Interfaces = hardwareMutableInfoTarget.Interfaces
	}

	return hardwareInfo
}
