package hostpath

import (
	"errors"
	"fmt"
	"github.com/carina-io/carina"
	"github.com/carina-io/carina/pkg/configuration"
	"github.com/carina-io/carina/pkg/csidriver/filesystem"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	"github.com/carina-io/carina/utils/mutx"
	"os"
	"path/filepath"
	"strings"
)

type HostPath interface {
	CreateVolume(name, deviceGroup string) error
	DeleteVolume(name, deviceGroup string) error
	ResizeVolume(name, deviceGroup string) error
}

const (
	VOLUMEMUTEX = "VolumeHostMutex"
)

type LocalHostImplement struct {
	Mutex *mutx.GlobalLocks
}

func (v *LocalHostImplement) CreateVolume(name, deviceGroup string) error {
	if !v.Mutex.TryAcquire(VOLUMEMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer v.Mutex.Release(VOLUMEMUTEX)

	workDir := carina.DefaultHostPath
	currentDiskSelector := configuration.DiskSelector()
	for _, v := range currentDiskSelector {
		if v.Name == deviceGroup && strings.ToLower(v.Policy) == carina.HostVolumeType {
			if v.Re != nil && len(v.Re) > 0 {
				if !filepath.IsAbs(v.Re[0]) {
					return errors.New(fmt.Sprint("path must be absolute: %s", v.Re[0]))
				}
				workDir = v.Re[0]
				break
			}
		}
	}
	name = carina.HostPrefix + name
	device := filepath.Join(workDir, name)

	if !utils.DirExists(device) {
		if err := os.MkdirAll(device, 0777); err != nil {
			return err
		}
		if err := os.Chmod(device, 0777); err != nil {
			return err
		}
	}
	return nil
}

func (v *LocalHostImplement) DeleteVolume(name, deviceGroup string) error {
	if !v.Mutex.TryAcquire(VOLUMEMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer v.Mutex.Release(VOLUMEMUTEX)

	workDir := carina.DefaultHostPath
	currentDiskSelector := configuration.DiskSelector()
	for _, v := range currentDiskSelector {
		if v.Name == deviceGroup && strings.ToLower(v.Policy) == carina.HostVolumeType {
			if v.Re != nil && len(v.Re) > 0 {
				if !filepath.IsAbs(v.Re[0]) {
					return errors.New(fmt.Sprint("path must be absolute: %s", v.Re[0]))
				}
				workDir = v.Re[0]
				break
			}
		}
	}
	name = carina.HostPrefix + name
	device := filepath.Join(workDir, name)

	if utils.DirExists(device) {
		_ = filesystem.UnbindMount(device)
		if err := os.RemoveAll(device); err != nil {
			return err
		}
	}
	return nil
}

func (v *LocalHostImplement) ResizeVolume(name, deviceGroup string) error {
	if !v.Mutex.TryAcquire(VOLUMEMUTEX) {
		log.Info("wait other task release mutex, please retry...")
		return errors.New("get global mutex failed")
	}
	defer v.Mutex.Release(VOLUMEMUTEX)
	return nil
}
