package driver

import (
	"carina/pkg/csidriver/csi"
	"carina/pkg/csidriver/driver/k8s"
	"carina/pkg/csidriver/filesystem"
	"carina/pkg/devicemanager/types"
	"carina/pkg/devicemanager/volume"
	"carina/utils"
	"carina/utils/log"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// DeviceDirectory is a directory where Node service creates device files.
	DeviceDirectory = "/dev/carina"
	//mkfsCmd          = "/sbin/mkfs"
	//mountCmd         = "/bin/mount"
	//mountpointCmd    = "/bin/mountpoint"
	//umountCmd        = "/bin/umount"
	devicePermission = 0600 | unix.S_IFBLK
)

// NewNodeService returns a new NodeServer.
func NewNodeService(nodeName string, volumeManager volume.LocalVolume, service *k8s.LogicVolumeService) csi.NodeServer {
	return &nodeService{
		nodeName:      nodeName,
		volumeManager: volumeManager,
		k8sLVService:  service,
	}
}

type nodeService struct {
	csi.UnimplementedNodeServer

	nodeName      string
	volumeManager volume.LocalVolume
	k8sLVService  *k8s.LogicVolumeService
	mu            sync.Mutex
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
	//isInlineEphemeralVolumeReq := volumeContext[ephVolConKey] == "true"

	s.mu.Lock()
	defer s.mu.Unlock()

	var lv *types.LvInfo
	var err error

	lvr, err := s.k8sLVService.GetLogicVolume(ctx, volumeID)
	if err != nil {
		return nil, err
	}
	lv, err = s.getLvFromContext(lvr.Spec.DeviceGroup, volumeID)
	if err != nil {
		return nil, err
	}

	if lv == nil {
		return nil, status.Errorf(codes.NotFound, "failed to find LV: %s", volumeID)
	}

	if isBlockVol {
		_, err = s.nodePublishBlockVolume(req, lv)
	} else if isFsVol {
		_, err = s.nodePublishFilesystemVolume(req, lv)
	}

	if err != nil {
		return nil, err
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) nodePublishBlockVolume(req *csi.NodePublishVolumeRequest, lv *types.LvInfo) (*csi.NodePublishVolumeResponse, error) {
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

	devno := unix.Mkdev(lv.LVKernelMajor, lv.LVKernelMinor)
	if err := filesystem.Mknod(target, devicePermission, int(devno)); err != nil {
		return nil, status.Errorf(codes.Internal, "mknod failed for %s: error=%v", req.GetTargetPath(), err)
	}

	log.Info("NodePublishVolume(block) succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", req.GetTargetPath())
	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) nodePublishFilesystemVolume(req *csi.NodePublishVolumeRequest, lv *types.LvInfo) (*csi.NodePublishVolumeResponse, error) {
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
	err := s.createDeviceIfNeeded(device, lv)
	if err != nil {
		return nil, err
	}

	fs, err := filesystem.New(mountOption.FsType, device)
	if err != nil {
		return nil, err
	}
	if !fs.Exists() {
		if err := fs.Mkfs(); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create filesystem: volume=%s, error=%v", req.GetVolumeId(), err)
		}
	}

	err = os.MkdirAll(req.GetTargetPath(), 0755)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mkdir failed: target=%s, error=%v", req.GetTargetPath(), err)
	}
	if err := fs.Mount(req.GetTargetPath(), req.GetReadonly()); err != nil {
		return nil, status.Errorf(codes.Internal, "mount failed: volume=%s, error=%v", req.GetVolumeId(), err)
	}
	if err := os.Chmod(req.GetTargetPath(), 0777|os.ModeSetgid); err != nil {
		return nil, status.Errorf(codes.Internal, "chmod 2777 failed: target=%s, error=%v", req.GetTargetPath(), err)
	}

	log.Info("NodePublishVolume(fs) succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", req.GetTargetPath(),
		" fstype ", mountOption.FsType)

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) createDeviceIfNeeded(device string, lv *types.LvInfo) error {
	var stat unix.Stat_t
	err := filesystem.Stat(device, &stat)
	switch err {
	case nil:
		// a block device already exists, check its attributes
		if stat.Rdev == unix.Mkdev(lv.LVKernelMajor, lv.LVKernelMinor) && (stat.Mode&devicePermission) == devicePermission {
			return nil
		}
		err := os.Remove(device)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to remove device file %s: error=%v", device, err)
		}
		fallthrough
	case unix.ENOENT:
		devno := unix.Mkdev(lv.LVKernelMajor, lv.LVKernelMinor)
		if err := filesystem.Mknod(device, devicePermission, int(devno)); err != nil {
			return status.Errorf(codes.Internal, "mknod failed for %s. major=%d, minor=%d, error=%v",
				device, lv.LVKernelMajor, lv.LVKernelMinor, err)
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

	s.mu.Lock()
	defer s.mu.Unlock()

	device := filepath.Join(DeviceDirectory, volID)

	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		// target_path does not exist, but device for mount-type PV may still exist.
		_ = os.Remove(device)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "stat failed for %s: %v", target, err)
	}

	// remove device file if target_path is device, unmount target_path otherwise
	if info.IsDir() {
		unpublishResp, err := s.nodeUnpublishFilesystemVolume(req, device)
		if err != nil {
			return unpublishResp, err
		}
		return &csi.NodeUnpublishVolumeResponse{}, nil
	}
	return s.nodeUnpublishBlockVolume(req)
}

func (s *nodeService) nodeUnpublishFilesystemVolume(req *csi.NodeUnpublishVolumeRequest, device string) (*csi.NodeUnpublishVolumeResponse, error) {
	target := req.GetTargetPath()
	if err := filesystem.Unmount(device, target); err != nil {
		return nil, status.Errorf(codes.Internal, "unmount failed for %s: error=%v", target, err)
	}
	if err := os.RemoveAll(target); err != nil {
		return nil, status.Errorf(codes.Internal, "remove dir failed for %s: error=%v", target, err)
	}
	if err := os.Remove(device); err != nil {
		return nil, status.Errorf(codes.Internal, "remove device failed for %s: error=%v", device, err)
	}

	log.Info("NodeUnpublishVolume(fs) is succeeded",
		" volume_id ", req.GetVolumeId(),
		" target_path ", target)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) nodeUnpublishBlockVolume(req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if err := os.Remove(req.GetTargetPath()); err != nil {
		return nil, status.Errorf(codes.Internal, "remove failed for %s: error=%v", req.GetTargetPath(), err)
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

	device := filepath.Join(DeviceDirectory, vid)
	lvr, err := s.k8sLVService.GetLogicVolume(ctx, vid)
	if err != nil {
		return nil, err
	}

	lv, err := s.getLvFromContext(lvr.Spec.DeviceGroup, vid)
	if err != nil {
		return nil, err
	}
	if lv == nil {
		return nil, status.Errorf(codes.NotFound, "failed to find LV: %s", vid)
	}
	err = s.createDeviceIfNeeded(device, lv)
	if err != nil {
		return nil, err
	}
	fsType, err := filesystem.DetectFilesystem(device)
	if err != nil || fsType == "" {
		return nil, status.Errorf(codes.Internal, "failed to detect filesystem of %s: %v", device, err)
	}

	fs, err := filesystem.New(fsType, device)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create filesystem object with device path %s: %v", device, err)
	}
	if !fs.Exists() {
		return nil, status.Errorf(codes.Internal, "filesystem %s is not mounted at %s", vid, vpath)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err = fs.Resize(vpath)
	if err != nil {
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
		NodeId:            s.nodeName,
		MaxVolumesPerNode: 1000,
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{
				utils.TopologyZoneKey: s.nodeName,
			},
		},
	}, nil
}

func (s *nodeService) getLvFromContext(deviceGroup, volumeID string) (*types.LvInfo, error) {
	lvs, err := s.volumeManager.VolumeList(volumeID, deviceGroup)
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
