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

package k8s

import (
	"context"
	"errors"
	"fmt"
	"github.com/carina-io/carina"
	"sync"
	"time"

	carinav1 "github.com/carina-io/carina/api/v1"
	"github.com/carina-io/carina/utils/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type logicVolumeService interface {
	CreateVolume(ctx context.Context, namespace, pvc, node, deviceGroup, name string, requestGb int64, owner metav1.OwnerReference, annotation map[string]string) (string, uint32, uint32, error)
	DeleteVolume(ctx context.Context, volumeID string) error
	ExpandVolume(ctx context.Context, volumeID string, requestGb int64) error
	GetLogicVolume(ctx context.Context, volumeID string) (*carinav1.LogicVolume, error)
	UpdateLogicVolumeCurrentSize(ctx context.Context, volumeID string, size *resource.Quantity) error
}

// ErrVolumeNotFound represents the specified volume is not found.
var ErrVolumeNotFound = errors.New("VolumeID is not found")

// LogicVolumeService represents service for LogicVolume.
type LogicVolumeService struct {
	client.Client
	mu sync.Mutex
}

const (
	indexFieldVolumeID = "status.volumeID"
)

// +kubebuilder:rbac:groups=carina.storage.io,resources=LogicVolumes,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// NewLogicVolumeService returns LogicVolumeService.
func NewLogicVolumeService(mgr manager.Manager) (*LogicVolumeService, error) {
	ctx := context.Background()
	err := mgr.GetFieldIndexer().IndexField(ctx, &carinav1.LogicVolume{}, indexFieldVolumeID,
		func(o client.Object) []string {
			return []string{o.(*carinav1.LogicVolume).Status.VolumeID}
		})
	if err != nil {
		return nil, err
	}

	return &LogicVolumeService{Client: mgr.GetClient()}, nil
}

// CreateVolume creates volume
func (s *LogicVolumeService) CreateVolume(ctx context.Context, namespace, pvc, node, deviceGroup, name string, requestGb int64, owner metav1.OwnerReference, annotation map[string]string) (string, uint32, uint32, error) {
	log.Info("k8s.CreateVolume called name ", name, " node ", node, " deviceGroup ", deviceGroup, " size_gb ", requestGb)
	s.mu.Lock()
	defer s.mu.Unlock()

	lv := &carinav1.LogicVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LogicVolume",
			APIVersion: "carina.storage.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotation,
		},
		Spec: carinav1.LogicVolumeSpec{
			NodeName:    node,
			DeviceGroup: deviceGroup,
			Size:        *resource.NewQuantity(requestGb<<30, resource.BinarySI),
			NameSpace:   namespace,
			Pvc:         pvc,
		},
	}

	lv.Finalizers = []string{carina.LogicVolumeFinalizer}

	if owner.Name != "" {
		lv.OwnerReferences = []metav1.OwnerReference{owner}
	}

	existingLV := new(carinav1.LogicVolume)
	err := s.Get(ctx, client.ObjectKey{Name: name}, existingLV)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", 0, 0, err
		}

		err := s.Create(ctx, lv)
		if err != nil {
			return "", 0, 0, err
		}
		log.Info("created LogicVolume CRD name ", name)
	} else {
		// LV with same name was found; check compatibility
		// skip check of capabilities because (1) we allow both of two access types, and (2) we allow only one access mode
		// for ease of comparison, sizes are compared strictly, not by compatibility of ranges
		if !existingLV.IsCompatibleWith(lv) {
			return "", 0, 0, status.Error(codes.AlreadyExists, "Incompatible LogicVolume already exists")
		}
		// compatible LV was found
	}

	for {
		log.Info("waiting for setting 'status.volumeID' name ", name)
		select {
		case <-ctx.Done():
			return "", 0, 0, ctx.Err()
		case <-time.After(1 * time.Second):
		}

		var newLV carinav1.LogicVolume
		err := s.Get(ctx, client.ObjectKey{Name: name}, &newLV)
		if err != nil {
			log.Error(err, " failed to get LogicVolume name ", name)
			return "", 0, 0, err
		}
		if newLV.Status.VolumeID != "" {
			log.Info("create complete k8s.LogicVolume volume_id ", newLV.Status.VolumeID)
			return newLV.Status.VolumeID, newLV.Status.DeviceMajor, newLV.Status.DeviceMinor, nil
		}
		if newLV.Status.Code != codes.OK {
			err := s.Delete(ctx, &newLV)
			if err != nil {
				// log this error but do not return this error, because newLV.Status.Message is more important
				log.Error(err, " failed to delete LogicVolume")
			}

			return "", 0, 0, status.Error(newLV.Status.Code, newLV.Status.Message)
		}
	}
}

// DeleteVolume deletes volume
func (s *LogicVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	log.Info("k8s.DeleteVolume called volumeID ", volumeID)

	lv, err := s.GetLogicVolume(ctx, volumeID)
	if err != nil {
		if err == ErrVolumeNotFound {
			log.Info("volume is not found volume_id ", volumeID)
			return nil
		}
		return err
	}

	err = s.Delete(ctx, lv)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// wait until delete the target volume
	for {
		log.Info("waiting for delete LogicalVolume name ", lv.Name)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}

		err := s.Get(ctx, client.ObjectKey{Name: lv.Name}, new(carinav1.LogicVolume))
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			log.Error(err, " failed to get LogicalVolume name ", lv.Name)
			return err
		}
	}
}

// ExpandVolume expands volume
func (s *LogicVolumeService) ExpandVolume(ctx context.Context, volumeID string, requestGb int64) error {
	log.Info("k8s.ExpandVolume called volumeID ", volumeID, " requestGb ", requestGb)
	s.mu.Lock()
	defer s.mu.Unlock()

	lv, err := s.GetLogicVolume(ctx, volumeID)
	if err != nil {
		return err
	}

	err = s.UpdateLogicVolumeSpecSize(ctx, volumeID, resource.NewQuantity(requestGb<<30, resource.BinarySI))
	if err != nil {
		return err
	}

	// wait until carina-node expands the target volume
	for {
		log.Info("waiting for update of 'status.currentSize' name ", lv.Name)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		var changedLV carinav1.LogicVolume
		err := s.Get(ctx, client.ObjectKey{Name: lv.Name}, &changedLV)
		if err != nil {
			log.Error(err, " failed to get LogicVolume name ", lv.Name)
			return err
		}
		if changedLV.Status.CurrentSize == nil {
			return errors.New("status.currentSize should not be nil")
		}
		if changedLV.Status.CurrentSize.Value() != changedLV.Spec.Size.Value() {
			log.Info("failed to match current size and requested size current ", changedLV.Status.CurrentSize.Value(), " requested ", changedLV.Spec.Size.Value())
			continue
		}

		if changedLV.Status.Code != codes.OK {
			log.Infof("volume expand success %s", volumeID)
			return status.Error(changedLV.Status.Code, changedLV.Status.Message)
		}

		return nil
	}
}

// GetLogicVolume GetVolume returns LogicVolume by volume ID.
func (s *LogicVolumeService) GetLogicVolume(ctx context.Context, volumeID string) (*carinav1.LogicVolume, error) {
	lvList := new(carinav1.LogicVolumeList)
	err := s.List(ctx, lvList, client.MatchingFields{indexFieldVolumeID: volumeID})
	if err != nil {
		return nil, err
	}

	if len(lvList.Items) == 0 {
		return nil, ErrVolumeNotFound
	} else if len(lvList.Items) > 1 {
		return nil, fmt.Errorf("multiple LogicVolume is found for VolumeID %s", volumeID)
	}
	return &lvList.Items[0], nil
}

// UpdateLogicVolumeCurrentSize UpdateCurrentSize updates .Status.CurrentSize of LogicVolume.
func (s *LogicVolumeService) UpdateLogicVolumeCurrentSize(ctx context.Context, volumeID string, size *resource.Quantity) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		lv, err := s.GetLogicVolume(ctx, volumeID)
		if err != nil {
			return err
		}

		lv.Status.CurrentSize = size

		if err := s.Status().Update(ctx, lv); err != nil {
			if apierrors.IsConflict(err) {
				log.Info("detect conflict when LogicVolume status update", "name", lv.Name)
				continue
			}
			log.Error(err, "failed to update LogicVolume status", "name", lv.Name)
			return err
		}

		return nil
	}
}

// UpdateLogicVolumeSpecSize UpdateSpecSize updates .Spec.Size of LogicVolume.
func (s *LogicVolumeService) UpdateLogicVolumeSpecSize(ctx context.Context, volumeID string, size *resource.Quantity) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		lv, err := s.GetLogicVolume(ctx, volumeID)
		if err != nil {
			return err
		}

		lv.Spec.Size = *size
		if lv.Annotations == nil {
			lv.Annotations = make(map[string]string)
		}
		lv.Annotations[carina.ResizeRequestedAtKey] = time.Now().UTC().String()

		if err := s.Update(ctx, lv); err != nil {
			if apierrors.IsConflict(err) {
				log.Info("detect conflict when LogicVolume spec update", "name", lv.Name)
				continue
			}
			log.Error(err, "failed to update LogicVolume spec", "name", lv.Name)
			return err
		}

		return nil
	}
}
