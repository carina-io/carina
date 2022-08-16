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

package lvmd

import (
	"github.com/carina-io/carina"
	"github.com/carina-io/carina/api"
	"github.com/carina-io/carina/pkg/devicemanager/types"
	"github.com/carina-io/carina/utils/log"
	"strconv"
	"strings"
)

func parseVgs(vgsString string) []api.VgGroup {
	// LVM2_VG_NAME='lvmvg',LVM2_PV_COUNT='1',LVM2_LV_COUNT='0',LVM2_VG_ATTR='wz--n-',LVM2_VG_SIZE='16101933056',LVM2_VG_FREE='16101933056'
	// LVM2_VG_NAME='v1',LVM2_PV_COUNT='2',LVM2_LV_COUNT='0',LVM2_VG_ATTR='wz--n-',LVM2_VG_SIZE='32203866112',LVM2_VG_FREE='32203866112'
	// LVM2_VG_NAME='v1',LVM2_PV_COUNT='1',LVM2_LV_COUNT='0',LVM2_VG_ATTR='wz--n-',LVM2_VG_SIZE='16101933056',LVM2_VG_FREE='16101933056'
	resp := []api.VgGroup{}

	if vgsString == "" {
		return resp
	}

	vgsString = strings.ReplaceAll(vgsString, "'", "")
	vgsString = strings.ReplaceAll(vgsString, " ", "")

	vgsList := strings.Split(vgsString, "\n")
	for _, vgs := range vgsList {
		tmp := api.VgGroup{}
		vg := strings.Split(vgs, ",")
		for _, v := range vg {
			k := strings.Split(v, "=")

			switch k[0] {
			case "LVM2_VG_NAME":
				tmp.VGName = k[1]
			case "LVM2_PV_COUNT":
				tmp.PVCount, _ = strconv.ParseUint(k[1], 10, 64)
			case "LVM2_LV_COUNT":
				tmp.LVCount, _ = strconv.ParseUint(k[1], 10, 64)
			case "LVM2_VG_ATTR":
				tmp.VGAttr = k[1]
			case "LVM2_VG_SIZE":
				tmp.VGSize, _ = strconv.ParseUint(k[1], 10, 64)
			case "LVM2_VG_FREE":
				tmp.VGFree, _ = strconv.ParseUint(k[1], 10, 64)
			default:
				log.Warnf("undefined filed %s-%s", k[0], k[1])
			}
		}
		tmp.PVS = []*api.PVInfo{}
		resp = append(resp, tmp)
	}
	return resp
}

func parseLvs(lvsString string) []types.LvInfo {
	// LVM2_LV_NAME='t1',LVM2_LV_PATH='/dev/v1/t1',LVM2_LV_SIZE='1073741824',LVM2_LV_KERNEL_MAJOR='252',LVM2_LV_KERNEL_MINOR='0',LVM2_ORIGIN='',LVM2_ORIGIN_SIZE='',LVM2_POOL_LV='',LVM2_THIN_COUNT='',LVM2_LV_TAGS='t1'
	// LVM2_LV_NAME='t5',LVM2_LV_PATH='',LVM2_LV_SIZE='6979321856',LVM2_LV_KERNEL_MAJOR='252',LVM2_LV_KERNEL_MINOR='3',LVM2_ORIGIN='',LVM2_ORIGIN_SIZE='',LVM2_POOL_LV='',LVM2_THIN_COUNT='1',LVM2_LV_TAGS=''
	// LVM2_LV_NAME='m2',LVM2_LV_PATH='/dev/v1/m2',LVM2_LV_SIZE='2147483648',LVM2_LV_KERNEL_MAJOR='252',LVM2_LV_KERNEL_MINOR='5',LVM2_ORIGIN='',LVM2_ORIGIN_SIZE='',LVM2_POOL_LV='t5',LVM2_THIN_COUNT='',LVM2_LV_TAGS=''

	resp := []types.LvInfo{}
	if lvsString == "" {
		return resp
	}

	lvsString = strings.ReplaceAll(lvsString, "'", "")
	lvsString = strings.ReplaceAll(lvsString, " ", "")

	lvsList := strings.Split(lvsString, "\n")
	for _, lvs := range lvsList {
		tmp := types.LvInfo{}
		lv := strings.Split(lvs, ",")
		for _, v := range lv {
			k := strings.Split(v, "=")

			switch k[0] {
			case "LVM2_LV_NAME":
				tmp.LVName = k[1]
			case "LVM2_VG_NAME":
				tmp.VGName = k[1]
			case "LVM2_LV_PATH":
				tmp.LVPath = k[1]
			case "LVM2_LV_SIZE":
				tmp.LVSize, _ = strconv.ParseUint(k[1], 10, 64)
			case "LVM2_LV_KERNEL_MAJOR":
				t, _ := strconv.ParseUint(k[1], 10, 32)
				tmp.LVKernelMajor = uint32(t)
			case "LVM2_LV_KERNEL_MINOR":
				t, _ := strconv.ParseUint(k[1], 10, 32)
				tmp.LVKernelMinor = uint32(t)
			case "LVM2_ORIGIN":
				tmp.Origin = k[1]
			case "LVM2_ORIGIN_SIZE":
				tmp.OriginSize, _ = strconv.ParseUint(k[1], 10, 64)
			case "LVM2_POOL_LV":
				tmp.PoolLV = k[1]
			case "LVM2_THIN_COUNT":
				tmp.ThinCount, _ = strconv.ParseUint(k[1], 10, 64)
			case "LVM2_LV_TAGS":
				tmp.LVTags = k[1]
			case "LVM2_DATA_PERCENT":
				tmp.DataPercent, _ = strconv.ParseFloat(k[1], 64)
			case "LVM2_LV_ATTR":
				tmp.LVAttr = k[1]
			case "LVM2_LV_ACTIVE":
				tmp.LVActive = k[1]
			default:
				log.Warnf("undefined field %s=%s", k[0], k[1])
			}
		}
		if strings.HasPrefix(tmp.LVName, carina.VolumePrefix) || strings.HasPrefix(tmp.LVName, carina.ThinPrefix) {
			resp = append(resp, tmp)
		}
	}
	return resp
}

func parsePvs(pvsString string) []api.PVInfo {
	// LVM2_PV_NAME='/dev/loop2',LVM2_VG_NAME='lvmvg',LVM2_PV_FMT='lvm2',LVM2_PV_ATTR='a--',LVM2_PV_SIZE='16101933056',LVM2_PV_FREE='16101933056'
	resp := []api.PVInfo{}

	if pvsString == "" {
		return resp
	}

	pvsString = strings.ReplaceAll(pvsString, "'", "")
	pvsString = strings.ReplaceAll(pvsString, " ", "")

	pvsList := strings.Split(pvsString, "\n")
	for _, pvs := range pvsList {
		tmp := api.PVInfo{}
		pv := strings.Split(pvs, ",")
		for _, v := range pv {
			k := strings.Split(v, "=")

			switch k[0] {
			case "LVM2_PV_NAME":
				tmp.PVName = k[1]
			case "LVM2_VG_NAME":
				tmp.VGName = k[1]
			case "LVM2_PV_FMT":
				tmp.PVFmt = k[1]
			case "LVM2_PV_ATTR":
				tmp.PVAttr = k[1]
			case "LVM2_PV_SIZE":
				tmp.PVSize, _ = strconv.ParseUint(k[1], 10, 64)
			case "LVM2_PV_FREE":
				tmp.PVFree, _ = strconv.ParseUint(k[1], 10, 64)
			default:
				log.Warnf("undefined field %s-%s", k[0], k[1])
			}
		}
		resp = append(resp, tmp)
	}
	return resp
}
