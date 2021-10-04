package hardware

import (
	"encoding/json"
	"github.com/jakub-dzon/k4e-operator/models"
	"github.com/openshift/assisted-installer-agent/src/inventory"
)

type Hardware struct {
}

func (s *Hardware) GetHardwareInformation() (*models.HardwareInfo, error) {
	inv := inventory.ReadInventory()

	// Instead of copying field-by-field marshal to JSON and unmarshal to other struct
	inventoryJson, err := json.Marshal(inv)
	if err != nil {
		return nil, err
	}
	hardwareInfo := models.HardwareInfo{}
	err = json.Unmarshal(inventoryJson, &hardwareInfo)
	if err != nil {
		return nil, err
	}

	return &hardwareInfo, nil
}
