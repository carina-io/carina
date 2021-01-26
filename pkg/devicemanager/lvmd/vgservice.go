package lvmd

import (
	"carina/pkg/device"
	"carina/pkg/device/command"
	"carina/pkg/device/types"
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sync"
)

// Service to retrieve information of the volume group
type VGService interface {
	// Get the list of logical volumes in the volume group
	GetLVList(ctx context.Context, request types.GetLVListRequest) (*types.GetLVListResponse, error)
	// Get the free space of the volume group in bytes
	GetFreeBytes(ctx context.Context, request types.GetFreeBytesRequest) (*types.GetFreeBytesResponse, error)
	// Stream the volume group metrics
	Watch() (*types.WatchResponse, error)
}

// NewVGService creates a VGServiceServer
func NewVGService(manager *deviceManager.DeviceClassManager) VGService {

	return &VGServiceImplement{
		dcManager: manager,
		mu:        sync.Mutex{},
		watchers:  make(map[int]chan struct{}),
	}
}

type VGServiceImplement struct {
	dcManager *deviceManager.DeviceClassManager

	mu             sync.Mutex
	watcherCounter int
	watchers       map[int]chan struct{}
}

func (s *VGServiceImplement) GetLVList(ctx context.Context, request types.GetLVListRequest) (*types.GetLVListResponse, error) {
	dc, err := s.dcManager.DeviceClass(request.DeviceClassName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), request.DeviceClassName)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}
	lvs, err := vg.ListVolumes()
	if err != nil {
		/*		log.Error("failed to list volumes", map[string]interface{}{
				log.FnError: err,
			})*/
		return nil, status.Error(codes.Internal, err.Error())
	}

	vols := make([]*types.LogicalVolume, len(lvs))
	for i, lv := range lvs {
		vols[i] = &types.LogicalVolume{
			Name:     lv.Name(),
			SizeGB:   (lv.Size() + (1 << 30) - 1) >> 30,
			DevMajor: lv.MajorNumber(),
			DevMinor: lv.MinorNumber(),
			Tags:     lv.Tags(),
		}
	}
	return &types.GetLVListResponse{Volumes: vols}, nil
}

func (s *VGServiceImplement) GetFreeBytes(ctx context.Context, request types.GetFreeBytesRequest) (*types.GetFreeBytesResponse, error) {
	dc, err := s.dcManager.DeviceClass(request.DeviceClassName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), request.DeviceClassName)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}
	vgFree, err := vg.Free()
	if err != nil {
		//log.Error("failed to free VG", map[string]interface{}{
		//	log.FnError: err,
		//})
		return nil, status.Error(codes.Internal, err.Error())
	}

	spare := dc.GetSpare()
	if vgFree < spare {
		vgFree = 0
	} else {
		vgFree -= spare
	}

	return &types.GetFreeBytesResponse{
		FreeBytes: vgFree,
	}, nil
}

/*func (s *VGServiceImplement) send(server proto.VGService_WatchServer) error {
	vgs, err := command.ListVolumeGroups()
	if err != nil {
		return err
	}
	res := &proto.WatchResponse{}
	for _, vg := range vgs {
		vgFree, err := vg.Free()
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		dc, err := s.dcManager.FindDeviceClassByVGName(vg.Name())
		if err == ErrNotFound {
			continue
		}
		if err != nil {
			return err
		}
		if dc.Default {
			res.FreeBytes = vgFree
		}
		res.Items = append(res.Items, &proto.WatchItem{
			DeviceClass: dc.Name,
			FreeBytes:   vgFree,
		})
	}
	return server.Send(res)
}*/

func (s *VGServiceImplement) addWatcher(ch chan struct{}) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	num := s.watcherCounter
	s.watcherCounter++
	s.watchers[num] = ch
	return num
}

func (s *VGServiceImplement) removeWatcher(num int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.watchers[num]; !ok {
		panic("bug")
	}
	delete(s.watchers, num)
}

func (s *VGServiceImplement) notifyWatchers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ch := range s.watchers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (s *VGServiceImplement) Watch() (*types.WatchResponse, error) {
	ch := make(chan struct{}, 1)
	num := s.addWatcher(ch)
	defer s.removeWatcher(num)

	//if err := s.send(server); err != nil {
	//	return err
	//}
	//
	//for {
	//	select {
	//	case <-server.Context().Done():
	//		return server.Context().Err()
	//	case <-ch:
	//		if err := s.send(server); err != nil {
	//			return err
	//		}
	//	}
	//}
	return nil, nil
}
