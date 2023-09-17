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
package runners

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/carina-io/carina/pkg/configuration"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
)

const deviceDir = "/tmp/disk/"

var loops []string
var names []string

func initDevice() {
	_ = os.MkdirAll(deviceDir, os.ModeDir)
	for i := 1; i < 13; i++ {
		name := fmt.Sprintf("%sdisk%d.device", deviceDir, i)
		names = append(names, name)
		loopDevice, err := makeLoopbackDevice(name)
		if err != nil {
			_ = cleanLoopback(loops, names)
			os.Exit(-1)
		}
		loops = append(loops, loopDevice)
	}
}

func makeLoopbackDevice(name string) (string, error) {
	command := exec.Command("losetup", "-f")
	command.Stderr = os.Stderr
	loop := bytes.Buffer{}
	command.Stdout = &loop
	err := command.Run()
	if err != nil {
		return "", err
	}
	loopDev := strings.TrimRight(loop.String(), "\n")
	out, err := exec.Command("truncate", "--size=200G", name).CombinedOutput()
	if err != nil {
		fmt.Println(fmt.Sprintf("failed to truncate output: %s", string(out)))
		return "", err
	}
	out, err = exec.Command("losetup", loopDev, name).CombinedOutput()
	if err != nil {
		fmt.Println(fmt.Sprintf("failed to losetup output: %s", string(out)))
		return "", err
	}
	return loopDev, nil
}

func cleanLoopback(loops []string, files []string) error {
	for _, loop := range loops {
		err := exec.Command("losetup", "-d", loop).Run()
		if err != nil {
			return err
		}
	}
	for _, file := range files {
		err := os.Remove(file)
		if err != nil {
			return err
		}
	}
	return nil
}

// 只有这一个测试方法
func TestDeviceManager(t *testing.T) {
	initDevice()
	defer cleanLoopback(loops, names)

	dm := deviceManager.NewDeviceManager("localhost", nil, nil)
	dc := &deviceCheck{dm: dm}
	defer func() {
		// 清理volumex
		_ = cleanVolume(dm)
		configuration.TestAssistDiskSelector = []string{"^o$"}
		dc.addAndRemoveDevice()
	}()

	err := deviceAddAndRemove(dc)
	if err != nil {
		return
	}
	// 创建volume测试， defter进行删除volume
	err = volumeCreate(dm)
	if err != nil {
		return
	}

	// 清除现有磁盘测试，正在使用的磁盘无法删除
	configuration.TestAssistDiskSelector = []string{"loop8"}
	err = deviceAddAndRemove(dc)
	if err != nil {
		return
	}
}

func deviceAddAndRemove(dc *deviceCheck) error {
	dc.addAndRemoveDevice()

	pvInfo, err := dc.dm.VolumeManager.GetCurrentPvStruct()
	if err != nil {
		return err
	}

	for _, pv := range pvInfo {
		fmt.Println(fmt.Sprintf("pv: %s, vg: %s, size: %d", pv.PVName, pv.VGName, pv.PVSize>>30))
	}
	vgInfo, err := dc.dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		return err
	}
	for _, vg := range vgInfo {
		fmt.Println(fmt.Sprintf("vg: %s, size: %d", vg.VGName, vg.VGSize>>30))
	}
	return nil
}

func cleanVolume(dm *deviceManager.DeviceManager) error {
	lvinfo, err := dm.VolumeManager.VolumeList("", "")
	if err != nil {
		return err
	}
	for _, lv := range lvinfo {
		if !strings.Contains(lv.LVName, "thin") {
			lvName := strings.Split(lv.LVName, "-")[1]
			err := dm.VolumeManager.DeleteVolume(lvName, lv.VGName)
			if err != nil {
				fmt.Println(fmt.Sprintf("delete volume error %s", err.Error()))
			}
		}
	}

	return nil
}

func volumeCreate(dm *deviceManager.DeviceManager) error {
	table := []struct {
		vgName string
		lvName string
		size   uint64
	}{
		{vgName: "carina-vg-hdd", lvName: "v1", size: 10 << 30},
		{vgName: "carina-vg-hdd", lvName: "v2", size: 100 << 30},
		{vgName: "carina-vg-hdd", lvName: "v3", size: 200 << 30},
		{vgName: "carina-vg-hdd", lvName: "v4", size: 80 << 30},
		{vgName: "carina-vg-hdd", lvName: "v5", size: 82 << 30},
		{vgName: "carina-vg-hdd", lvName: "v6", size: 39 << 30},
		{vgName: "carina-vg-hdd", lvName: "v7", size: 500 << 30},
	}

	for _, e := range table {
		err := dm.VolumeManager.CreateVolume(e.lvName, e.vgName, e.size, 1)
		if err != nil {
			fmt.Println(fmt.Sprintf("craete volume failed %s", err.Error()))
			return err
		}
	}

	vl, err := dm.VolumeManager.VolumeList("", "")
	if err != nil {
		return err
	}
	for _, v := range vl {
		fmt.Println(fmt.Sprintf("volume name %s vg name %s size %d", v.LVName, v.VGName, v.LVSize>>30))
	}
	return nil
}
