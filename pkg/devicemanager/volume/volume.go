package volume

import (
	"carina/pkg/devicemanager/lvmd"
	"carina/utils/mutx"
)

type LocalVolumeImplement struct {
	lv    lvmd.Lvm2
	Mutex *mutx.GlobalLocks
}

func (v *LocalVolumeImplement) CreateVolume(lvName, vgName string, size, ratio uint64) error {
	// TODO: 检查一下该Volume是否已经存在
	thinName := THIN + lvName
	// 配置pool和volume倍数比例，为了创建快照做准备，快照需要volume同等的存储空间
	sizePool := size * ratio
	// 首先创建thin pool
	if err := v.lv.CreateThinPool(thinName, vgName, sizePool); err != nil {
		return err
	}
	// 创建volume卷
	name := LVVolume + lvName
	if err := v.lv.LVCreateFromPool(name, thinName, vgName, size); err != nil {
		return err
	}

	return nil
}

func (v *LocalVolumeImplement) DeleteVolume(lvName, vgName string) error {

	lvInfo, err := v.lv.LVDisplay(lvName, vgName)
	if err != nil {
		return err
	}
	// TODO: 解析lvInfo 获取thin pool name
	thinName := lvInfo

	if err := v.lv.LVRemove(lvName, vgName); err != nil {
		return err
	}

	// ToDO: 需要检查pool中是否有快照

	if err := v.lv.DeleteThinPool(thinName, vgName); err != nil {
		return err
	}

	return nil
}

func (v *LocalVolumeImplement) ResizeVolume(lvName, vgName string, size, ratio uint64) error {

	name := LVVolume + lvName
	_, err := v.lv.LVDisplay(name, vgName)
	if err != nil {
		return err
	}
	// TODO: 判断volume容量是否已经扩容过

	thinName := THIN + lvName
	sizePool := size * ratio
	if err := v.lv.ResizeThinPool(thinName, vgName, sizePool); err != nil {
		return err
	}

	if err := v.lv.LVResize(name, vgName, size); err != nil {
		return err
	}

	return nil
}

func (v *LocalVolumeImplement) CreateSnapshot(snapName, lvName, vgName string) error {

	name := SNAP + snapName
	if err := v.lv.CreateSnapshot(name, lvName, vgName); err != nil {
		return err
	}

	return nil
}

func (v *LocalVolumeImplement) DeleteSnapshot(snapName, vgName string) error {
	if err := v.lv.DeleteSnapshot(snapName, vgName); err != nil {
		return nil
	}
	return nil
}

func (v *LocalVolumeImplement) RestoreSnapshot(snapName, vgName string) error {
	// TODO： 需要检查是否已经umount
	// 恢复快照会导致快照消失
	if err := v.lv.RestoreSnapshot(snapName, vgName); err != nil {
		return err
	}
	return nil
}

func (v *LocalVolumeImplement) CloneVolume(lvName, vgName, newLvName string) error {
	// 获取pool lv，创建一个和一模一样对池子
	_, err := v.lv.LVDisplay(lvName, vgName)
	if err != nil {
		return err
	}
	// TODO: 获取池子大小
	size := uint64(100)
	// 创建thin pool
	thinName := THIN + lvName
	if err := v.lv.CreateThinPool(thinName, vgName, size); err != nil {
		return err
	}
	name := LVVolume + lvName
	if err := v.lv.LVCreateFromPool(name, thinName, vgName, size); err != nil {
		return err
	}

	// TODO： 分两种情况，若是从快照克隆则挂在快照进行文件拷贝，若是克隆volume则创建快照后挂在拷贝

	// mount -t

	return nil
}
