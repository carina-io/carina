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

package driver

import (
	"context"
	"errors"
	"github.com/anuvu/disko"
	"github.com/anuvu/disko/linux"
	"github.com/carina-io/carina"
	"github.com/carina-io/carina/pkg/configuration"
	"github.com/carina-io/carina/pkg/csidriver/driver/k8s"
	"github.com/carina-io/carina/pkg/csidriver/filesystem"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	"github.com/carina-io/carina/pkg/devicemanager/types"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	"github.com/carina-io/carina/utils/mutx"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mountutil "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
)

const (
	// DeviceDirectory is a directory where Node service creates device files.
	DeviceDirectory  = "/dev/carina"
	findmntCmd       = "/usr/bin/findmnt"
	devicePermission = 0600 | unix.S_IFBLK
)

// NewNodeService returns a new NodeServer.
func NewNodeService(dm *deviceManager.DeviceManager, service *k8s.LogicVolumeService) csi.NodeServer {
	return &nodeService{
		dm:           dm,
		k8sLVService: service,
		mutex:        mutx.NewGlobalLocks(),
		mounter: mountutil.SafeFormatAndMount{
			Interface: mountutil.New(""),
			Exec:      utilexec.New(),
		},
	}
}

type nodeService struct {
	csi.UnimplementedNodeServer
	dm           *deviceManager.DeviceManager
	k8sLVService *k8s.LogicVolumeService
	mutex        *mutx.GlobalLocks
	mounter      mountutil.SafeFormatAndMount
}

func (s *nodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volumeContext := req.GetVolumeContext()
	volumeID := req.GetVolumeId()

	log.Info("NodePublishVolume called",
		" volume_id ", volumeID,
		" publish_context ", req.GetPublishContext(),
		" target_path ", req.GetTargetPath(),
		" volume_capability ", req.GetVolumeCapability(),
		" read_only ", req.GetReadonly(),
		" num_secrets ", len(req.GetSecrets()),
		" volume_context ", volumeContext)

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no target_path is provided")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "no volume_capability is provided")
	}
	isBlockVol := req.GetVolumeCapability().GetBlock() != nil
	isFsVol := req.GetVolumeCapability().GetMount() != nil
	if !(isBlockVol || isFsVol) {
		return nil, status.Errorf(codes.InvalidArgument, "no supported volume capability: %v", req.GetVolumeCapability())
	}

	if acquired := s.mutex.TryAcquire(volumeID); !acquired {
		log.Warnf("An publish operation with the given volume %s already exists", volumeID)
		return nil, status.Errorf(codes.Aborted, "an publish operation with the given volume %s already exists", volumeID)
	}
	defer s.mutex.Release(volumeID)

	cacheVolumeId := volumeContext[carina.VolumeCacheId]
	if cacheVolumeId != "" {
		return s.nodePublishBcacheVolume(ctx, req)
	}

	var lv *types.LvInfo
	var err error
	lvr, err := s.k8sLVService.GetLogicVolumeByVolumeId(ctx, volumeID)
	if err != nil {
		return nil, err
	}
	switch lvr.Annotations[carina.VolumeManagerType] {
	case carina.LvmVolumeType:
		lv, err = s.getLvFromContext(lvr.Spec.DeviceGroup, volumeID)
		if err != nil {
			return nil, err
		}

		if lv == nil {
			return nil, status.Errorf(codes.NotFound, "failed to find LV: %s", volumeID)
		}
		if isBlockVol {
			_, err = s.nodePublishLvmBlockVolume(req, lv)
		} else if isFsVol {
			_, err = s.nodePublishLvmFilesystemVolume(req, lv)
		}

		if err != nil {
			return nil, err
		}
	case carina.RawVolumeType:
		partition, err := s.getPartitionFromContext(lvr.Spec.DeviceGroup, volumeID)
		if err != nil {
			return nil, err
		}
		disk, err := s.dm.Partition.ScanDisk(lvr.Spec.DeviceGroup)
		if err != nil {
			return nil, err
		}
		log.Infof(" IsBlockVol: %v ,Touch partittion name:%s,num:%d,start:%d,size:%d", isBlockVol, partition.Name, partition.Number, partition.Start, partition.Size())
		if partition.Name == "" {
			return nil, status.Errorf(codes.NotFound, "failed to find partition: %s", utils.PartitionName(volumeID))
		}
		if isBlockVol {
			_, err = s.nodePublishRawBlockVolume(req, disk, &partition)
		} else if isFsVol {
			_, err = s.nodePublishRawFilesystemVolume(req, disk, &partition)
		}

		if err != nil {
			return nil, err
		}

	default:
		log.Errorf("Create LogicVolume: Create with no support volume type undefined")
		return nil, status.Errorf(codes.InvalidArgument, "Create with no support type ")
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) nodePublishLvmBlockVolume(req *csi.NodePublishVolumeRequest, lv *types.LvInfo) (*csi.NodePublishVolumeResponse, error) {
	// Find lv and create a block device with it
	var stat unix.Stat_t
	target := req.GetTargetPath()
	err := filesystem.Stat(target, &stat)
	switch err {
	case nil:
		if stat.Rdev == unix.Mkdev(lv.LVKernelMajor, lv.LVKernelMinor) && stat.Mode&devicePermission == devicePermission {
			return &csi.NodePublishVolumeResponse{}, nil
		}
		if err := os.Remove(target); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to remove %s", target)
		}
	case unix.ENOENT:
	default:
		return nil, status.Errorf(codes.Internal, "failed to stat: %v", err)
	}

	err = os.MkdirAll(path.Dir(target), 0755)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mkdir failed: target=%s, error=%v", path.Dir(target), err)
	}

	devno := unix.Mkdev(lv.LVKernelMajor, lv.LVKernelMinor)
	if err := filesystem.Mknod(target, devicePermission, int(devno)); err != nil {
		return nil, status.Errorf(codes.Internal, "mknod failed for %s: error=%v", target, err)
	}

	log.Info("NodePublishVolume(block) succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", target)
	return &csi.NodePublishVolumeResponse{}, nil
}
func (s *nodeService) nodePublishRawBlockVolume(req *csi.NodePublishVolumeRequest, disk disko.Disk, part *disko.Partition) (*csi.NodePublishVolumeResponse, error) {
	// Find parttion
	name := linux.GetPartitionKname(disk.Path, part.Number)
	partinfo, err := linux.GetUdevInfo(name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get partinfo %s", err)
	}
	var MAJOR uint64
	var MINOR uint64
	if str, ok := partinfo.Properties["MAJOR"]; ok {
		MAJOR, _ = strconv.ParseUint(str, 10, 32)
	}

	if str, ok := partinfo.Properties["MINOR"]; ok {
		MINOR, _ = strconv.ParseUint(str, 10, 32)

	}

	var stat unix.Stat_t
	target := req.GetTargetPath()
	err = filesystem.Stat(target, &stat)
	isBlock := (stat.Mode & unix.S_IFMT) == unix.S_IFBLK

	log.Info("targetpath is block:", isBlock, MAJOR, MINOR, stat, "err:", err)
	switch err {
	case nil:
		if stat.Rdev == unix.Mkdev(uint32(MAJOR), uint32(MINOR)) && stat.Mode&devicePermission == devicePermission {
			log.Info("stat.Rdev%s", stat.Rdev, unix.Mkdev(uint32(MAJOR), uint32(MINOR)), "stat.Mode", stat.Mode)
			return &csi.NodePublishVolumeResponse{}, nil
		}
		if err := os.Remove(target); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to remove %s", target)
		}
	case unix.ENOENT:
	default:
		return nil, status.Errorf(codes.Internal, "failed to stat: %v", err)
	}

	err = os.MkdirAll(path.Dir(target), 0755)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mkdir failed: target=%s, error=%v", path.Dir(target), err)
	}

	devno := unix.Mkdev(uint32(MAJOR), uint32(MINOR))
	if err := filesystem.Mknod(target, devicePermission, int(devno)); err != nil {
		return nil, status.Errorf(codes.Internal, "mknod failed for %s: error=%v", target, err)
	}

	log.Info("NodePublishVolume(block) succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", target)
	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) nodePublishLvmFilesystemVolume(req *csi.NodePublishVolumeRequest, lv *types.LvInfo) (*csi.NodePublishVolumeResponse, error) {
	// Check request
	mountOption := req.GetVolumeCapability().GetMount()
	if mountOption.FsType == "" {
		mountOption.FsType = "ext4"
	}
	accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
	if accessMode != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
		modeName := csi.VolumeCapability_AccessMode_Mode_name[int32(accessMode)]
		return nil, status.Errorf(codes.FailedPrecondition, "unsupported access mode: %s", modeName)
	}

	// Find lv and create a block device with it
	device := filepath.Join(DeviceDirectory, req.GetVolumeId())
	err := s.createDeviceIfNeeded(device, lv.LVKernelMajor, lv.LVKernelMinor)
	if err != nil {
		return nil, err
	}

	var mountOptions []string
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}

	for _, m := range mountOption.MountFlags {
		if m == "rw" && req.GetReadonly() {
			return nil, status.Error(codes.InvalidArgument, "mount option \"rw\" is specified even though read only mode is specified")
		}
		mountOptions = append(mountOptions, m)
	}

	err = os.MkdirAll(req.GetTargetPath(), 0755)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mkdir failed: target=%s, error=%v", req.GetTargetPath(), err)
	}

	fsType, err := filesystem.DetectFilesystem(device)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "filesystem check failed: volume=%s, error=%v", req.GetVolumeId(), err)
	}

	if fsType != "" && fsType != mountOption.FsType {
		return nil, status.Errorf(codes.Internal, "target device is already formatted with different filesystem: volume=%s, current=%s, new:%s", req.GetVolumeId(), fsType, mountOption.FsType)
	}

	if mountOption.FsType == "xfs" {
		mountOptions = append(mountOptions, "nouuid")
	}

	mounted, err := filesystem.IsMounted(device, req.GetTargetPath())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mount check failed: target=%s, error=%v", req.GetTargetPath(), err)
	}

	if !mounted {
		log.Infof("mount %s %s %s %s", device, req.GetTargetPath(), mountOption.FsType, strings.Join(mountOptions, ","))
		if err := s.mounter.FormatAndMount(device, req.GetTargetPath(), mountOption.FsType, mountOptions); err != nil {
			return nil, status.Errorf(codes.Internal, "mount failed: volume=%s, error=%v", req.GetVolumeId(), err)
		}
		if err := os.Chmod(req.GetTargetPath(), 0777|os.ModeSetgid); err != nil {
			return nil, status.Errorf(codes.Internal, "chmod 2777 failed: target=%s, error=%v", req.GetTargetPath(), err)
		}
	}

	log.Info("NodePublishVolume(fs) succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", req.GetTargetPath(),
		" fstype ", mountOption.FsType)

	return &csi.NodePublishVolumeResponse{}, nil
}
func (s *nodeService) nodePublishRawFilesystemVolume(req *csi.NodePublishVolumeRequest, disk disko.Disk, part *disko.Partition) (*csi.NodePublishVolumeResponse, error) {
	// Check request
	log.Info("NodePublishVolume device: Filesystem")
	mountOption := req.GetVolumeCapability().GetMount()
	if mountOption.FsType == "" {
		mountOption.FsType = "ext4"
	}
	accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
	if accessMode != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
		modeName := csi.VolumeCapability_AccessMode_Mode_name[int32(accessMode)]
		return nil, status.Errorf(codes.FailedPrecondition, "unsupported access mode: %s", modeName)
	}
	device := linux.GetPartitionKname(disk.Path, part.Number)
	log.Info("NodePublishVolume device: ", device)

	partinfo, err := linux.GetUdevInfo(device)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get partinfo %s", err)
	}
	var MAJOR uint64
	var MINOR uint64
	if str, ok := partinfo.Properties["MAJOR"]; ok {
		MAJOR, _ = strconv.ParseUint(str, 10, 32)
	}

	if str, ok := partinfo.Properties["MINOR"]; ok {
		MINOR, _ = strconv.ParseUint(str, 10, 32)

	}
	// Find lv and create a block device with it
	//device := filepath.Join(DeviceDirectory, req.GetVolumeId())
	err = s.createDeviceIfNeeded(device, uint32(MAJOR), uint32(MINOR))
	if err != nil {
		return nil, err
	}

	var mountOptions []string
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}

	for _, m := range mountOption.MountFlags {
		if m == "rw" && req.GetReadonly() {
			return nil, status.Error(codes.InvalidArgument, "mount option \"rw\" is specified even though read only mode is specified")
		}
		mountOptions = append(mountOptions, m)
	}

	err = os.MkdirAll(req.GetTargetPath(), 0755)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mkdir failed: target=%s, error=%v", req.GetTargetPath(), err)
	}

	fsType, err := filesystem.DetectFilesystem(device)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "filesystem check failed: volume=%s, error=%v", req.GetVolumeId(), err)
	}

	if fsType != "" && fsType != mountOption.FsType {
		return nil, status.Errorf(codes.Internal, "target device is already formatted with different filesystem: volume=%s, current=%s, new:%s", req.GetVolumeId(), fsType, mountOption.FsType)
	}

	mounted, err := filesystem.IsMounted(device, req.GetTargetPath())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mount check failed: target=%s, error=%v", req.GetTargetPath(), err)
	}

	if !mounted {
		log.Infof("mount %s %s %s %s", device, req.GetTargetPath(), mountOption.FsType, strings.Join(mountOptions, ","))
		if err := s.mounter.FormatAndMount(device, req.GetTargetPath(), mountOption.FsType, mountOptions); err != nil {
			return nil, status.Errorf(codes.Internal, "mount failed: volume=%s, error=%v", req.GetVolumeId(), err)
		}
		if err := os.Chmod(req.GetTargetPath(), 0777|os.ModeSetgid); err != nil {
			return nil, status.Errorf(codes.Internal, "chmod 2777 failed: target=%s, error=%v", req.GetTargetPath(), err)
		}
	}

	log.Info("NodePublishVolume(fs) succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", req.GetTargetPath(),
		" fstype ", mountOption.FsType)

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) createDeviceIfNeeded(device string, major, minor uint32) error {
	var stat unix.Stat_t
	err := filesystem.Stat(device, &stat)
	switch err {
	case nil:
		// a block device already exists, check its attributes
		if stat.Rdev == unix.Mkdev(major, minor) && (stat.Mode&devicePermission) == devicePermission {
			return nil
		}
		err := os.Remove(device)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to remove device file %s: error=%v", device, err)
		}
		fallthrough
	case unix.ENOENT:
		devno := unix.Mkdev(major, minor)
		if err := filesystem.Mknod(device, devicePermission, int(devno)); err != nil {
			return status.Errorf(codes.Internal, "mknod failed for %s. major=%d, minor=%d, error=%v",
				device, major, minor, err)
		}
	default:
		return status.Errorf(codes.Internal, "failed to stat %s: error=%v", device, err)
	}
	return nil
}

func (s *nodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volID := req.GetVolumeId()
	target := req.GetTargetPath()
	log.Info("NodeUnpublishVolume called",
		" volume_id ", volID,
		" target_path ", target)

	if len(volID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no target_path is provided")
	}

	if acquired := s.mutex.TryAcquire(volID); !acquired {
		log.Warnf("An unpublish operation with the given volume %s already exists", volID)
		return nil, status.Errorf(codes.Aborted, "an unpublish operation with the given volume %s already exists", volID)
	}
	defer s.mutex.Release(volID)

	lvr, err := s.k8sLVService.GetLogicVolumeByVolumeId(ctx, volID)
	if err != nil {
		return nil, err
	}
	var device string
	var backendDevice string = ""
	switch lvr.Annotations[carina.VolumeManagerType] {
	case carina.LvmVolumeType:
		device = filepath.Join(DeviceDirectory, volID)
	case carina.RawVolumeType:
		partition, err := s.getPartitionFromContext(lvr.Spec.DeviceGroup, volID)
		if err != nil {
			return nil, err
		}
		disk, err := s.dm.Partition.ScanDisk(lvr.Spec.DeviceGroup)
		if err != nil {
			return nil, err
		}
		device = linux.GetPartitionKname(disk.Path, partition.Number)

	default:
		log.Errorf("Create LogicVolume: Create with no support volume type undefined")
		return nil, status.Errorf(codes.InvalidArgument, "Create with no support type ")
	}

	bcacheDevice, err := s.getBcacheDevice(volID)
	if err == nil && bcacheDevice != nil {
		device = bcacheDevice.BcachePath
		backendDevice = bcacheDevice.DevicePath
		log.Infof("bcache volume cache device %s backend device %s", device, backendDevice)
	}

	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		if backendDevice != "" {
			_ = s.dm.VolumeManager.DeleteBcache(backendDevice, "")
		}
		// target_path does not exist, but device for mount-type PV may still exist.
		_ = os.Remove(device)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "stat failed for %s: %v", target, err)
	}

	// remove device file if target_path is device, unmount target_path otherwise
	if info.IsDir() {
		if backendDevice != "" {
			unpublishResp, err := s.nodeUnpublishBFileSystemCacheVolume(req, device, backendDevice)
			if err != nil {
				return unpublishResp, err
			}
		} else {
			unpublishResp, err := s.nodeUnpublishFilesystemVolume(req, device)
			if err != nil {
				return unpublishResp, err
			}
		}
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}
	if backendDevice != "" {
		return s.nodeUnpublishBlockCacheVolume(req, device, backendDevice)
	}
	return s.nodeUnpublishBlockVolume(req, device)
}

func (s *nodeService) nodeUnpublishFilesystemVolume(req *csi.NodeUnpublishVolumeRequest, device string) (*csi.NodeUnpublishVolumeResponse, error) {
	target := req.GetTargetPath()
	mounted, err := filesystem.IsMounted(device, target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mount check failed: target=%s, error=%v", target, err)
	}
	if mounted {
		if err := s.mounter.Unmount(target); err != nil {
			return nil, status.Errorf(codes.Internal, "unmount failed for %s: error=%v", target, err)
		}
	}
	if err := os.RemoveAll(target); err != nil {
		return nil, status.Errorf(codes.Internal, "remove dir failed for %s: error=%v", target, err)
	}
	err = os.Remove(device)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "remove device failed for %s: error=%v", device, err)
	}
	log.Info("NodeUnpublishVolume(fs) is succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", target)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) nodeUnpublishBlockVolume(req *csi.NodeUnpublishVolumeRequest, device string) (*csi.NodeUnpublishVolumeResponse, error) {
	if err := os.Remove(req.GetTargetPath()); err != nil {
		return nil, status.Errorf(codes.Internal, "remove failed for %s: error=%v", req.GetTargetPath(), err)
	}

	log.Info("NodeUnpublishVolume(block) is succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", req.GetTargetPath())
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) nodeUnpublishBFileSystemCacheVolume(req *csi.NodeUnpublishVolumeRequest, device, backendDevice string) (*csi.NodeUnpublishVolumeResponse, error) {
	target := req.GetTargetPath()
	mounted, err := filesystem.IsMounted(device, target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mount check failed: target=%s, error=%v", target, err)
	}
	if mounted {
		if err := s.mounter.Unmount(target); err != nil {
			return nil, status.Errorf(codes.Internal, "unmount failed for %s: error=%v", target, err)
		}
	}
	if err := os.RemoveAll(target); err != nil {
		return nil, status.Errorf(codes.Internal, "remove dir failed for %s: error=%v", target, err)
	}
	// delete bcache device
	err = s.dm.VolumeManager.DeleteBcache(backendDevice, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "remove device failed for %s: error=%v", device, err)
	}
	log.Info("NodeUnpublishVolume(fs) is succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", target)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) nodeUnpublishBlockCacheVolume(req *csi.NodeUnpublishVolumeRequest, device, backendDevice string) (*csi.NodeUnpublishVolumeResponse, error) {
	if err := os.Remove(req.GetTargetPath()); err != nil {
		return nil, status.Errorf(codes.Internal, "remove failed for %s: error=%v", req.GetTargetPath(), err)
	}
	// delete bcache device
	err := s.dm.VolumeManager.DeleteBcache(backendDevice, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "remove device failed for %s: error=%v", device, err)
	}
	log.Info("NodeUnpublishVolume(block) is succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", req.GetTargetPath())
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	volID := req.GetVolumeId()
	p := req.GetVolumePath()
	log.Info("NodeGetVolumeStats is called volume_id ", volID, " volume_path ", p)
	if len(volID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(p) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_path is provided")
	}

	var st unix.Stat_t
	switch err := filesystem.Stat(p, &st); err {
	case unix.ENOENT:
		return nil, status.Error(codes.NotFound, "Volume is not found at "+p)
	case nil:
	default:
		return nil, status.Errorf(codes.Internal, "stat on %s was failed: %v", p, err)
	}

	if (st.Mode & unix.S_IFMT) == unix.S_IFBLK {
		f, err := os.Open(p)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "open on %s was failed: %v", p, err)
		}
		defer f.Close()
		pos, err := f.Seek(0, io.SeekEnd)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "seek on %s was failed: %v", p, err)
		}
		return &csi.NodeGetVolumeStatsResponse{
			Usage: []*csi.VolumeUsage{{Total: pos, Unit: csi.VolumeUsage_BYTES}},
		}, nil
	}

	if st.Mode&unix.S_IFDIR == 0 {
		return nil, status.Errorf(codes.Internal, "invalid mode bits for %s: %d", p, st.Mode)
	}

	var sfs unix.Statfs_t
	if err := filesystem.Statfs(p, &sfs); err != nil {
		return nil, status.Errorf(codes.Internal, "statvfs on %s was failed: %v", p, err)
	}

	var usage []*csi.VolumeUsage
	if sfs.Blocks > 0 {
		usage = append(usage, &csi.VolumeUsage{
			Unit:      csi.VolumeUsage_BYTES,
			Total:     int64(sfs.Blocks) * sfs.Frsize,
			Used:      int64(sfs.Blocks-sfs.Bfree) * sfs.Frsize,
			Available: int64(sfs.Bavail) * sfs.Frsize,
		})
	}
	if sfs.Files > 0 {
		usage = append(usage, &csi.VolumeUsage{
			Unit:      csi.VolumeUsage_INODES,
			Total:     int64(sfs.Files),
			Used:      int64(sfs.Files - sfs.Ffree),
			Available: int64(sfs.Ffree),
		})
	}
	return &csi.NodeGetVolumeStatsResponse{Usage: usage}, nil
}

func (s *nodeService) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	vid := req.GetVolumeId()
	vpath := req.GetVolumePath()

	log.Info("NodeExpandVolume is called",
		" volume_id ", vid,
		" volume_path ", vpath,
		" required ", req.GetCapacityRange().GetRequiredBytes(),
		" limit ", req.GetCapacityRange().GetLimitBytes(),
	)

	if len(vid) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(vpath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_path is provided")
	}

	// We need to check the capacity range but don't use the converted value
	// because the filesystem can be resized without the requested size.
	_, err := convertRequestCapacity(req.GetCapacityRange().GetRequiredBytes(), req.GetCapacityRange().GetLimitBytes())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Device type (block or fs, fs type detection) checking will be removed after CSI v1.2.0
	// because `volume_capability` field will be added in csi.NodeExpandVolumeRequest
	info, err := os.Stat(vpath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "volume path is not exist: %s", vpath)
		}
		return nil, status.Errorf(codes.Internal, "stat failed for %s: %v", vpath, err)
	}

	isBlock := !info.IsDir()
	if isBlock {
		log.Info("NodeExpandVolume(block) is skipped",
			" volume_id ", vid,
			" target_path ", vpath,
		)
		return &csi.NodeExpandVolumeResponse{}, nil
	}

	var device string
	lvr, err := s.k8sLVService.GetLogicVolumeByVolumeId(ctx, vid)
	if err != nil {
		return nil, err
	}
	switch lvr.Annotations[carina.VolumeManagerType] {
	case carina.LvmVolumeType:
		lv, err := s.getLvFromContext(lvr.Spec.DeviceGroup, vid)
		if err != nil {
			return nil, err
		}
		if lv == nil {
			return nil, status.Errorf(codes.NotFound, "failed to find LV: %s", vid)
		}
		device = filepath.Join(DeviceDirectory, vid)
		err = s.createDeviceIfNeeded(device, lv.LVKernelMajor, lv.LVKernelMinor)
		if err != nil {
			return nil, err
		}
	case carina.RawVolumeType:
		partition, err := s.getPartitionFromContext(lvr.Spec.DeviceGroup, vid)
		if err != nil {
			return nil, err
		}
		if partition.Name == "" {
			return nil, status.Errorf(codes.NotFound, "failed to find partition: %s", vid)
		}
		disk, err := s.dm.Partition.ScanDisk(lvr.Spec.DeviceGroup)
		if err != nil {
			return nil, err
		}
		device = linux.GetPartitionKname(disk.Path, partition.Number)
		// mounted, err := filesystem.IsMounted(device, vpath)
		// if err != nil {
		// 	return nil, status.Errorf(codes.Internal, "mount check failed: target=%s, error=%v", vpath, err)
		// }
		// if mounted {
		// 	log.Infof("umount %s %s", device, vpath)
		// 	if err := s.mounter.Unmount(vpath); err != nil {
		// 		return nil, status.Errorf(codes.Internal, "mount failed: volume=%s, error=%v", req.GetVolumeId(), err)
		// 	}

		// }
		// var mountOptions []string
		// //mountOptions = append(mountOptions, "rw")
		// fsType, err := filesystem.DetectFilesystem(vpath)
		// if err != nil {
		// 	return nil, status.Errorf(codes.Internal, "filesystem check failed: volume=%s, error=%v", req.GetVolumeId(), err)
		// }

		// if fsType == "" {
		// 	fsType = "ext4"
		// }
		// r := filesystem.NewResizeFs(&s.mounter)
		// if _, err := r.Resize(device, vpath); err != nil {
		// 	return nil, status.Errorf(codes.Internal, "failed to resize filesystem %s (mounted at: %s): %v", vid, vpath, err)
		// }
		// if err := s.mounter.FormatAndMount(device, vpath, fsType, mountOptions); err != nil {
		// 	return nil, status.Errorf(codes.Internal, "mount failed: volume=%s, error=%v", req.GetVolumeId(), err)
		// }
		// log.Info("NodeExpandVolume(fs) is succeeded",
		// 	" volume_id ", vid,
		// 	" target_path ", vpath,
		// )

		//return &csi.NodeExpandVolumeResponse{}, nil

	default:
		log.Errorf("Create LogicVolume: Create with no support volume type undefined")
		return nil, status.Errorf(codes.InvalidArgument, "Create with no support type ")
	}

	args := []string{"-o", "source", "--noheadings", "--target", req.GetVolumePath()}
	output, err := s.mounter.Exec.Command(findmntCmd, args...).Output()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "findmnt error occured: %v", err)
	}

	devicePath := strings.TrimSpace(string(output))
	if len(devicePath) == 0 {
		return nil, status.Errorf(codes.Internal, "filesystem %s is not mounted at %s", vid, vpath)
	}

	if acquired := s.mutex.TryAcquire(vid); !acquired {
		log.Warnf("An expand operation with the given volume %s already exists", vid)
		return nil, status.Errorf(codes.Aborted, "an expand operation with the given volume %s already exists", vid)
	}
	defer s.mutex.Release(vid)

	r := filesystem.NewResizeFs(&s.mounter)
	if _, err := r.Resize(device, vpath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resize filesystem %s (mounted at: %s): %v", vid, vpath, err)
	}

	log.Info("NodeExpandVolume(fs) is succeeded",
		" volume_id ", vid,
		" target_path ", vpath,
	)

	return &csi.NodeExpandVolumeResponse{}, nil
}

func (s *nodeService) NodeGetCapabilities(context.Context, *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	capabilities := []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
	}

	csiCaps := make([]*csi.NodeServiceCapability, len(capabilities))
	for i, capability := range capabilities {
		csiCaps[i] = &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: capability,
				},
			},
		}
	}

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: csiCaps,
	}, nil
}

func (s *nodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId:            s.dm.NodeName,
		MaxVolumesPerNode: 1000,
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{
				carina.TopologyNodeKey: s.dm.NodeName,
			},
		},
	}, nil
}

func (s *nodeService) getLvFromContext(deviceGroup, volumeID string) (*types.LvInfo, error) {
	lvs, err := s.dm.VolumeManager.VolumeList(volumeID, deviceGroup)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list lv :%v", err)
	}

	for _, v := range lvs {
		if v.LVName == volumeID {
			return &v, nil
		}
	}

	return nil, errors.New("not found")
}
func (s *nodeService) getPartitionFromContext(deviceGroup, volumeID string) (disko.Partition, error) {
	return s.dm.Partition.GetPartition(utils.PartitionName(volumeID), deviceGroup)
}

func (s *nodeService) getBcacheDevice(volumeID string) (*types.BcacheDeviceInfo, error) {
	currentDiskSelector := configuration.DiskSelector()
	var diskClass = []string{}
	for _, v := range currentDiskSelector {
		if strings.ToLower(v.Policy) == "raw" {
			continue
		}
		diskClass = append(diskClass, strings.ToLower(v.Name))
	}
	for _, d := range diskClass {
		devicePath := filepath.Join("/dev", d, volumeID)
		_, err := os.Stat(devicePath)
		if err == nil {
			info, err := s.dm.VolumeManager.BcacheDeviceInfo(devicePath)
			if err != nil {
				return nil, err
			}
			return info, nil
		}
	}
	return nil, errors.New("not found")
}

func (s *nodeService) nodePublishBcacheVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {

	volumeContext := req.GetVolumeContext()

	backendDevice := volumeContext[carina.VolumeDevicePath]
	cacheDevice := volumeContext[carina.VolumeCacheDevicePath]
	block := volumeContext[carina.VolumeCacheBlock]
	bucket := volumeContext[carina.VolumeCacheBucket]
	cachePolicy := volumeContext[carina.VolumeCachePolicy]

	if backendDevice == "" || cacheDevice == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "carina.storage.io/path %s carina.storage.io/cache/path %s, can not be empty", backendDevice, cacheDevice)
	}

	cacheDeviceInfo, err := s.dm.VolumeManager.CreateBcache(backendDevice, cacheDevice, block, bucket, cachePolicy)
	if err != nil {
		return nil, err
	}

	isBlockVol := req.GetVolumeCapability().GetBlock() != nil
	isFsVol := req.GetVolumeCapability().GetMount() != nil
	if isBlockVol {
		_, err = s.nodePublishBcacheBlockVolume(req, cacheDeviceInfo)
	} else if isFsVol {
		_, err = s.nodePublishBcacheFilesystemVolume(req, cacheDeviceInfo)
	}
	if err != nil {
		return nil, err
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) nodePublishBcacheBlockVolume(req *csi.NodePublishVolumeRequest, cacheDeviceInfo *types.BcacheDeviceInfo) (*csi.NodePublishVolumeResponse, error) {
	// Find lv and create a block device with it
	var stat unix.Stat_t
	target := req.GetTargetPath()
	err := filesystem.Stat(target, &stat)
	switch err {
	case nil:
		if stat.Rdev == unix.Mkdev(cacheDeviceInfo.KernelMajor, cacheDeviceInfo.KernelMinor) && stat.Mode&devicePermission == devicePermission {
			return &csi.NodePublishVolumeResponse{}, nil
		}
		if err := os.Remove(target); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to remove %s", target)
		}
	case unix.ENOENT:
	default:
		return nil, status.Errorf(codes.Internal, "failed to stat: %v", err)
	}

	err = os.MkdirAll(path.Dir(target), 0755)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mkdir failed: target=%s, error=%v", path.Dir(target), err)
	}

	devno := unix.Mkdev(cacheDeviceInfo.KernelMajor, cacheDeviceInfo.KernelMinor)
	if err := filesystem.Mknod(target, devicePermission, int(devno)); err != nil {
		return nil, status.Errorf(codes.Internal, "mknod failed for %s: error=%v", target, err)
	}

	log.Info("NodePublishVolume(block) succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", target)
	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) nodePublishBcacheFilesystemVolume(req *csi.NodePublishVolumeRequest, cacheDeviceInfo *types.BcacheDeviceInfo) (*csi.NodePublishVolumeResponse, error) {
	// Check request
	mountOption := req.GetVolumeCapability().GetMount()
	if mountOption.FsType == "" {
		mountOption.FsType = "ext4"
	}
	accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
	if accessMode != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
		modeName := csi.VolumeCapability_AccessMode_Mode_name[int32(accessMode)]
		return nil, status.Errorf(codes.FailedPrecondition, "unsupported access mode: %s", modeName)
	}

	var mountOptions []string
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}

	for _, m := range mountOption.MountFlags {
		if m == "rw" && req.GetReadonly() {
			return nil, status.Error(codes.InvalidArgument, "mount option \"rw\" is specified even though read only mode is specified")
		}
		mountOptions = append(mountOptions, m)
	}

	err := os.MkdirAll(req.GetTargetPath(), 0755)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mkdir failed: target=%s, error=%v", req.GetTargetPath(), err)
	}

	fsType, err := filesystem.DetectFilesystem(cacheDeviceInfo.BcachePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "filesystem check failed: volume=%s, error=%v", req.GetVolumeId(), err)
	}

	if fsType != "" && fsType != mountOption.FsType {
		return nil, status.Errorf(codes.Internal, "target device is already formatted with different filesystem: volume=%s, current=%s, new:%s", req.GetVolumeId(), fsType, mountOption.FsType)
	}

	mounted, err := filesystem.IsMounted(cacheDeviceInfo.BcachePath, req.GetTargetPath())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mount check failed: target=%s, error=%v", req.GetTargetPath(), err)
	}

	if !mounted {
		log.Infof("mount %s %s %s %s", cacheDeviceInfo.BcachePath, req.GetTargetPath(), mountOption.FsType, strings.Join(mountOptions, ","))
		if err := s.mounter.FormatAndMount(cacheDeviceInfo.BcachePath, req.GetTargetPath(), mountOption.FsType, mountOptions); err != nil {
			return nil, status.Errorf(codes.Internal, "mount failed: volume=%s, error=%v", req.GetVolumeId(), err)
		}
		if err := os.Chmod(req.GetTargetPath(), 0777|os.ModeSetgid); err != nil {
			return nil, status.Errorf(codes.Internal, "chmod 2777 failed: target=%s, error=%v", req.GetTargetPath(), err)
		}

		r := filesystem.NewResizeFs(&s.mounter)
		if _, err := r.Resize(cacheDeviceInfo.BcachePath, req.GetTargetPath()); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to resize filesystem %s (mounted at: %s): %v", cacheDeviceInfo.BcachePath, req.GetTargetPath(), err)
		}
	}

	log.Info("NodePublishVolume(fs) succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", req.GetTargetPath(),
		" fstype ", mountOption.FsType)

	return &csi.NodePublishVolumeResponse{}, nil
}
