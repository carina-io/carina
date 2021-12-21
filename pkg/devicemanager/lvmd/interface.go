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

import "github.com/carina-io/carina/pkg/devicemanager/types"

type Lvm2 interface {
	// 检查pv是否存在
	PVCheck(dev string) (string, error)
	PVCreate(dev string) error
	PVRemove(dev string) error
	PVResize(dev string) error
	// 列出pv列表
	PVS() ([]types.PVInfo, error)
	// 扫盲pv加入cache,在服务启动时执行
	PVScan(dev string) error
	PVDisplay(dev string) (*types.PVInfo, error)

	VGCheck(vg string) error
	VGCreate(vg string, tags, pvs []string) error
	VGRemove(vg string) error
	VGS() ([]types.VgGroup, error)
	VGDisplay(vg string) (*types.VgGroup, error)
	VGScan(vg string) error
	// vg卷组增加新的pv
	VGExtend(vg, pv string) error
	// vg卷组安全移除pv
	VGReduce(vg, pv string) error

	// 每一个Volume对应的是一个thin pool下一个lvm卷
	// 若是要扩容卷，则必须先扩容池子
	// 快照占用的是池子剩余的容量
	CreateThinPool(lv, vg string, size uint64) error
	ResizeThinPool(lv, vg string, size uint64) error
	DeleteThinPool(lv, vg string) error
	LVCreateFromPool(lv, thin, vg string, size uint64) error
	// 这个方法不用
	LVCreateFromVG(lv, vg string, size uint64, tags []string, stripe uint, stripeSize string) error
	LVRemove(lv, vg string) error
	LVResize(lv, vg string, size uint64) error
	LVDisplay(lv, vg string) (*types.LvInfo, error)
	// 这个方法会频繁调用
	LVS(lvName string) ([]types.LvInfo, error)

	// 快照占用Pool空间，要有足够对池空间才能创建快照，不然会导致数据损坏
	CreateSnapshot(snap, lv, vg string) error
	DeleteSnapshot(snap, vg string) error
	// 恢复快照会导致此快照消失
	RestoreSnapshot(snap, vg string) error

	// 启动必要的lvm2服务
	StartLvm2() error
	// 清理unknown设备
	RemoveUnknownDevice(vg string) error
	//同步分区表
	PartProbe() error
}
