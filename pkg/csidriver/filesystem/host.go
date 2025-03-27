package filesystem

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// CheckBindMountByProc 检查 target 目录是否是 source 目录的 bind mount
func CheckBindMountByProc(source, target string) (bool, error) {
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		mountPoint := fields[4] // 挂载目标
		options := fields[len(fields)-1]

		if mountPoint == target && strings.Contains(options, "bind") {
			return true, nil
		}
	}
	return false, nil
}

// CheckBindMountByInode 通过对比 inode 判断是否是 bind mount
func CheckBindMountByInode(source, target string) (bool, error) {
	srcInfo, err := os.Stat(source)
	if err != nil {
		return false, err
	}
	tgtInfo, err := os.Stat(target)
	if err != nil {
		return false, err
	}

	srcStat := srcInfo.Sys().(*syscall.Stat_t)
	tgtStat := tgtInfo.Sys().(*syscall.Stat_t)

	return srcStat.Ino == tgtStat.Ino, nil
}

// BindMount 将源目录绑定挂载到目标目录
func BindMount(source, target string) error {
	// 执行 mount --bind 命令
	cmd := exec.Command("mount", "--bind", source, target)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("绑定挂载失败: %v, 输出: %s", err, out.String())
	}
	return nil
}

// UnbindMount 解除目标目录上的绑定挂载
func UnbindMount(target string) error {
	// 验证目标目录是否存在
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return fmt.Errorf("目标目录不存在或不是一个目录: %s", target)
	}

	// 执行 umount 命令
	cmd := exec.Command("umount", target)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("解除绑定挂载失败: %v, 输出: %s", err, out.String())
	}
	return nil
}
