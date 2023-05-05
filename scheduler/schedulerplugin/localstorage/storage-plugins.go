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

package localstorage

import (
	"context"
	"errors"
	carinav1 "github.com/carina-io/carina-api/api/v1"
	carinav1beta1 "github.com/carina-io/carina-api/api/v1beta1"
	carina "github.com/carina-io/carina/scheduler"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"sort"
	"strconv"
	"strings"

	"k8s.io/client-go/dynamic"

	"github.com/carina-io/carina/scheduler/configuration"
	"github.com/carina-io/carina/scheduler/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	lcorev1 "k8s.io/client-go/listers/core/v1"
	lstoragev1 "k8s.io/client-go/listers/storage/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// Name 插件名称
const Name = "local-storage"
const MaxScore int64 = 10

type LocalStorage struct {
	handle        framework.Handle
	scLister      lstoragev1.StorageClassLister
	pvcLister     lcorev1.PersistentVolumeClaimLister
	pvLister      lcorev1.PersistentVolumeLister
	lvLister      cache.GenericLister
	nsrLister     cache.GenericLister
	dynamicClient dynamic.Interface
}

type pvcRequest struct {
	exclusive bool
	request   int64
}

var _ framework.FilterPlugin = &LocalStorage{}
var _ framework.ScorePlugin = &LocalStorage{}

// New type PluginFactory = func(configuration *runtime.Unknown, f FrameworkHandle) (Plugin, error)
func New(_ runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	scLister := handle.SharedInformerFactory().Storage().V1().StorageClasses().Lister()
	pvcLister := handle.SharedInformerFactory().Core().V1().PersistentVolumeClaims().Lister()
	pvLister := handle.SharedInformerFactory().Core().V1().PersistentVolumes().Lister()
	dynamicClient := newDynamicClientFromConfig()
	dynamicSharedInformerFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient, 0, v1.NamespaceAll, nil)
	lvLister := dynamicSharedInformerFactory.ForResource(carinav1.GroupVersion.WithResource("logicvolumes")).Lister()
	nsrLister := dynamicSharedInformerFactory.ForResource(carinav1beta1.GroupVersion.WithResource("nodestorageresources")).Lister()
	ctx := context.TODO()
	dynamicSharedInformerFactory.Start(ctx.Done())
	dynamicSharedInformerFactory.WaitForCacheSync(ctx.Done())
	return &LocalStorage{
		handle:        handle,
		pvcLister:     pvcLister,
		scLister:      scLister,
		pvLister:      pvLister,
		lvLister:      lvLister,
		nsrLister:     nsrLister,
		dynamicClient: dynamicClient,
	}, nil
}

func (ls *LocalStorage) Name() string {
	return Name
}

// Filter 过滤掉不符合当前 Pod 运行条件的Node（相当于旧版本的 predicate）
func (ls *LocalStorage) Filter(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod, node *framework.NodeInfo) *framework.Status {
	klog.V(3).Infof("filter pod: %s, node: %s", pod.Name, node.Node().Name)
	pvcRequestMap, nodeName, useRaw, err := ls.getPvcRequestMap(pod)
	if err != nil {
		klog.V(3).ErrorS(err, "failed to get pvc/sc, pod: %s, node: %vs", pod.Name, node.Node().Name)
		return framework.NewStatus(framework.Error, err.Error())
	}

	if nodeName != "" && nodeName != node.Node().Name {
		klog.V(3).Infof("mismatch pod: %s, node: %s", pod.Name, node.Node().Name)
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, "pv node mismatch")
	}

	if len(pvcRequestMap) == 0 {
		return framework.NewStatus(framework.Success, "")
	}

	allocatableMap, err := ls.getAllocatableMap(useRaw, pod.Name, node.Node().Name)
	if err != nil {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
	}

	// 检查节点容量是否充足
	for scDeviceGroup, pvcRequests := range pvcRequestMap {
		sort.Slice(pvcRequests, func(i, j int) bool {
			return pvcRequests[i].request > pvcRequests[j].request
		})
		if configuration.CheckRawDeviceGroup(scDeviceGroup) {
			var allocatableList []int64
			for lvGroup, allocatable := range allocatableMap {
				if !strings.Contains(lvGroup, scDeviceGroup) {
					continue
				}
				allocatableList = append(allocatableList, allocatable)
			}
			for _, pvcR := range pvcRequests {
				index := minimumValueMinus(allocatableList, pvcR)
				if index < 0 {
					klog.V(3).Infof("mismatch pod: %s, node: %s, scDeviceGroup: %s", pod.Name, node.Node().Name, scDeviceGroup)
					return framework.NewStatus(framework.UnschedulableAndUnresolvable, "node storage resource insufficient")
				}
			}
		} else {
			var requestTotalBytes int64
			for _, pvcR := range pvcRequests {
				requestTotalBytes += pvcR.request
			}
			requestTotalGb := (requestTotalBytes-1)>>30 + 1
			if requestTotalGb > allocatableMap[scDeviceGroup] {
				klog.V(3).Infof("mismatch pod: %s, node: %s, request: %d, scDeviceGroup:%s, allocatable: %d", pod.Name, node.Node().Name, requestTotalGb, scDeviceGroup, allocatableMap[scDeviceGroup])
				return framework.NewStatus(framework.UnschedulableAndUnresolvable, "node storage resource insufficient")
			}
		}
	}

	klog.V(3).Infof("filter success pod: %s, node: %s", pod.Name, node.Node().Name)
	return framework.NewStatus(framework.Success, "")
}

// Score 对节点进行打分（相当于旧版本的 priorities）
func (ls *LocalStorage) Score(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	klog.V(3).Infof("score pod: %s, node: %s", pod.Name, nodeName)
	pvcRequestMap, node, useRaw, err := ls.getPvcRequestMap(pod)
	if err != nil {
		klog.V(3).ErrorS(err, "failed to get pvc/sc, pod: %s, node: %vs", pod.Name, nodeName)
		return 0, framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
	}

	if node == nodeName {
		return MaxScore, framework.NewStatus(framework.Success)
	}

	if len(pvcRequestMap) == 0 {
		return 5, framework.NewStatus(framework.Success, "")
	}

	allocatableMap, err := ls.getAllocatableMap(useRaw, pod.Name, nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
	}

	// 计算节点分数
	// 影响磁盘分数的有磁盘容量,磁盘上现有pv数量,磁盘IO
	// 在此我们以磁盘容量作为标准，同时配合配置文件中磁盘选择策略
	var scoref float64 = 0
	count := 0
	for scDeviceGroup, pvcRequests := range pvcRequestMap {
		var requestTotalBytes int64
		for _, pvcR := range pvcRequests {
			requestTotalBytes += pvcR.request
		}
		requestTotalGb := (requestTotalBytes-1)>>30 + 1

		var allocatableTotal int64
		if configuration.CheckRawDeviceGroup(scDeviceGroup) {
			for lvGroup, allocatable := range allocatableMap {
				if !strings.Contains(lvGroup, scDeviceGroup) {
					continue
				}
				allocatableTotal = allocatableTotal + allocatable
			}
		} else {
			allocatableTotal = allocatableMap[scDeviceGroup]
		}

		count++
		if configuration.SchedulerStrategy() == configuration.Schedulerspreadout {
			scoref += 1.0 - float64(requestTotalGb)/float64(allocatableTotal)
		}
		if configuration.SchedulerStrategy() == configuration.SchedulerBinpack {
			scoref += float64(requestTotalGb) / float64(allocatableTotal)
		}
	}

	score := int64(scoref / float64(count) * float64(MaxScore))

	klog.V(3).Infof("score pod: %s, node: %s, score: %d", pod.Name, nodeName, score)
	return score, framework.NewStatus(framework.Success)
}

// ScoreExtensions of the Score plugin.
func (ls *LocalStorage) ScoreExtensions() framework.ScoreExtensions {
	return nil
}

func (ls *LocalStorage) getPvcRequestMap(pod *v1.Pod) (map[string][]*pvcRequest, string, bool, error) {
	nodeName := ""
	pvcRequestMap := map[string][]*pvcRequest{}
	var useRaw, exclusive bool
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim == nil {
			continue
		}
		pvcName := vol.PersistentVolumeClaim.ClaimName

		pvc, err := ls.pvcLister.PersistentVolumeClaims(pod.Namespace).Get(pvcName)
		if err != nil {
			return pvcRequestMap, nodeName, useRaw, err
		}

		if pvc.Spec.StorageClassName == nil {
			continue
		}

		sc, err := ls.scLister.Get(*pvc.Spec.StorageClassName)
		if err != nil {
			return pvcRequestMap, nodeName, useRaw, err
		}
		if sc.Provisioner != carina.CSIPluginName {
			continue
		}

		// 如果存在某个pv已经绑定到某个节点，则新创建对pv只能在该节点创建
		if pvc.Status.Phase == v1.ClaimBound {
			pv, err := ls.pvLister.Get(pvc.Spec.VolumeName)
			if err != nil {
				return pvcRequestMap, nodeName, useRaw, err
			}
			if nodeName == "" {
				nodeName = pv.Spec.CSI.VolumeAttributes[carina.VolumeDeviceNode]
			} else if nodeName != pv.Spec.CSI.VolumeAttributes[carina.VolumeDeviceNode] {
				return pvcRequestMap, nodeName, useRaw, errors.New("pvc node clash")
			}
			continue
		}

		deviceGroup := sc.Parameters[carina.DeviceDiskKey]

		if configuration.CheckRawDeviceGroup(deviceGroup) {
			useRaw = true
		}

		// bcache device
		if deviceGroup == "" {
			deviceGroup = sc.Parameters[carina.VolumeBackendDiskType]
		}

		cacheGroup := sc.Parameters[carina.VolumeCacheDiskType]
		if cacheGroup != "" {
			cacheGroup = configuration.GetDeviceGroup(deviceGroup)
			cacheDiskRatio := sc.Parameters[carina.VolumeCacheDiskRatio]
			ratio, err := strconv.ParseInt(cacheDiskRatio, 10, 64)
			if err != nil {
				return pvcRequestMap, nodeName, useRaw, errors.New("carina.storage.io/cache-disk-ratio should be in 1-100")
			}
			if ratio < 1 || ratio >= 100 {
				return pvcRequestMap, nodeName, useRaw, errors.New("carina.storage.io/cache-disk-ratio should be in 1-100")
			}
			cacheRequestBytes := pvc.Spec.Resources.Requests.Storage().Value() * ratio / 100
			pvcRequestMap[cacheGroup] = append(pvcRequestMap[cacheGroup], &pvcRequest{false, cacheRequestBytes})
		}

		if deviceGroup == "" {
			return pvcRequestMap, nodeName, useRaw, errors.New("not set deviceGroup in storageClass " + sc.Name)
		} else {
			deviceGroup = configuration.GetDeviceGroup(deviceGroup)
		}
		if sc.Parameters[carina.ExclusivityDisk] == "true" {
			exclusive = true
		}
		pvcRequestMap[deviceGroup] = append(pvcRequestMap[cacheGroup], &pvcRequest{exclusive, pvc.Spec.Resources.Requests.Storage().Value()})
	}
	klog.V(3).Infof("pvcRequestMap: %v, node: %s, useRaw: %v", pvcRequestMap, nodeName, useRaw)
	return pvcRequestMap, nodeName, useRaw, nil
}

func (ls *LocalStorage) getAllocatableMap(useRaw bool, podName, nodeName string) (map[string]int64, error) {
	var lvExclusivityDisks []string
	var err error
	allocatableMap := map[string]int64{}
	if useRaw {
		lvExclusivityDisks, err = getLvExclusivityDisks(ls.dynamicClient, ls.lvLister, nodeName)
		if err != nil {
			klog.V(3).Infof("Failed to obtain node lvs, pod: %s node: %s, err: %s", podName, nodeName, err.Error())
			return allocatableMap, errors.New("failed to obtain node lvs, " + err.Error())
		}
	}

	nsr, err := getNodeStorageResource(ls.dynamicClient, ls.nsrLister, nodeName)
	if err != nil {
		klog.V(3).Infof("Failed to obtain node storages, pod: %s node: %s, err: %s", podName, nodeName, err.Error())
		return allocatableMap, errors.New("Failed to obtain node storages, " + err.Error())
	}

	for groupDetail, allocatable := range nsr.Status.Allocatable {
		if !strings.HasPrefix(groupDetail, carina.DeviceCapacityKeyPrefix) {
			continue
		}
		lvGroup := strings.TrimPrefix(groupDetail, carina.DeviceCapacityKeyPrefix)

		isRawDevice := configuration.CheckRawDeviceGroup(strings.Split(lvGroup, "/")[0])
		if isRawDevice {
			//skip exclusivityDisk
			if utils.ContainsString(lvExclusivityDisks, lvGroup) {
				continue
			}
			allocatableMap[lvGroup] = allocatable.Value()
		} else {
			allocatableMap[lvGroup] = allocatable.Value()
		}
	}
	klog.V(3).Infof("allocatableMap: %v", allocatableMap)

	if len(allocatableMap) == 0 {
		klog.V(3).Infof("can't get device allocatableMap, pod: %s, node: %s", podName, nodeName)
		return allocatableMap, errors.New("can't get device allocatableMap")
	}
	return allocatableMap, nil
}

// 在所有容量列表中，找到最低满足的值，并减去请求容量
// 循环便能判断该节点是否可满足所有pvc请求容量
func minimumValueMinus(array []int64, pvcR *pvcRequest) int {
	sort.Slice(array, func(i, j int) bool {
		return array[i] < array[j]
	})
	requestGb := (pvcR.request-1)>>30 + 1
	index := -1
	for i, a := range array {
		if a >= requestGb {
			index = i
			break
		}
	}
	if index < 0 {
		return index
	}
	if pvcR.exclusive {
		array[index] = 0
	} else {
		array[index] = array[index] - requestGb
	}

	return index
}
