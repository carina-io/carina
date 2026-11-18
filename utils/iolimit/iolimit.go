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

package iolimit

import (
	"fmt"
	"github.com/carina-io/carina/utils"
	v1 "k8s.io/api/core/v1"
	"os"
	"path"
	"strconv"
	"strings"
)

type CgroupDrivertype string

const (
	CgroupfsDriver CgroupDrivertype = "cgroupfs"
	SystemdDriver  CgroupDrivertype = "systemd"
)

// cgroupRoot is a var so tests can point it at a temp dir.
var cgroupRoot = "/sys/fs/cgroup"

type CgroupName []string

// IsCgroup2UnifiedMode reports whether the cgroup root is a cgroup v2 (unified) hierarchy.
// cgroup.controllers only exists at the root of a cgroup v2 mount.
func IsCgroup2UnifiedMode() bool {
	return utils.FileExists(path.Join(cgroupRoot, "cgroup.controllers"))
}

func SetIOLimit(blkIO *PodBlkIO) error {
	podPath := getPodCgroupPath(blkIO)
	if !utils.DirExists(podPath) {
		return fmt.Errorf("the pod(uid %s)'s cgroup path(%s) does not exist", blkIO.PodUid, podPath)
	}
	ioMaxPath := path.Join(podPath, ioMaxFile)
	if !utils.FileExists(ioMaxPath) {
		return fmt.Errorf("the pod(uid %s)'s %s does not exist: the io controller is not enabled in %s",
			blkIO.PodUid, ioMaxPath, path.Join(path.Dir(podPath), "cgroup.subtree_control"))
	}
	for deviceNo, deviceIOLimit := range blkIO.DeviceIOSet {
		ioStr := getCG2IOLimitStr(deviceNo, deviceIOLimit)
		if err := os.WriteFile(ioMaxPath, []byte(ioStr), 0600); err != nil {
			return fmt.Errorf("failed to write ioStr(%s) to path(%s): %v", ioStr, ioMaxPath, err)
		}
	}
	return nil
}

func NewCgroupName(base CgroupName, components ...string) CgroupName {
	for _, component := range components {
		if strings.Contains(component, "/") || strings.Contains(component, "_") {
			panic(fmt.Errorf("invalid character in component [%q] of CgroupName", component))
		}
	}
	return append(append([]string{}, base...), components...)
}

func escapeSystemdCgroupName(part string) string {
	return strings.Replace(part, "-", "_", -1)
}

func (cgroupName CgroupName) toSystemd() string {
	if len(cgroupName) == 0 || (len(cgroupName) == 1 && cgroupName[0] == "") {
		return "/"
	}
	newparts := []string{}
	for _, part := range cgroupName {
		part = escapeSystemdCgroupName(part)
		newparts = append(newparts, part)
	}

	result, err := expandSlice(strings.Join(newparts, "-") + ".slice")
	if err != nil {
		// Should never happen...
		panic(fmt.Errorf("error converting cgroup name [%v] to systemd format: %v", cgroupName, err))
	}
	return result
}

func (cgroupName CgroupName) toCgroupfs() string {
	return "/" + path.Join(cgroupName...)
}

func getPodCgroupPath(blkIO *PodBlkIO) string {
	cgroupName := generatePodCgroupName(blkIO.PodQos, blkIO.PodUid)
	if getCgroupDriverType() == SystemdDriver {
		return path.Join(cgroupRoot, cgroupName.toSystemd())
	}
	return path.Join(cgroupRoot, cgroupName.toCgroupfs())
}

func generatePodCgroupName(qos v1.PodQOSClass, podUid string) CgroupName {
	cgroupName := CgroupName{"kubepods"}
	var qosClass string
	switch qos {
	case v1.PodQOSGuaranteed:
		return NewCgroupName(cgroupName, "pod"+string(podUid))
	case v1.PodQOSBurstable:
		qosClass = "burstable"
	case v1.PodQOSBestEffort:
		qosClass = "besteffort"
	}
	return NewCgroupName(cgroupName, qosClass, "pod"+string(podUid))
}

// check cgroup driver

func getCgroupDriverType() CgroupDrivertype {
	if utils.DirExists(path.Join(cgroupRoot, "kubepods.slice")) {
		return SystemdDriver
	}
	return CgroupfsDriver
}

func getCG2IOLimitStr(deviceNo string, iolt *IOLimit) string {
	line := deviceNo
	if iolt.Rbps != 0 {
		line += " rbps=" + strconv.FormatUint(iolt.Rbps, 10)
	} else {
		line += " rbps=max"
	}
	if iolt.Riops != 0 {
		line += " riops=" + strconv.FormatUint(iolt.Riops, 10)
	} else {
		line += " riops=max"
	}
	if iolt.Wbps != 0 {
		line += " wbps=" + strconv.FormatUint(iolt.Wbps, 10)
	} else {
		line += " wbps=max"
	}
	if iolt.Wiops != 0 {
		line += " wiops=" + strconv.FormatUint(iolt.Wiops, 10)
	} else {
		line += " wiops=max"
	}
	return line
}
