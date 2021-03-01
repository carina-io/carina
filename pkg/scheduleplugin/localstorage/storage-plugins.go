package localstorage

import (
	"carina/utils"
	"context"
	"errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	lcorev1 "k8s.io/client-go/listers/core/v1"
	lstoragev1 "k8s.io/client-go/listers/storage/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"strings"
)

// 插件名称
const Name = "carina-schedule"
const undefined = "undefined"

type LocalStorage struct {
	handle    framework.Handle
	scLister  lstoragev1.StorageClassLister
	pvcLister lcorev1.PersistentVolumeClaimLister
	pvLister  lcorev1.PersistentVolumeLister
}

//type PluginFactory = func(configuration *runtime.Unknown, f FrameworkHandle) (Plugin, error)
func New(_ runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	scLister := handle.SharedInformerFactory().Storage().V1().StorageClasses().Lister()
	pvcLister := handle.SharedInformerFactory().Core().V1().PersistentVolumeClaims().Lister()
	pvLister := handle.SharedInformerFactory().Core().V1().PersistentVolumes().Lister()
	return &LocalStorage{
		handle:    handle,
		pvcLister: pvcLister,
		scLister:  scLister,
		pvLister:  pvLister,
	}, nil
}

func (ls *LocalStorage) Name() string {
	return Name
}

// 过滤掉不符合当前 Pod 运行条件的Node（相当于旧版本的 predicate）
func (ls *LocalStorage) Filter(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod, node *framework.NodeInfo) *framework.Status {

	klog.V(3).Infof("filter pod: %v, node: %v", pod.Name, node.Node().Name)

	pvcmap, nodeName, err := ls.getLocalStoragePvc(pod)
	if err != nil {
		klog.V(3).ErrorS(err, "get pvc sc failed pod: %v, node: %v", pod.Name, node.Node().Name)
		return framework.NewStatus(framework.Error, "get pv/sc resource error")
	}
	if nodeName != "" && nodeName != node.Node().Name {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, "pv node mismatch")
	}
	if len(pvcmap) == 0 {
		return framework.NewStatus(framework.Success, "")
	}

	capacityMap := map[string]int64{}
	total := int64(0)
	for key, v := range node.Node().Status.Allocatable {
		if strings.HasPrefix(string(key), utils.DeviceCapacityKeyPrefix) {
			capacityMap[string(key)] = v.Value()
			total += v.Value()
		}
	}

	// 检查节点容量是否充足
	for key, pvs := range pvcmap {
		requestBytes := int64(0)
		for _, pv := range pvs {
			requestBytes += pv.Spec.Resources.Requests.Storage().Value()
		}
		requestGb := (requestBytes-1)>>30 + 1
		if key == undefined {
			if requestGb > total {
				return framework.NewStatus(framework.UnschedulableAndUnresolvable, "node storage resource insufficient")
			}
		} else {
			if v, ok := capacityMap[key]; !ok {
				return framework.NewStatus(framework.UnschedulableAndUnresolvable, "not found disk group: "+key)
			} else {
				if requestGb > v {
					framework.NewStatus(framework.UnschedulableAndUnresolvable, "node storage resource insufficient")
				}
			}
		}
	}

	return framework.NewStatus(framework.Success, "")
}

func (ls *LocalStorage) getLocalStoragePvc(pod *v1.Pod) (map[string][]*v1.PersistentVolumeClaim, string, error) {
	nodeName := ""
	localPvc := map[string][]*v1.PersistentVolumeClaim{}
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		pvcName := vol.PersistentVolumeClaim.ClaimName

		pvc, err := ls.pvcLister.PersistentVolumeClaims(pod.Namespace).Get(pvcName)
		if err != nil {
			return localPvc, nodeName, err
		}

		if pvc.Spec.StorageClassName == nil {
			continue
		}

		sc, err := ls.scLister.Get(*pvc.Spec.StorageClassName)
		if err != nil {
			return localPvc, nodeName, err
		}
		if sc.Provisioner != utils.CSIPluginName {
			continue
		}

		// 如果存在某个pv已经绑定到某个节点，则新创建对pv只能在该节点创建
		if pvc.Status.Phase == v1.ClaimBound {
			pv, err := ls.pvLister.Get(pvc.Spec.VolumeName)
			if err != nil {
				return localPvc, nodeName, err
			}
			if nodeName == "" {
				nodeName = pv.Spec.CSI.VolumeAttributes[utils.VolumeDeviceNode]
			} else if nodeName != pv.Spec.CSI.VolumeAttributes[utils.VolumeDeviceNode] {
				return localPvc, nodeName, errors.New("pvc node clash")
			}
			continue
		}

		deviceGroup := sc.Parameters[utils.DeviceDiskKey]
		if deviceGroup == "" {
			// sc中未设置device group
			deviceGroup = undefined
		} else if !strings.HasPrefix(deviceGroup, utils.DeviceCapacityKeyPrefix) {
			deviceGroup = utils.DeviceCapacityKeyPrefix + deviceGroup
		}
		localPvc[deviceGroup] = append(localPvc[deviceGroup], pvc)
	}
	return localPvc, nodeName, nil
}
