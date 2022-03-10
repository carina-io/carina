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
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/carina-io/carina/pkg/csidriver/driver/k8s"
	"github.com/carina-io/carina/pkg/version"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	"github.com/carina-io/carina/utils/mutx"
	"github.com/container-storage-interface/spec/lib/go/csi"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewControllerService returns a new ControllerServer.
func NewControllerService(lvService *k8s.LogicVolumeService, nodeService *k8s.NodeService) csi.ControllerServer {
	return &controllerService{lvService: lvService, nodeService: nodeService, mutex: mutx.NewGlobalLocks()}
}

type controllerService struct {
	csi.UnimplementedControllerServer
	mutex *mutx.GlobalLocks

	lvService   *k8s.LogicVolumeService
	nodeService *k8s.NodeService
}

func (s controllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {

	capabilities := req.GetVolumeCapabilities()
	source := req.GetVolumeContentSource()
	deviceGroup := req.GetParameters()[utils.DeviceDiskKey]
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid name")
	}
	name = strings.ToLower(name)

	// 处理磁盘类型参数，支持carina.storage.io/disk-type:ssd书写方式
    deviceGroup = version.GetDeviceGroup(deviceGroup)
	log.Info("CreateVolume called ",
		" name ", req.GetName(),
		" device_group ", deviceGroup,
		" required ", req.GetCapacityRange().GetRequiredBytes(),
		" limit ", req.GetCapacityRange().GetLimitBytes(),
		" parameters ", req.GetParameters(),
		" num_secrets ", len(req.GetSecrets()),
		" capabilities ", capabilities,
		" content_source ", source,
		" accessibility_requirements ", req.GetAccessibilityRequirements().String())

	if source != nil {
		return nil, status.Error(codes.InvalidArgument, "volume_content_source not supported")
	}
	if capabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "no volume capabilities are provided")
	}

	if acquired := s.mutex.TryAcquire(name); !acquired {
		log.Warnf("an operation with the given Volume ID %s already exists", name)
		return nil, status.Errorf(codes.Aborted, "an operation with the given Volume ID %s already exists", name)
	}
	defer s.mutex.Release(name)

	// check required volume capabilities
	for _, capability := range capabilities {
		if block := capability.GetBlock(); block != nil {
			log.Info("CreateVolume specifies volume capability ", "access_type ", "block")
		} else if mount := capability.GetMount(); mount != nil {
			log.Info("CreateVolume specifies volume capability ",
				"access_type ", "mount",
				"fs_type ", mount.GetFsType(),
				"flags ", mount.GetMountFlags())
		} else {
			return nil, status.Error(codes.InvalidArgument, "unknown or empty access_type")
		}

		if mode := capability.GetAccessMode(); mode != nil {
			modeName := csi.VolumeCapability_AccessMode_Mode_name[int32(mode.GetMode())]
			log.Info("CreateVolume specifies volume capability ", "access_mode ", modeName)
			// we only support SINGLE_NODE_WRITER
			if mode.GetMode() != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
				return nil, status.Errorf(codes.InvalidArgument, "unsupported access mode: %s", modeName)
			}
		}
	}

	requestGb, err := convertRequestCapacity(req.GetCapacityRange().GetRequiredBytes(), req.GetCapacityRange().GetLimitBytes())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// process topology
	var node string
	segments := map[string]string{}
	requirements := req.GetAccessibilityRequirements()

	// 为什么requirements是所有的节点列表，不应该只有已选定的节点吗
	// 只能通过pvc Annotations来判断pvc是否已经被选定节点
	pvcName := req.Parameters["csi.storage.k8s.io/pvc/name"]
	namespace := req.Parameters["csi.storage.k8s.io/pvc/namespace"]
	node, err = s.nodeService.HaveSelectedNode(ctx, namespace, pvcName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can not find pvc %s %s", namespace, name)
	}

	// if bcache type, need create two lvm volume
	cacheDiskRatio := req.GetParameters()[utils.VolumeCacheDiskRatio]
	if cacheDiskRatio != "" && cacheDiskRatio != "0" {
		return s.CreateBcacheVolume(ctx, req, node, requestGb)
	}

	// sc parameter未设置device group
	if node != "" && deviceGroup == "" {
		group, err := s.nodeService.SelectDeviceGroup(ctx, requestGb, node)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get device group %v", err)
		}
		if group == "" {
			return nil, status.Errorf(codes.Internal, "can not find any device group")
		}
		deviceGroup = group
	}

	// 不是调度器完成pv调度，则采用controller调度
	if node == "" {
		// In CSI spec, controllers are required that they response OK even if accessibility_requirements field is nil.
		// So we must create volume, and must not return error response in this case.
		// - https://github.com/container-storage-interface/spec/blob/release-1.1/spec.md#createvolume
		// - https://github.com/kubernetes-csi/csi-test/blob/6738ab2206eac88874f0a3ede59b40f680f59f43/pkg/sanity/controller.go#L404-L428
		log.Info("decide node because accessibility_requirements not found")
		nodeName, group, segmentsTmp, err := s.nodeService.SelectVolumeNode(ctx, requestGb, deviceGroup, requirements)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get max capacity node %v", err)
		}
		if nodeName == "" {
			return nil, status.Error(codes.Internal, "can not find any node")
		}
		if group == "" {
			return nil, status.Error(codes.Internal, "can not find any device group")
		}
		node = nodeName
		segments = segmentsTmp
		deviceGroup = group
	}

	volumeID, deviceMajor, deviceMinor, err := s.lvService.CreateVolume(ctx, namespace, pvcName, node, deviceGroup, name, requestGb, metav1.OwnerReference{}, map[string]string{})
	if err != nil {
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	// pv csi VolumeAttributes
	volumeContext := req.GetParameters()
	volumeContext[utils.DeviceDiskKey] = deviceGroup
	volumeContext[utils.VolumeDevicePath] = fmt.Sprintf("/dev/%s/volume-%s", deviceGroup, name)
	volumeContext[utils.VolumeDeviceNode] = node
	volumeContext[utils.VolumeDeviceMajor] = fmt.Sprintf("%d", deviceMajor)
	volumeContext[utils.VolumeDeviceMinor] = fmt.Sprintf("%d", deviceMinor)

	// pv nodeAffinity
	segments[utils.TopologyNodeKey] = node

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: requestGb << 30,
			VolumeId:      volumeID,
			VolumeContext: volumeContext,
			ContentSource: source,
			AccessibleTopology: []*csi.Topology{
				{
					Segments: segments,
				},
			},
		},
	}, nil
}

func (s controllerService) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	log.Info("DeleteVolume called volume_id ", req.GetVolumeId(), " num_secrets ", len(req.GetSecrets()))
	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume_id is not provided")
	}

	err := s.lvService.DeleteVolume(ctx, req.GetVolumeId())
	if err != nil {
		log.Error(err, " DeleteVolume failed volume_id ", req.GetVolumeId())
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	return &csi.DeleteVolumeResponse{}, nil
}

func (s controllerService) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	log.Info("ValidateVolumeCapabilities called ",
		"volume_id ", req.GetVolumeId(),
		"volume_context ", req.GetVolumeContext(),
		"volume_capabilities ", req.GetVolumeCapabilities(),
		"parameters ", req.GetParameters(),
		"num_secrets ", len(req.GetSecrets()))

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume id is nil")
	}
	if len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume capabilities are empty")
	}

	_, err := s.lvService.GetLogicVolume(ctx, req.GetVolumeId())
	if err != nil {
		if err == k8s.ErrVolumeNotFound {
			return nil, status.Errorf(codes.NotFound, "LogicalVolume for volume id %s is not found", req.GetVolumeId())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Since Carina does not provide means to pre-provision volumes,
	// any existing volume is valid.
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeContext:      req.GetVolumeContext(),
			VolumeCapabilities: req.GetVolumeCapabilities(),
			Parameters:         req.GetParameters(),
		},
	}, nil
}

func (s controllerService) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	topology := req.GetAccessibleTopology()
	capabilities := req.GetVolumeCapabilities()
	log.Info("GetCapacity called volume_capabilities ", capabilities,
		" parameters ", req.GetParameters(),
		" accessible_topology ", topology)
	if capabilities != nil {
		log.Info("capability argument is not nil, but Carina ignores it")
	}

	deviceGroup := req.GetParameters()[utils.DeviceDiskKey]

	// 处理磁盘类型参数，支持carina.storage.io/disk-type:ssd书写方式
	deviceGroup = strings.ToLower(deviceGroup)
	if deviceGroup != "" {
		if !strings.HasPrefix(deviceGroup, "carina-vg-") {
			deviceGroup = fmt.Sprintf("carina-vg-%s", deviceGroup)
		}
	}

	capacity, err := s.nodeService.GetTotalCapacity(ctx, deviceGroup, topology)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &csi.GetCapacityResponse{
		AvailableCapacity: capacity,
	}, nil
}

func (s controllerService) ControllerGetCapabilities(context.Context, *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	capabilities := []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_GET_CAPACITY,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
	}

	csiCaps := make([]*csi.ControllerServiceCapability, len(capabilities))
	for i, capability := range capabilities {
		csiCaps[i] = &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: capability,
				},
			},
		}
	}

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: csiCaps,
	}, nil
}

func (s controllerService) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	log.Infof("ControllerExpandVolume called volumeID %s required %d limit %d num_secrets %d", volumeID, req.GetCapacityRange().GetRequiredBytes(),
		req.GetCapacityRange().GetLimitBytes(), len(req.GetSecrets()))

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "volume id is nil")
	}

	if acquired := s.mutex.TryAcquire(volumeID); !acquired {
		log.Warnf("an operation with the given Volume ID %s already exists", volumeID)
		return nil, status.Errorf(codes.Aborted, "an operation with the given Volume ID %s already exists", volumeID)
	}
	defer s.mutex.Release(volumeID)

	lv, err := s.lvService.GetLogicVolume(ctx, volumeID)
	if err != nil {
		if err == k8s.ErrVolumeNotFound {
			return nil, status.Errorf(codes.NotFound, "LogicalVolume for volume id %s is not found", volumeID)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	requestGb, err := convertRequestCapacity(req.GetCapacityRange().GetRequiredBytes(), req.GetCapacityRange().GetLimitBytes())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	currentSize := lv.Status.CurrentSize
	if currentSize == nil {
		// fill currentGb for old volume created in v0.3.0 or before.
		err := s.lvService.UpdateLogicVolumeCurrentSize(ctx, volumeID, &lv.Spec.Size)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		currentSize = &lv.Spec.Size
	}

	currentGb := currentSize.Value() >> 30
	if requestGb <= currentGb {
		// "NodeExpansionRequired" is still true because it is unknown
		// whether node expansion is completed or not.
		return &csi.ControllerExpandVolumeResponse{
			CapacityBytes:         currentGb << 30,
			NodeExpansionRequired: true,
		}, nil
	}
	capacity, err := s.nodeService.GetCapacityByNodeName(ctx, lv.Spec.NodeName, lv.Spec.DeviceGroup)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if capacity < (requestGb - currentGb) {
		return nil, status.Error(codes.Internal, "not enough space")
	}

	err = s.lvService.ExpandVolume(ctx, volumeID, requestGb)
	if err != nil {
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	// if bcache enable
	cacheDiskRatio := lv.Annotations[utils.VolumeCacheDiskRatio]
	if cacheDiskRatio != "" {
		go func() {
			timeCtx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			ratio, err := strconv.ParseInt(cacheDiskRatio, 10, 64)
			if err != nil {
				log.Errorf("carina.storage.io/cache-disk-ratio %s, Should be in 1-100", cacheDiskRatio)
			}
			if ratio < 1 || ratio >= 100 {
				log.Errorf("carina.storage.io/cache-disk-ratio %s, Should be in 1-100", cacheDiskRatio)
			}
			cacheVolumeName := "volume-cache-" + lv.Name[6:]
			cacheRequestGb := requestGb * ratio / 100

			err = s.lvService.ExpandVolume(timeCtx, cacheVolumeName, cacheRequestGb)
			if err != nil {
				_, ok := status.FromError(err)
				if !ok {
					log.Errorf("cache expand failed %s", err.Error())
				}
			}
		}()
	}

	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         requestGb << 30,
		NodeExpansionRequired: true,
	}, nil
}

func convertRequestCapacity(requestBytes, limitBytes int64) (int64, error) {
	if requestBytes < 0 {
		return 0, errors.New("required capacity must not be negative")
	}
	if limitBytes < 0 {
		return 0, errors.New("capacity limit must not be negative")
	}

	if limitBytes != 0 && requestBytes > limitBytes {
		return 0, fmt.Errorf(
			"requested capacity exceeds limit capacity: request=%d limit=%d", requestBytes, limitBytes,
		)
	}

	if requestBytes == 0 {
		return utils.MinRequestSizeGb, nil
	}
	return (requestBytes-1)>>30 + 1, nil
}

func (s controllerService) CreateBcacheVolume(ctx context.Context, req *csi.CreateVolumeRequest, node string, requestGb int64) (*csi.CreateVolumeResponse, error) {
	source := req.GetVolumeContentSource()
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid name")
	}
	name = strings.ToLower(name)
	requirements := req.GetAccessibilityRequirements()

	backendDiskType := req.GetParameters()[utils.VolumeBackendDiskType]
	cacheDiskType := req.GetParameters()[utils.VolumeCacheDiskType]
	cacheDiskRatio := req.GetParameters()[utils.VolumeCacheDiskRatio]
	cachepolicy := req.GetParameters()[utils.VolumeCachePolicy]

	if backendDiskType == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "%s %s, can not be empty", utils.VolumeBackendDiskType, backendDiskType)
	}

	if cacheDiskType == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "%s %s, can not be empty", utils.VolumeCacheDiskType, cacheDiskType)
	}

	backendDiskType = strings.ToLower(backendDiskType)
	if backendDiskType != "" {
		if !strings.HasPrefix(backendDiskType, "carina-vg-") {
			backendDiskType = fmt.Sprintf("carina-vg-%s", backendDiskType)
		}
	}

	cacheDiskType = strings.ToLower(cacheDiskType)
	if cacheDiskType != "" {
		if !strings.HasPrefix(cacheDiskType, "carina-vg-") {
			cacheDiskType = fmt.Sprintf("carina-vg-%s", cacheDiskType)
		}
	}

	if !utils.ContainsString([]string{"writethrough", "writeback", "writearound"}, cachepolicy) {
		cachepolicy = "writethrough"
	}

	ratio, err := strconv.ParseInt(cacheDiskRatio, 10, 64)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "carina.storage.io/cache-disk-ratio %s, Should be in 1-100", cacheDiskRatio)
	}
	if ratio < 1 || ratio >= 100 {
		return nil, status.Errorf(codes.FailedPrecondition, "carina.storage.io/cache-disk-ratio %s, Should be in 1-100", cacheDiskRatio)
	}

	cacheRequestGb := requestGb * ratio / 100
	backendRequestGb := requestGb

	if cacheRequestGb <= 0 || backendRequestGb <= 0 {
		return nil, status.Errorf(codes.FailedPrecondition, "pvc request capacity and cache ratio are inappropriate, cacheRequestGb is %d", cacheRequestGb)
	}

	backendVolumeName := name
	cacheVolumeName := "cache-" + name[6:]
	pvcName := req.Parameters["csi.storage.k8s.io/pvc/name"]
	namespace := req.Parameters["csi.storage.k8s.io/pvc/namespace"]
	segments := map[string]string{}

	if node == "" {
		// xxxx
		log.Info("decide node because accessibility_requirements not found")
		nodeName, segmentsTmp, err := s.nodeService.SelectMultiVolumeNode(ctx, backendDiskType, cacheDiskType, backendRequestGb, cacheRequestGb, requirements)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get max capacity node %v", err)
		}
		if nodeName == "" {
			return nil, status.Error(codes.Internal, "can not find any node")
		}
		node = nodeName
		segments = segmentsTmp
	}

	annotation := map[string]string{
		utils.VolumeCacheDiskRatio: cacheDiskRatio,
	}

	backendDiskVolumeID, backendDiskDeviceMajor, backendDiskDeviceMinor, err := s.lvService.CreateVolume(ctx, namespace, pvcName, node, backendDiskType, backendVolumeName, backendRequestGb, metav1.OwnerReference{}, annotation)
	if err != nil {
		s, ok := status.FromError(err)
		if s.Code() != codes.AlreadyExists {
			if !ok {
				return nil, status.Error(codes.Internal, err.Error())
			}
			return nil, err
		}
	}

	lv, err := s.lvService.GetLogicVolume(ctx, backendDiskVolumeID)
	if err != nil {
		if err == k8s.ErrVolumeNotFound {
			return nil, status.Errorf(codes.NotFound, "LogicalVolume for volume id %s is not found", backendDiskVolumeID)
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	controller := true
	blockOwnerDeletion := true
	owner := metav1.OwnerReference{
		APIVersion:         lv.APIVersion,
		Kind:               lv.Kind,
		Name:               lv.Name,
		UID:                lv.UID,
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}

	cacheDiskVolumeID, cacheDiskDeviceMajor, cacheDiskDeviceMinor, err := s.lvService.CreateVolume(ctx, namespace, pvcName, node, cacheDiskType, cacheVolumeName, cacheRequestGb, owner, annotation)
	if err != nil {
		_, ok := status.FromError(err)
		if !ok {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return nil, err
	}

	// pv csi VolumeAttributes
	volumeContext := req.GetParameters()
	volumeContext[utils.DeviceDiskKey] = backendDiskType
	volumeContext[utils.VolumeDevicePath] = fmt.Sprintf("/dev/%s/volume-%s", backendDiskType, backendVolumeName)
	volumeContext[utils.VolumeDeviceNode] = node
	volumeContext[utils.VolumeDeviceMajor] = fmt.Sprintf("%d", backendDiskDeviceMajor)
	volumeContext[utils.VolumeDeviceMinor] = fmt.Sprintf("%d", backendDiskDeviceMinor)
	volumeContext[utils.VolumeCacheDiskType] = cacheDiskType
	volumeContext[utils.VolumeCacheDevicePath] = fmt.Sprintf("/dev/%s/volume-%s", cacheDiskType, cacheVolumeName)
	volumeContext[utils.VolumeCacheDeviceMajor] = fmt.Sprintf("%d", cacheDiskDeviceMajor)
	volumeContext[utils.VolumeCacheDeviceMinor] = fmt.Sprintf("%d", cacheDiskDeviceMinor)
	volumeContext[utils.VolumeCachePolicy] = cachepolicy
	volumeContext[utils.VolumeCacheDiskRatio] = cacheDiskRatio
	volumeContext[utils.VolumeCacheId] = cacheDiskVolumeID

	// pv nodeAffinity
	segments[utils.TopologyNodeKey] = node

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: requestGb << 30,
			VolumeId:      backendDiskVolumeID,
			VolumeContext: volumeContext,
			ContentSource: source,
			AccessibleTopology: []*csi.Topology{
				{
					Segments: segments,
				},
			},
		},
	}, nil
}
