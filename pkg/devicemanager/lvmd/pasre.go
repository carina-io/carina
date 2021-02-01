package lvmd

import (
	"carina/pkg/devicemanager/types"
	"carina/utils/log"
	"strconv"
	"strings"
)

func parseVgs(vgsString string) []types.VgGroup {
	// LVM2_VG_NAME='lvmvg',LVM2_PV_COUNT='1',LVM2_LV_COUNT='0',LVM2_SNAP_COUNT='0',LVM2_VG_ATTR='wz--n-',LVM2_VG_SIZE='16101933056',LVM2_VG_FREE='16101933056'
	// LVM2_VG_NAME='v1',LVM2_PV_COUNT='2',LVM2_LV_COUNT='0',LVM2_SNAP_COUNT='0',LVM2_VG_ATTR='wz--n-',LVM2_VG_SIZE='32203866112',LVM2_VG_FREE='32203866112'
	// LVM2_VG_NAME='v1',LVM2_PV_NAME='/dev/loop2',LVM2_PV_COUNT='1',LVM2_LV_COUNT='0',LVM2_SNAP_COUNT='0',LVM2_VG_ATTR='wz--n-',LVM2_VG_SIZE='16101933056',LVM2_VG_FREE='16101933056'
	resp := []types.VgGroup{}

	vgsString = strings.ReplaceAll(vgsString, "'", "")
	vgsString = strings.ReplaceAll(vgsString, " ", "")

	vgsList := strings.Split(vgsString, "\n")
	for _, vgs := range vgsList {
		tmp := types.VgGroup{}
		vg := strings.Split(vgs, ",")
		for _, v := range vg {
			k := strings.Split(v, "=")

			switch k[0] {
			case "LVM2_VG_NAME":
				tmp.VGName = k[1]
			case "LVM2_PV_NAME":
				tmp.PVName = k[1]
			case "LVM2_PV_COUNT":
				tmp.PVCount, _ = strconv.ParseUint(k[1], 10, 64)
			case "LVM2_LV_COUNT":
				tmp.LVCount, _ = strconv.ParseUint(k[1], 10, 64)
			case "LVM2_SNAP_COUNT":
				tmp.SnapCount, _ = strconv.ParseUint(k[1], 10, 64)
			case "LVM2_VG_ATTR":
				tmp.VGAttr = k[1]
			case "LVM2_VG_SIZE":
				tmp.VGSize, _ = strconv.ParseUint(k[1], 10, 64)
			case "LVM2_VG_FREE":
				tmp.VGFree, _ = strconv.ParseUint(k[1], 10, 64)
			default:
				log.Warnf("undefined flied %s-%s", k[0], k[1])
			}
		}
		tmp.PVS = []*types.PVInfo{}
		resp = append(resp, tmp)
	}
	return resp
}
