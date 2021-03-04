package localstorage

import (
	"context"
	"errors"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	lcorev1 "k8s.io/client-go/listers/core/v1"
	lstoragev1 "k8s.io/client-go/listers/storage/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"bocloud.com/cloudnative/carina/scheduler/configuration"
	"bocloud.com/cloudnative/carina/scheduler/utils"
	"sort"
	"strings"
)

// 插件名称
const Name = "local-storage"
const undefined = "undefined"

type LocalStorage struct {
	handle    framework.Handle
	scLister  lstoragev1.StorageClassLister
	pvcLister lcorev1.PersistentVolumeClaimLister
	pvLister  lcorev1.PersistentVolumeLister
}

var _ framework.FilterPlugin = &LocalStorage{}
var _ framework.ScorePlugin = &LocalStorage{}

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

	pvcMap, nodeName, err := ls.getLocalStoragePvc(pod)
	if err != nil {
		klog.V(3).ErrorS(err, "get pvc sc failed pod: %v, node: %v", pod.Name, node.Node().Name)
		return framework.NewStatus(framework.Error, "get pv/sc resource error")
	}
	if nodeName != "" && nodeName != node.Node().Name {
		klog.V(3).Infof("mismatch pod: %v, node: %v", pod.Name, node.Node().Name)
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, "pv node mismatch")
	}
	if len(pvcMap) == 0 {
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
	for key, pvs := range pvcMap {
		sort.Slice(pvs, func(i, j int) bool {
			return pvs[i].Spec.Resources.Requests.Storage().Value() > pvs[j].Spec.Resources.Requests.Storage().Value()
		})

		// 对于sc中未设置Device组处理比较复杂,需要判断在多个Device组的情况下，pv是否能够分配
		// 如carina-vg-hdd 20G carina-vg-ssd 40G, pv1.request30 pv2.request.15 pv3.request 6G
		// 我们这里不能采取最优分配算法，应该采用贪婪算法，因为我们CSI控制器对PV的创建是逐个进行的，它没有全局视图
		// 即便如此，由于创建PV是由csi-provisioner发起的，请求顺序不确有可能导致pv不合理分配，所以建议sc设置Device组
		// 正因为如此，按照最小满足开始过滤.
		if key == undefined {
			capacityList := []int64{}
			for _, c := range capacityMap {
				capacityList = append(capacityList, c)
			}
			sort.Slice(capacityList, func(i, j int) bool {
				return capacityList[i] < capacityList[j]
			})
			for _, pv := range pvs {
				requestBytes := pv.Spec.Resources.Requests.Storage().Value()
				requestGb := (requestBytes-1)>>30 + 1
				capacityList = minimumValueMinus(capacityList, requestGb)
				if len(capacityList) == 0 {
					klog.V(3).Infof("mismatch pod: %v, node: %v", pod.Name, node.Node().Name)
					return framework.NewStatus(framework.UnschedulableAndUnresolvable, "node storage resource insufficient")
				}
			}
		} else {
			requestTotalBytes := int64(0)
			for _, pv := range pvs {
				requestTotalBytes += pv.Spec.Resources.Requests.Storage().Value()
			}
			requestTotalGb := (requestTotalBytes-1)>>30 + 1
			if requestTotalGb > capacityMap[key] {
				klog.V(3).Infof("mismatch pod: %v, node: %v", pod.Name, node.Node().Name)
				return framework.NewStatus(framework.UnschedulableAndUnresolvable, "node storage resource insufficient")
			}
		}
	}
	klog.V(3).Infof("filter success pod: %v, node: %v", pod.Name, node.Node().Name)
	return framework.NewStatus(framework.Success, "")
}

// 对节点进行打分（相当于旧版本的 priorities）
func (ls *LocalStorage) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	klog.V(3).Infof("score pod: %v, node: %v", pod.Name, nodeName)
	pvcMap, node, _ := ls.getLocalStoragePvc(pod)
	if node == nodeName {
		return 10, framework.NewStatus(framework.Success)
	}

	if len(pvcMap) == 0 {
		return 5, framework.NewStatus(framework.Success, "")
	}

	// Get Node Info
	// 节点信息快照在执行调度时创建，并在在整个调度周期内不变
	nodeInfo, err := ls.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
	}

	capacityMap := map[string]int64{}
	total := int64(0)
	for key, v := range nodeInfo.Node().Status.Allocatable {
		if strings.HasPrefix(string(key), utils.DeviceCapacityKeyPrefix) {
			capacityMap[string(key)] = v.Value()
			total += v.Value()
		}
	}
	var score int64
	// 计算节点分数
	// 影响磁盘分数的有磁盘容量,磁盘上现有pv数量,磁盘IO
	// 在此我们以磁盘容量作为标准，同时配合设置对磁盘选择策略
	for key, pvs := range pvcMap {
		sort.Slice(pvs, func(i, j int) bool {
			return pvs[i].Spec.Resources.Requests.Storage().Value() > pvs[j].Spec.Resources.Requests.Storage().Value()
		})
		if key == undefined {
			capacityList := []int64{}
			for _, c := range capacityMap {
				capacityList = append(capacityList, c)
			}
			sort.Slice(capacityList, func(i, j int) bool {
				return capacityList[i] < capacityList[j]
			})
			for _, pv := range pvs {
				requestBytes := pv.Spec.Resources.Requests.Storage().Value()
				requestGb := (requestBytes-1)>>30 + 1
				capacityList = minimumValueMinus(capacityList, requestGb)
				if len(capacityList) > 0 {
					score += 1
				}
			}
		} else {
			requestTotalBytes := int64(0)
			for _, pv := range pvs {
				requestTotalBytes += pv.Spec.Resources.Requests.Storage().Value()
			}
			requestTotalGb := (requestTotalBytes-1)>>30 + 1
			ratio := int64(capacityMap[key] / requestTotalGb)

			if configuration.SchedulerStrategy() == configuration.SchedulerSpradout {
				score = reasonableScore(ratio)
			}
			if configuration.SchedulerStrategy() == configuration.SchedulerBinpack {
				score = 6 - reasonableScore(ratio)
			}
		}
	}
	klog.V(3).Infof("score pod: %v, node: %v score %v", pod.Name, nodeName, score)
	return score, framework.NewStatus(framework.Success)
}

// ScoreExtensions of the Score plugin.
func (ls *LocalStorage) ScoreExtensions() framework.ScoreExtensions {
	return nil
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

func minimumValueMinus(array []int64, value int64) []int64 {
	index := -1
	for i, a := range array {
		if a >= value {
			index = i
			break
		}
	}
	if index < 0 {
		return []int64{}
	}
	array[index] = array[index] - value
	sort.Slice(array, func(i, j int) bool {
		return array[i] < array[j]
	})
	return array
}

// 分值范围为0-10，在此降低pv分值比例限制为1-5分
// 考虑到扩容以及提高资源利用率方面，进行中性的评分
// 对于申请用量与现存容量差距巨大，则配置文件中选节点策略可以忽略
func reasonableScore(ratio int64) int64 {
	if ratio > 10 {
		return 5
	}
	if ratio < 2 {
		return 1
	}
	return ratio / 2
}
