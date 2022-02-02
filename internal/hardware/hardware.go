package hardware

import (
	"github.com/project-flotta/flotta-operator/models"
	"github.com/openshift/assisted-installer-agent/src/inventory"
	"github.com/openshift/assisted-installer-agent/src/util"
)

type Hardware struct {
}

func (s *Hardware) GetHardwareInformation() (*models.HardwareInfo, error) {
	d := util.NewDependencies("/")
	cpu := inventory.GetCPU(d)
	hostname := inventory.GetHostname(d)
	systemVendor := inventory.GetVendor(d)
	interfaces := inventory.GetInterfaces(d)

	hardwareInfo := models.HardwareInfo{}

	hardwareInfo.CPU = &models.CPU{
		Architecture: cpu.Architecture,
		ModelName:    cpu.ModelName,
		Flags:        []string{},
	}
	hardwareInfo.Hostname = hostname
	hardwareInfo.SystemVendor = (*models.SystemVendor)(systemVendor)
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

	return &hardwareInfo, nil
}
