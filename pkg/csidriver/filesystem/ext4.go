package filesystem

import (
	"carina/utils/exec"
	"carina/utils/log"
	"fmt"
)

const (
	cmdDumpe2fs   = "/sbin/dumpe2fs"
	cmdMkfsExt4   = "/sbin/mkfs.ext4"
	cmdResize2fs  = "/sbin/resize2fs"
	ext4MountOpts = ""
)

type ext4 struct {
	device   string
	executor exec.Executor
}

func init() {
	fsTypeMap["ext4"] = func(device string) Filesystem {
		return ext4{device: device, executor: &exec.CommandExecutor{}}
	}
}

func (fs ext4) Exists() bool {
	return fs.executor.ExecuteCommand(cmdDumpe2fs, "-h", fs.device) == nil
}

func (fs ext4) Mkfs() error {
	fsType, err := DetectFilesystem(fs.device)
	if err != nil {
		return err
	}
	if fsType != "" {
		return ErrFilesystemExists
	}
	if err := fs.executor.ExecuteCommand(cmdDumpe2fs, "-h", fs.device); err == nil {
		return ErrFilesystemExists
	}

	out, err := fs.executor.ExecuteCommandWithCombinedOutput(cmdMkfsExt4, "-F", "-q", "-m", "0", fs.device)
	if err != nil {
		log.Error(err, "ext4: failed to create",
			" device ", fs.device,
			" output ", string(out))
	}

	log.Info("ext4: created device ", fs.device)
	return nil
}

func (fs ext4) Mount(target string, readonly bool) error {
	return Mount(fs.device, target, "ext4", ext4MountOpts, readonly)
}

func (fs ext4) Unmount(target string) error {
	return Unmount(fs.device, target)
}

func (fs ext4) Resize(_ string) error {
	out, err := fs.executor.ExecuteCommandWithCombinedOutput(cmdResize2fs, fs.device)
	if err != nil {
		log.Error(err, "failed to resize ext4 filesystem",
			" device ", fs.device,
			" output ", out)
		return fmt.Errorf("failed to resize ext4 filesystem: device=%s, err=%v, output=%s",
			fs.device, err, out)
	}

	return nil
}
