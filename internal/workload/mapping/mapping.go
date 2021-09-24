package mapping

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"sync"
)

type mapping struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type MappingRepository struct {
	mappingFilePath string
	idToName        map[string]string
	nameToId        map[string]string
	lock            sync.RWMutex
}

func NewMappingRepository(configDir string) (*MappingRepository, error) {
	mappingFilePath := path.Join(configDir, "workload-mapping.json")

	mappingJson, err := ioutil.ReadFile(mappingFilePath)
	var mappings []mapping
	if err == nil {
		err := json.Unmarshal(mappingJson, &mappings)
		if err != nil {
			return nil, err
		}
	}

	idToName := make(map[string]string)
	nameToId := make(map[string]string)
	for _, mapping := range mappings {
		idToName[mapping.Id] = mapping.Name
		nameToId[mapping.Name] = mapping.Id
	}

	return &MappingRepository{
		mappingFilePath: mappingFilePath,
		lock:            sync.RWMutex{},
		idToName:        idToName,
		nameToId:        nameToId,
	}, nil
}

func (m *MappingRepository) Add(name, id string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.idToName[id] = name
	m.nameToId[name] = id

	return m.persist()
}

func (m *MappingRepository) Remove(name string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	id := m.nameToId[name]
	delete(m.idToName, id)
	delete(m.nameToId, name)

	return m.persist()
}

func (m *MappingRepository) GetId(name string) string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.nameToId[name]
}

func (m *MappingRepository) GetName(id string) string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.idToName[id]
}

func (m *MappingRepository) Persist() error {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.persist()
}

func (m *MappingRepository) Size() int {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return len(m.nameToId)
}

func (m *MappingRepository) persist() error {
	var mappings []mapping
	for id, name := range m.idToName {
		mappings = append(mappings, mapping{Id: id, Name: name})
	}
	mappingsJson, err := json.Marshal(mappings)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(m.mappingFilePath, mappingsJson, 0640)
	if err != nil {
		return err
	}
	return nil
}
