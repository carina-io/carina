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

package carina

const (
	// Version project
	Version = "beta"
	// CSIPluginName PluginName is the name of the CSI plugin.
	CSIPluginName = "carina.storage.io"
	// DefaultCSISocket is the default path of the CSI socket file.
	DefaultCSISocket = "/tmp/csi/csi-provisioner.sock"
	// DefaultReservedSpace Default disk space hold
	DefaultReservedSpace = 10 << 30
	DefaultEdgeSpace     = 1 << 30

	// LogicVolumeFinalizer LogicalVolumeFinalizer is the name of LogicalVolume finalizer
	LogicVolumeFinalizer = "carina.storage.io/logicvolume"
	// ResizeRequestedAtKey is the key of LogicalVolume that represents the timestamp of the resize request.
	ResizeRequestedAtKey = "carina.storage.io/resize-requested-at"

	//  ExclusivityDisk  true or false  is the key indicates that only the disk is used by one pod
	ExclusivityDisk = "carina.storage.io/exclusively-raw-disk"

	VolumeManagerType = "carina.io/volume-manage-type"

	// DeviceDiskKey storage class
	// DeviceDiskKey is the key used in CSI volume create requests to specify a DeviceDiskKey support carina-vg-ssd carina-vg-hdd
	DeviceDiskKey = "carina.storage.io/disk-group-name"

	VolumeBackendDiskType = "carina.storage.io/backend-disk-group-name"
	VolumeCacheDiskType   = "carina.storage.io/cache-disk-group-name"
	// VolumeCacheDiskRatio value: 1-100 Cache Capacity Ratio
	VolumeCacheDiskRatio = "carina.storage.io/cache-disk-ratio"
	// VolumeCachePolicy value: writethrough|writeback|writearound
	VolumeCachePolicy = "carina.storage.io/cache-policy"

	// MinRequestSizeGb pvc
	// default size in GiB for volumes (PVC or inline ephemeral volumes) w/o capacity requests.
	MinRequestSizeGb = 1
	// AnnSelectedNode This annotation is added to a PVC that has been triggered by scheduler to
	// be dynamically provisioned. Its value is the name of the selected node.
	AnnSelectedNode = "volume.kubernetes.io/selected-node"

	// VolumeDevicePath pv csi VolumeAttributes
	VolumeDevicePath  = "carina.storage.io/path"
	VolumeDeviceNode  = "carina.storage.io/node"
	VolumeDeviceMajor = "carina.storage.io/major"
	VolumeDeviceMinor = "carina.storage.io/minor"

	VolumeCacheDevicePath  = "carina.storage.io/cache/path"
	VolumeCacheDeviceMajor = "carina.storage.io/cache/major"
	VolumeCacheDeviceMinor = "carina.storage.io/cache/minor"
	VolumeCacheId          = "carina.storage.io/cache-volume-id"
	VolumeCacheBlock       = "carina.storage.io/cache/block"
	VolumeCacheBucket      = "carina.storage.io/cache/bucket"

	// TopologyNodeKey topology
	// TopologyZoneKey is the key of topology that represents zone name.
	TopologyNodeKey = "topology.carina.storage.io/node"

	// DeviceCapacityKeyPrefix device plugin
	DeviceCapacityKeyPrefix = "carina.storage.io/"
	// DeviceVGSSD support disk type
	DeviceVGSSD = "carina-vg-ssd"
	DeviceVGHDD = "carina-vg-hdd"

	// CarinaSchedule custom schedule
	CarinaSchedule = "carina-scheduler"

	// DeviceVolumeType type
	LvmVolumeType = "lvm"
	RawVolumeType = "raw"

	AllowPodMigrationIfNodeNotready = "carina.storage.io/allow-pod-migration-if-node-notready"

	CarinaPrefix = "carina.io"

	ConfigSourceAnnotationKey = "kubernetes.io/config.source"
	// Updates from Kubernetes API Server
	ApiserverSource = "api"

	ThinPrefix   = "thin-"
	VolumePrefix = "volume-"

	ResourceExhausted = "don't have enough space"
)
