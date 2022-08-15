package mount

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"sync"

	"git.sr.ht/~spc/go-log"
	"github.com/hashicorp/go-multierror"
	"github.com/openshift/assisted-installer-agent/src/util"
	"github.com/project-flotta/flotta-operator/models"
	"golang.org/x/sys/unix"
)

const (
	filesystemsFile = "/etc/filesystems"
	flottaGroup     = "flotta"
	flottaUser      = "flotta"
)

type Manager struct {
	lock sync.Mutex
	dep  util.IDependencies

	// filesystems holds the content of /etc/filesystems
	// it used to validate the mount type
	filesystems    string
	defaultOptions string
}

func New() (*Manager, error) {
	content, err := os.ReadFile(filesystemsFile)
	if err != nil {
		log.Warnf("Cannot list content of '%s': %s", filesystemsFile, err)
		return nil, fmt.Errorf("cannot list content of '%s': %s", filesystemsFile, err)
	}
	return &Manager{
		filesystems:    string(content),
		dep:            util.NewDependencies("/"),
		defaultOptions: getDefaultMountOptions(),
	}, nil
}

func (m *Manager) Init(config models.DeviceConfigurationMessage) error {
	return m.Update(config)
}

func (m *Manager) Update(config models.DeviceConfigurationMessage) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	_, currentMounts, err := GetMounts(m.dep)
	if err != nil {
		return err
	}

	// holds the directories mounted in the current _Update_ call.
	// It helps to avoid mounting the same directory twice.
	alreadyMounted := make(map[string]struct{})
	var merr error
	for _, mm := range config.Configuration.Mounts {
		if _, found := alreadyMounted[mm.Directory]; found {
			errf := fmt.Errorf("mount path '%s' has already been mounted. Skipping mount path", mm.Directory)
			merr = multierror.Append(merr, errf)
			log.Info(errf)
			continue
		}

		if err := m.isValid(mm); err != nil {
			errf := fmt.Errorf("mount configuration '%+v' not valid: %s", mm, err)
			merr = multierror.Append(merr, errf)
			log.Warn(errf)
			continue
		}

		// if the directory is mounted (not in the current call) and the configuration is different
		// try to force umount it first and then mount it with the new configuration.
		if c, found := currentMounts[mm.Directory]; found {
			if isEqual(c, mm) {
				log.Debugf("Device %s is already mounted at %s", mm.Device, mm.Directory)
				alreadyMounted[mm.Directory] = struct{}{}
				continue
			}

			if err := umount(c); err != nil {
				errf := fmt.Errorf("cannot umount '%s': %+v. New configuration '%+v' will not be mounted", c.Directory, err, mm)
				merr = multierror.Append(merr, errf)
				log.Warn(errf)
				continue
			}

			log.Infof("Device '%s' umounted", c.Directory)
		}

		if err := mount(mm, m.defaultOptions); err != nil {
			errf := fmt.Errorf("cannot mount '%s' on '%s': %+v", mm.Device, mm.Directory, err)
			merr = multierror.Append(merr, errf)
			log.Error(errf)
			continue
		}

		log.Infof("Device '%s' mounted on '%s' with type '%s' and options '%s'", mm.Device, mm.Directory, mm.Type, mm.Options)
		alreadyMounted[mm.Directory] = struct{}{}
	}
	return merr
}

// isValid return nil if everything is ok:
// - device is a block device
// - directory exists
// - type of the mount is found in /etc/filesystems
func (m *Manager) isValid(mount *models.Mount) error {

	if !strings.Contains(m.filesystems, mount.Type) {
		return fmt.Errorf("mount type not supported")
	}

	if _, err := os.Stat(mount.Directory); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory '%s' not found", mount.Directory)
		}

		return fmt.Errorf("failed to stat '%s': '%w'", mount.Directory, err)
	}

	var stat unix.Stat_t
	err := unix.Lstat(mount.Device, &stat)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("device '%s' not found", mount.Device)
		}

		return fmt.Errorf("failed to stat '%s': '%w'", mount.Device, err)
	}

	if stat.Mode&unix.S_IFMT != unix.S_IFBLK {
		return fmt.Errorf("device '%s' is not a block device", mount.Device)
	}

	return nil
}

func isEqual(mount *models.Mount, other *models.Mount) bool {
	return mount.Device == other.Device && mount.Directory == other.Directory && mount.Type == other.Directory && mount.Options == other.Options
}

// TODO Question: what flags should be passed here?
func mount(m *models.Mount, defaultOptions string) error {
	options := defaultOptions
	if m.Options != "" {
		options = m.Options
	}
	return unix.Mount(m.Device, m.Directory, m.Type, uintptr(0), options)
}

func umount(m *models.Mount) error {
	return unix.Unmount(m.Directory, unix.MNT_FORCE)
}

func getDefaultMountOptions() string {
	group, err := user.LookupGroup(flottaGroup)
	if err != nil {
		return ""
	}
	usr, err := user.Lookup(flottaUser)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("gid=%s,uid=%s", group.Gid, usr.Uid)
}

func (m *Manager) String() string {
	return "mount"
}
