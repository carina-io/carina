package lvmd

type lvm2 interface {
	// 检查pv是否存在
	PVCheck(dev string) error
	PVCreate(dev string) error
	PVRemove(dev string) error
	// 列出pv列表
	PVS() (string, error)
	// 扫盲pv加入cache,在服务启动时执行
	PVScan(dev string) error
	PVDisplay(dev string) (string, error)

	VGCheck(vg string) error
	VGCreate(vg string, tags, pvs []string) error
	VGRemove(vg string) error
	VGS() (string, error)
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
	LVDisplay(lv, vg string) (string, error)

	// 快照占用Pool空间，要有足够对池空间才能创建快照，不然会导致数据损坏
	CreateSnapshot(snap, lv, vg string) (string, error)
	DeleteSnapshot(snap, vg string) error
	// 恢复快照会导致此快照消失
	RestoreSnapshot(snap, vg string) error
}
