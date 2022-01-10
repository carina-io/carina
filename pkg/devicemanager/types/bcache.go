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

package types

// BcacheDeviceInfo bcache
type BcacheDeviceInfo struct {
	Magic            string `json:"magic"`
	FirstSector      string `json:"first_sector"`
	Csum             string `json:"csum"`
	Label            string `json:"label"`
	Uuid             string `json:"uuid"`
	SectorsPerBlock  string `json:"sectors_per_block"`
	SectorsPerBucket string `json:"sectors_per_bucket"`
	DataFirstSector  string `json:"data_first_sector"`
	DataCacheMode    string `json:"data_cache_mode"`
	DataCacheState   string `json:"data_cache_state"`
	CsetUuid         string `json:"cset_uuid"`
	Version          string `json:"version"`

	Name        string `json:"name"`
	BcachePath  string `json:"bcache_path"`
	DevicePath  string `json:"device_path"`
	KernelMajor uint32 `json:"lvKernelMajor"`
	KernelMinor uint32 `json:"lvKernelMinor"`
}
