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
	"github.com/bocloud/carina/pkg/devicemanager/types"
	"github.com/bocloud/carina/utils/exec"
)

type BcacheImplement struct {
	Executor exec.Executor
}

func (bi *BcacheImplement) CreateBcache(dev, cacheDev string) error {
	return bi.Executor.ExecuteCommand("make-bcache", "-B", dev, "-C", cacheDev)
}

func (bi *BcacheImplement) RemoveBcache(dev, cacheDev string) error {

	return nil

}

func (bi *BcacheImplement) GetDeviceBcache(dev string) (string, error) {
	return "", nil
}

func (bi *BcacheImplement) RegisterDevice(dev ...string) error {
	for _, d := range dev {
		err := bi.Executor.ExecuteCommand("bcache-register", d)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bi *BcacheImplement) ShowDevice(dev string) (*types.BcacheDeviceInfo, error) {
	bcacheInfo, err := bi.Executor.ExecuteCommandWithOutput("bcache-super-show", dev)
	if err != nil {
		return nil, err
	}
	return parseBcache(bcacheInfo), nil
}
