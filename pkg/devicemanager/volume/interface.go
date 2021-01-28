package volume

const (
	THIN     = "thin-"
	SNAP     = "snap-"
	LVVolume = "volume-"
)

type LocalVolume interface {
	CreateVolume(lvName, vgName string, size, ratio uint64) error
	DeleteVolume(lvName, vgName string) error
	ExtendVolume(lvName, vgName string, size, ratio uint64) error

	CreateSnapshot(snapName, lvName, vgName string) error
	DeleteSnapshot(snapName, vgName string) error
	RestoreSnapshot(snapName, vgName string) error

	CloneVolume(lvName, vgName, newLvName string) error
}
