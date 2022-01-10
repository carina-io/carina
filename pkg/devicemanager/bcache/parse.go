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

package bcache

import (
	"github.com/carina-io/carina/pkg/devicemanager/types"
	"github.com/carina-io/carina/utils/log"
	"strconv"
	"strings"
)

/*
sb.magic		ok
sb.first_sector		8 [match]
sb.csum			712A837772AEBF62 [match]
sb.version		1 [backing device]

dev.label		(empty)
dev.uuid		f1fdcdb6-9661-49e9-92f1-b8f076bb7145
dev.sectors_per_block	1
dev.sectors_per_bucket	1024
dev.data.first_sector	16
dev.data.cache_mode	0 [writethrough]
dev.data.cache_state	1 [clean]

cset.uuid		2b4e7d83-19b4-4703-b31f-7b7ff54d7d6e
*/

func parseBcache(bcacheInfo string) *types.BcacheDeviceInfo {
	resp := &types.BcacheDeviceInfo{}
	bcacheInfoList := strings.Split(bcacheInfo, "\n")

	for _, bi := range bcacheInfoList {
		k1 := strings.ReplaceAll(bi, "\t\t\t", "\t")
		k2 := strings.ReplaceAll(k1, "\t\t", "\t")
		k := strings.Split(k2, "\t")
		if len(k) < 2 {
			continue
		}
		switch k[0] {
		case "sb.magic":
			resp.Magic = k[1]
		case "sb.first_sector":
			resp.FirstSector = k[1]
		case "sb.csum":
			resp.Csum = k[1]
		case "sb.version":
			resp.Version = k[1]
		case "dev.label":
			resp.Label = k[1]
		case "dev.uuid":
			resp.Uuid = k[1]
		case "dev.sectors_per_block":
			resp.SectorsPerBlock = k[1]
		case "dev.sectors_per_bucket":
			resp.SectorsPerBucket = k[1]
		case "dev.data.first_sector":
			resp.DataFirstSector = k[1]
		case "dev.data.cache_mode":
			resp.DataCacheMode = k[1]
		case "dev.data.cache_state":
			resp.DataCacheState = k[1]
		case "cset.uuid":
			resp.CsetUuid = k[1]
		default:
			log.Warnf("undefined field %s=%s", k[0], k[1])
		}
	}
	return resp
}

/*
KNAME="dm-0" MAJ:MIN="252:0"
KNAME="bcache0" MAJ:MIN="251:0"
*/
func parseDevice(deviceInfo string) *types.BcacheDeviceInfo {
	resp := &types.BcacheDeviceInfo{}
	if deviceInfo == "" {
		log.Error("the device information is empty")
		return resp
	}
	deviceInfo = strings.ReplaceAll(deviceInfo, "\"", "")
	cacheDeviceList := strings.Split(deviceInfo, "\n")
	for _, cacheDevice := range cacheDeviceList {
		cds := strings.Split(cacheDevice, " ")
		for _, cd := range cds {
			k := strings.Split(cd, "=")
			switch k[0] {
			case "MAJ:MIN":
				k1 := strings.Split(k[1], ":")
				t, _ := strconv.ParseUint(k1[0], 10, 32)
				resp.KernelMajor = uint32(t)
				t, _ = strconv.ParseUint(k1[1], 10, 32)
				resp.KernelMinor = uint32(t)
			case "KNAME":
				resp.Name = k[1]
			default:
				log.Warnf("undefined filed %s-%s", k[0], k[1])
			}
		}
	}
	resp.BcachePath = "/dev/" + resp.Name
	return resp
}
