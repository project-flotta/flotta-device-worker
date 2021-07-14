package hardware

import (
	"encoding/json"
	"git.sr.ht/~spc/go-log"
	"github.com/jakub-dzon/k4e-operator/models"
	"github.com/openshift/assisted-installer-agent/src/inventory"
)

type Hardware struct {
}

func (s *Hardware) GetHardwareInformation() (*models.HardwareInfo, error) {
	inv := inventory.ReadInventory()

	// Instead of copying filed-by-filed marshal to JSON and unmarshal to other struct
	inventoryJson, err := json.Marshal(inv)
	if err != nil {
		return nil, err
	}
	hardwareInfo := models.HardwareInfo{}
	err = json.Unmarshal(inventoryJson, &hardwareInfo)
	if err != nil {
		return nil, err
	}
	log.Infof("Hardware info: %s", inventoryJson)
	return &hardwareInfo, nil
}
