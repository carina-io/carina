/*
   Copyright @ 2021 bocloud <fushaosong@beyondcent.com>.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package filesystem

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
	"k8s.io/utils/io"

	"github.com/carina-io/carina/utils/log"
)

const (
	blkidCmd = "/sbin/blkid"
)

type temporaryer interface {
	Temporary() bool
}

func isSameDevice(dev1, dev2 string) (bool, error) {
	if dev1 == dev2 {
		return true, nil
	}

	var st1, st2 unix.Stat_t
	if err := Stat(dev1, &st1); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat failed for %s: %v", dev1, err)
	}

	if err := Stat(dev2, &st2); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat failed for %s: %v", dev2, err)
	}

	return st1.Rdev == st2.Rdev, nil
}

// IsMounted returns true if device is mounted on target.
// The implementation uses /proc/mounts because some filesystem uses a virtual device.
func IsMounted(device, target string) (bool, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}

	target, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return false, err
	}

	data, err := io.ConsistentRead("/proc/mounts", 3)
	if err != nil {
		return false, fmt.Errorf("could not read /proc/mounts: %v", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// If the filesystem is nfs(cephfs、ussfs etc) and its connection is broken, EvalSymlinks will be stuck.
		// So it should be in before calling EvalSymlinks.
		ok, err := isSameDevice(device, fields[0])
		if err != nil {
			return false, err
		}
		if !ok {
			continue
		}

		d, err := filepath.EvalSymlinks(fields[1])
		if err != nil {
			return false, err
		}
		if d == target {
			return true, nil
		}
	}

	return false, nil
}

// DetectFilesystem returns filesystem type if device has a filesystem.
// This returns an empty string if no filesystem exists.
func DetectFilesystem(device string) (string, error) {
	f, err := os.Open(device)
	if err != nil {
		return "", err
	}
	// synchronizes dirty data
	f.Sync()
	f.Close()

	log.Infof("%s -c /dev/null -o export %s", blkidCmd, device)
	out, err := exec.Command(blkidCmd, "-c", "/dev/null", "-o", "export", device).CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// blkid exists with status 2 when anything can be found
			if exitErr.ExitCode() == 2 {
				return "", nil
			}
		}
		return "", fmt.Errorf("blkid failed: output=%s, device=%s, error=%v", string(out), device, err)
	}
	log.Info(string(out))

	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "TYPE=") {
			return line[5:], nil
		}
	}

	return "", nil
}

// Stat wrapped a golang.org/x/sys/unix.Stat function to handle EINTR signal for Go 1.14+
func Stat(path string, stat *unix.Stat_t) error {
	for {
		err := unix.Stat(path, stat)
		if err == nil {
			return nil
		}
		if e, ok := err.(temporaryer); ok && e.Temporary() {
			continue
		}
		return err
	}
}

// Mknod wrapped a golang.org/x/sys/unix.Mknod function to handle EINTR signal for Go 1.14+
func Mknod(path string, mode uint32, dev int) (err error) {
	for {
		err := unix.Mknod(path, mode, dev)
		if err == nil {
			return nil
		}
		if e, ok := err.(temporaryer); ok && e.Temporary() {
			continue
		}
		return err
	}
}

// Statfs Stats wrapped a golang.org/x/sys/unix.Statfs function to handle EINTR signal for Go 1.14+
func Statfs(path string, buf *unix.Statfs_t) (err error) {
	for {
		err := unix.Statfs(path, buf)
		if err == nil {
			return nil
		}
		if e, ok := err.(temporaryer); ok && e.Temporary() {
			continue
		}
		return err
	}
}
