package lvmd

import (
	"carina/pkg/device"
	"carina/pkg/device/command"
	"carina/pkg/device/types"
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service to manage logical volumes of the volume group.
type LVService interface {
	// Create a logical volume
	CreateLV(ctx context.Context, request types.CreateLVRequest) (*types.CreateLVResponse, error)
	// Remove a logical volume
	RemoveLV(ctx context.Context, request types.RemoveLVRequest) error
	// Resize a logical volume
	ResizeLV(ctx context.Context, request types.ResizeLVRequest) error
}

func NewLVService(mapper *deviceManager.DeviceClassManager, notifyFunc func()) LVService {
	return &LVServiceImplement{
		mapper:     mapper,
		notifyFunc: notifyFunc,
	}
}

type LVServiceImplement struct {
	mapper     *deviceManager.DeviceClassManager
	notifyFunc func()
}

func (s *LVServiceImplement) CreateLV(ctx context.Context, request types.CreateLVRequest) (*types.CreateLVResponse, error) {
	dc, err := s.mapper.DeviceClass(request.DeviceClassName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), request.DeviceClassName)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}
	requested := request.SizeGB << 30
	free, err := vg.Free()
	if err != nil {
		//log.Error("failed to free VG", map[string]interface{}{
		//	log.FnError: err,
		//})
		return nil, status.Error(codes.Internal, err.Error())
	}

	if free < requested {
		//log.Error("no enough space left on VG", map[string]interface{}{
		//	"free":      free,
		//	"requested": requested,
		//})
		return nil, status.Errorf(codes.ResourceExhausted, "no enough space left on VG: free=%d, requested=%d", free, requested)
	}

	var stripe uint
	if dc.Stripe != nil {
		stripe = *dc.Stripe
	}

	lv, err := vg.CreateVolume(request.Name, requested, request.Tags, stripe, dc.StripeSize)
	if err != nil {
		//log.Error("failed to create volume", map[string]interface{}{
		//	"name":      request.GetName(),
		//	"requested": requested,
		//	"tags":      request.GetTags(),
		//})
		return nil, status.Error(codes.Internal, err.Error())
	}
	//s.notify()

	//log.Info("created a new LV", map[string]interface{}{
	//	"name": request.GetName(),
	//	"size": requested,
	//})

	return &types.CreateLVResponse{
		Volume: &types.LogicalVolume{
			Name:     lv.Name(),
			SizeGB:   lv.Size() >> 30,
			DevMajor: lv.MajorNumber(),
			DevMinor: lv.MinorNumber(),
		},
	}, nil
}

func (s *LVServiceImplement) RemoveLV(ctx context.Context, request types.RemoveLVRequest) error {
	dc, err := s.mapper.DeviceClass(request.DeviceClassName)
	if err != nil {
		return status.Errorf(codes.NotFound, "%s: %s", err.Error(), request.DeviceClassName)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return err
	}
	lvs, err := vg.ListVolumes()
	if err != nil {
		//log.Error("failed to list volumes", map[string]interface{}{
		//	log.FnError: err,
		//})
		return status.Error(codes.Internal, err.Error())
	}

	for _, lv := range lvs {
		if lv.Name() != request.Name {
			continue
		}

		err = lv.Remove()
		if err != nil {
			//log.Error("failed to remove volume", map[string]interface{}{
			//	log.FnError: err,
			//	"name":      lv.Name(),
			//})
			return status.Error(codes.Internal, err.Error())
		}
		//s.notify()

		//log.Info("removed a LV", map[string]interface{}{
		//	"name": request.GetName(),
		//})
		break
	}

	return nil
}

func (s *LVServiceImplement) ResizeLV(ctx context.Context, request types.ResizeLVRequest) error {
	dc, err := s.mapper.DeviceClass(request.DeviceClassName)
	if err != nil {
		return status.Errorf(codes.NotFound, "%s: %s", err.Error(), request.DeviceClassName)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return err
	}
	lv, err := vg.FindVolume(request.Name)
	if err == command.ErrNotFound {
		//log.Error("logical volume is not found", map[string]interface{}{
		//	log.FnError: err,
		//	"name":      request.GetName(),
		//})
		return status.Errorf(codes.NotFound, "logical volume %s is not found", request.Name)
	}
	if err != nil {
		//log.Error("failed to find volume", map[string]interface{}{
		//	log.FnError: err,
		//	"name":      request.GetName(),
		//})
		return status.Error(codes.Internal, err.Error())
	}

	requested := request.SizeGB << 30
	current := lv.Size()

	if requested < current {
		//log.Error("shrinking volume size is not allowed", map[string]interface{}{
		//	log.FnError: err,
		//	"name":      request.GetName(),
		//	"requested": requested,
		//	"current":   current,
		//})
		return status.Error(codes.OutOfRange, "shrinking volume size is not allowed")
	}

	free, err := vg.Free()
	if err != nil {
		//log.Error("failed to free VG", map[string]interface{}{
		//	log.FnError: err,
		//	"name":      request.GetName(),
		//})
		return status.Error(codes.Internal, err.Error())
	}
	if free < (requested - current) {
		//log.Error("no enough space left on VG", map[string]interface{}{
		//	log.FnError: err,
		//	"name":      request.GetName(),
		//	"requested": requested,
		//	"current":   current,
		//	"free":      free,
		//})
		return status.Errorf(codes.ResourceExhausted, "no enough space left on VG: free=%d, requested=%d", free, requested-current)
	}

	err = lv.Resize(requested)
	if err != nil {
		//log.Error("failed to resize LV", map[string]interface{}{
		//	log.FnError: err,
		//	"name":      request.GetName(),
		//	"requested": requested,
		//	"current":   current,
		//	"free":      free,
		//})
		return status.Error(codes.Internal, err.Error())
	}
	//s.notify()
	//
	//log.Info("resized a LV", map[string]interface{}{
	//	"name": request.GetName(),
	//	"size": requested,
	//})

	return nil
}
