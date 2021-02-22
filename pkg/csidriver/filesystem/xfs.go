package filesystem

import (
	"carina/utils/exec"
	"carina/utils/log"
	"fmt"
)

const (
	cmdXFSAdmin  = "/usr/sbin/xfs_admin"
	cmdMkfsXfs   = "/sbin/mkfs.xfs"
	cmdXFSGrowFS = "/usr/sbin/xfs_growfs"
	xfsMountOpts = "wsync"
)

type xfs struct {
	device   string
	executor exec.Executor
}

func init() {
	fsTypeMap["xfs"] = func(device string) Filesystem {
		return xfs{device: device, executor: &exec.CommandExecutor{}}
	}
}

func (fs xfs) Exists() bool {
	return fs.executor.ExecuteCommand(cmdXFSAdmin, "-l", fs.device) == nil
}

func (fs xfs) Mkfs() error {
	fsType, err := DetectFilesystem(fs.device)
	if err != nil {
		return err
	}
	if fsType != "" {
		return ErrFilesystemExists
	}

	if err := fs.executor.ExecuteCommand(cmdXFSAdmin, "-l", fs.device); err == nil {
		return ErrFilesystemExists
	}

	out, err := fs.executor.ExecuteCommandWithCombinedOutput(cmdMkfsXfs, "-f", "-q", fs.device)
	if err != nil {
		log.Error(err, "xfs: failed to create",
			" device ", fs.device,
			" output ", string(out))
	}

	log.Info("xfs: created device ", fs.device)
	return nil
}

func (fs xfs) Mount(target string, readonly bool) error {
	return Mount(fs.device, target, "xfs", xfsMountOpts, readonly)
}

func (fs xfs) Unmount(target string) error {
	return Unmount(fs.device, target)
}

func (fs xfs) Resize(target string) error {
	out, err := fs.executor.ExecuteCommandWithCombinedOutput(cmdXFSGrowFS, target)
	if err != nil {
		log.Error(err, "failed to resize xfs filesystem",
			" device ", fs.device,
			" directory ", target,
			" output ", out)
		return fmt.Errorf("failed to resize xfs filesystem: device=%s, directory=%s, err=%v, output=%s",
			fs.device, target, err, out)
	}

	return nil
}
