package mapping

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type mapping struct {
	ModTime  int64  `json:"mod_time"` //unix nano
	FilePath string `json:"file_path"`
}

//go:generate mockgen -package=mapping -destination=mock_mapping.go . MappingRepository
type MappingRepository interface {
	GetSha256(fileContent []byte) string
	Add(fileContent []byte, modTime time.Time) error
	Remove(fileContent []byte) error
	RemoveMappingFile() error
	GetModTime(filePath string) int64
	GetFilePath(modTime time.Time) string
	Persist() error
	Size() int
	Exists(modTime time.Time) bool
	GetAll() map[int]string
}

type mappingRepository struct {
	mappingFilePath string
	modTimeToPath   map[int64]string
	pathToModTime   map[string]int64
	lock            sync.RWMutex
	configDir       string
}

func NewMappingRepository(configDir string) (MappingRepository, error) {

	mappingFilePath := path.Join(configDir, "playbook-mapping.json")

	mappingJSON, err := ioutil.ReadFile(mappingFilePath) //#nosec
	var mappings []mapping
	if err == nil {
		err := json.Unmarshal(mappingJSON, &mappings)
		if err != nil {
			return nil, err
		}
	}

	modTimeToPath := make(map[int64]string)
	pathToModTime := make(map[string]int64)
	for _, mapping := range mappings {
		modTimeToPath[mapping.ModTime] = mapping.FilePath
		pathToModTime[mapping.FilePath] = mapping.ModTime
	}

	return &mappingRepository{
		mappingFilePath: mappingFilePath,
		lock:            sync.RWMutex{},
		modTimeToPath:   modTimeToPath,
		pathToModTime:   pathToModTime,
		configDir:       configDir,
	}, nil
}

func (m *mappingRepository) GetAll() map[int]string {
	all := make(map[int]string)
	keys := make([]int64, 0, len(m.modTimeToPath))
	for k := range m.modTimeToPath {
		keys = append(keys, k)
	}

	sort.SliceStable(keys, func(i, j int) bool { return keys[i] < keys[j] })

	for i, v := range keys {
		all[i] = m.modTimeToPath[v]
	}

	return all
}

func (m *mappingRepository) GetSha256(fileContent []byte) string {
	h := sha256.New()
	h.Write(fileContent)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (m *mappingRepository) Add(fileContent []byte, modTime time.Time) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	filePath := path.Join(m.configDir, m.GetSha256(fileContent))
	err := os.WriteFile(filePath, []byte(fileContent), 0600)

	if err != nil {
		return err
	}

	m.modTimeToPath[modTime.UnixNano()] = filePath
	m.pathToModTime[filePath] = modTime.UnixNano()

	return m.persist()
}

func (m *mappingRepository) Remove(fileContent []byte) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	filePath := path.Join(m.configDir, m.GetSha256(fileContent))
	modTime := m.pathToModTime[filePath]
	delete(m.modTimeToPath, modTime)
	delete(m.pathToModTime, filePath)

	return m.persist()
}

func (m *mappingRepository) RemoveMappingFile() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	log.Infof("deleting %s file", m.mappingFilePath)
	err := os.RemoveAll(m.mappingFilePath)
	if err != nil {
		log.Errorf("failed to delete %s: %v", m.mappingFilePath, err)
		return err
	}

	return nil
}

func (m *mappingRepository) GetModTime(filePath string) int64 {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.pathToModTime[filePath]
}

func (m *mappingRepository) GetFilePath(modTime time.Time) string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.modTimeToPath[modTime.UnixNano()]
}

func (m *mappingRepository) Exists(modTime time.Time) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()

	_, x := m.modTimeToPath[modTime.UnixNano()]
	return x
}

func (m *mappingRepository) Persist() error {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return m.persist()
}

func (m *mappingRepository) Size() int {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return len(m.pathToModTime)
}

func (m *mappingRepository) persist() error {
	var mappings []mapping

	for modTime, path := range m.modTimeToPath {
		mappings = append(mappings, mapping{ModTime: modTime, FilePath: path})
	}
	mappingsJSON, err := json.Marshal(mappings)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(m.mappingFilePath, mappingsJSON, 0640) //#nosec
	if err != nil {
		return err
	}
	return nil
}
