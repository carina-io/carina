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
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/anuvu/disko"
	"github.com/carina-io/carina/api"
	carinav1beta1 "github.com/carina-io/carina/api/v1beta1"
	"github.com/carina-io/carina/utils/log"

	"github.com/carina-io/carina/pkg/configuration"
	"github.com/carina-io/carina/pkg/version"
	"github.com/carina-io/carina/utils"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/rand"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// This annotation is present on K8s 1.11 release.
const annAlphaSelectedNode = "volume.alpha.kubernetes.io/selected-node"

type nodeService interface {
	getNodes(ctx context.Context) (*corev1.NodeList, error)
	// getNodeStorageResources
	getNodeStorageResources(ctx context.Context) (map[string]carinav1beta1.NodeStorageResourceStatus, error)
	// SelectVolumeNode 支持 volume size 及 topology match
	SelectVolumeNode(ctx context.Context, request int64, deviceGroup string, requirement *csi.TopologyRequirement) (string, string, map[string]string, error)
	GetCapacityByNodeName(ctx context.Context, nodeName, deviceGroup string) (int64, error)
	GetTotalCapacity(ctx context.Context, deviceGroup string, topology *csi.Topology) (int64, error)
	SelectDeviceGroup(ctx context.Context, request int64, nodeName string) (string, error)
	// HaveSelectedNode sc WaitForConsumer
	HaveSelectedNode(ctx context.Context, namespace, name string) (string, error)

	// SelectMultiVolumeNode multi volume node select
	SelectMultiVolumeNode(ctx context.Context, backendDeviceGroup, cacheDeviceGroup string, backendRequestGb, cacheRequestGb int64, requirement *csi.TopologyRequirement) (string, map[string]string, error)
}

// ErrNodeNotFound represents the error that node is not found.
var ErrNodeNotFound = errors.New("node not found")

// NodeService represents node service.
type NodeService struct {
	client.Client
}

// +kubebuilder:rbac:groups=carina.storage.io,resources=NodeStorageResources,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// NewNodeService returns NodeService.
func NewNodeService(mgr manager.Manager) *NodeService {

	ctx := context.Background()
	err := mgr.GetFieldIndexer().IndexField(ctx, &carinav1beta1.NodeStorageResource{}, "spec.nodeName",
		func(o client.Object) []string {
			return []string{o.(*carinav1beta1.NodeStorageResource).Spec.NodeName}
		})
	if err != nil {
		return nil
	}

	return &NodeService{Client: mgr.GetClient()}
}

func (s NodeService) getNodes(ctx context.Context) (*corev1.NodeList, error) {
	nl := new(corev1.NodeList)
	err := s.List(ctx, nl)
	if err != nil {
		return nil, err
	}
	return nl, nil
}

func (s NodeService) getNodeStorageResources(ctx context.Context) (map[string]carinav1beta1.NodeStorageResourceStatus, error) {
	readyNSR := map[string]carinav1beta1.NodeStorageResourceStatus{}

	nl := new(corev1.NodeList)
	err := s.List(ctx, nl)
	if err != nil {
		return readyNSR, err
	}
	// only ready nodes are returned
	readyNode := map[string]uint8{}
	for _, n := range nl.Items {
		for _, s := range n.Status.Conditions {
			if s.Type == corev1.NodeReady && s.Status == corev1.ConditionTrue {
				readyNode[n.Name] = 1
			}
		}
	}
	nsr := new(carinav1beta1.NodeStorageResourceList)
	err = s.Client.List(ctx, nsr)
	if err != nil {
		return readyNSR, err
	}
	for _, sr := range nsr.Items {
		if _, exists := readyNode[sr.Spec.NodeName]; exists {
			readyNSR[sr.Spec.NodeName] = sr.Status
		}
	}
	return readyNSR, nil
}

func (s NodeService) SelectVolumeNode(ctx context.Context, requestGb int64, deviceGroup string, requirement *csi.TopologyRequirement) (string, string, map[string]string, error) {
	// 在并发场景下，兼顾调度效率与调度公平，将pv分配到不同时间段
	time.Sleep(time.Duration(rand.Int63nRange(1, 30)) * time.Second)

	var nodeName, selectDeviceGroup string
	segments := map[string]string{}
	nl, err := s.getNodes(ctx)
	if err != nil {
		return "", "", segments, err
	}

	nsr, err := s.getNodeStorageResources(ctx)
	if err != nil {
		return "", "", segments, err
	}

	type pairs struct {
		Key   string
		Value int64
	}

	preselectNode := []pairs{}

	for _, node := range nl.Items {

		// topology selector
		// 若是sc配置了allowedTopologies，在此过滤出符合条件的node
		if requirement != nil {
			topologySelector := false
			for _, topo := range requirement.GetRequisite() {
				selector := labels.SelectorFromSet(topo.GetSegments())
				if selector.Matches(labels.Set(node.Labels)) {
					topologySelector = true
					break
				}
			}
			// 如果没有通过topology selector则节点不可用
			if !topologySelector {
				continue
			}
		}
		// Only ready nodes exist
		status, exists := nsr[node.Name]
		if !exists {
			continue
		}

		// capacity selector
		// 注册设备时有特殊前缀的，若是sc指定了设备组则过滤出所有节点上符合条件的设备组
		for key, value := range status.Allocatable {
			if strings.HasPrefix(key, utils.DeviceCapacityKeyPrefix) {
				if deviceGroup != "" && key != deviceGroup && key != utils.DeviceCapacityKeyPrefix+deviceGroup {
					continue
				}
				if value.Value() < requestGb {
					continue
				}
				//skip raw
				if version.CheckRawDeviceGroup(strings.Split(key, "/")[1]) {
					continue
				}
				preselectNode = append(preselectNode, pairs{
					Key:   node.Name + "-*-" + key,
					Value: value.Value(),
				})
			}
		}
	}
	if len(preselectNode) < 1 {
		return "", "", segments, ErrNodeNotFound
	}

	sort.Slice(preselectNode, func(i, j int) bool {
		return preselectNode[i].Value < preselectNode[j].Value
	})

	// 根据配置文件中设置算法进行节点选择
	if configuration.SchedulerStrategy() == configuration.SchedulerBinpack {
		nodeName = strings.Split(preselectNode[0].Key, "-*-")[0]
		selectDeviceGroup = strings.Split(preselectNode[0].Key, "/")[1]
	} else if configuration.SchedulerStrategy() == configuration.Schedulerspreadout {
		nodeName = strings.Split(preselectNode[len(preselectNode)-1].Key, "-*-")[0]
		selectDeviceGroup = strings.Split(preselectNode[len(preselectNode)-1].Key, "/")[1]
	} else {
		return "", "", segments, errors.New(fmt.Sprintf("Unsupported scheduling policies %s", configuration.SchedulerStrategy()))
	}

	// 获取选择节点的label
	for _, node := range nl.Items {
		if node.Name == nodeName {
			for _, topo := range requirement.GetRequisite() {
				for k, _ := range topo.GetSegments() {
					segments[k] = node.Labels[k]
				}
			}
		}
	}

	return nodeName, selectDeviceGroup, segments, nil
}

// GetCapacityByNodeName returns VG capacity of specified node by name.
func (s NodeService) GetCapacityByNodeName(ctx context.Context, name, deviceGroup string) (int64, error) {

	nsr := new(carinav1beta1.NodeStorageResource)
	err := s.Get(ctx, client.ObjectKey{Name: name}, nsr)
	if err != nil {
		return 0, err
	}

	for key, v := range nsr.Status.Allocatable {
		if key == deviceGroup || key == utils.DeviceCapacityKeyPrefix+deviceGroup {
			return v.Value(), nil
		}
	}
	return 0, errors.New("device group not found")
}

// GetTotalCapacity returns total VG capacity of all nodes.
func (s NodeService) GetTotalCapacity(ctx context.Context, deviceGroup string, topology *csi.Topology) (int64, error) {

	nl, err := s.getNodes(ctx)
	if err != nil {
		return 0, err
	}

	nsr, err := s.getNodeStorageResources(ctx)
	if err != nil {
		return 0, err
	}
	var volumeType string
	if version.CheckRawDeviceGroup(deviceGroup) {
		volumeType = utils.RawVolumeType
	} else {
		volumeType = utils.LvmVolumeType
	}

	capacity := int64(0)
	for _, node := range nl.Items {
		// topology selector
		if topology != nil {
			selector := labels.SelectorFromSet(topology.GetSegments())
			if !selector.Matches(labels.Set(node.Labels)) {
				continue
			}
		}

		// Only ready nodes exist
		status, exists := nsr[node.Name]
		if !exists {
			continue
		}

		for key, v := range status.Capacity {

			if deviceGroup == "" && strings.HasPrefix(key, utils.DeviceCapacityKeyPrefix) {
				strArr := strings.Split(key, "/")
				if volumeType == utils.RawVolumeType {
					if version.CheckRawDeviceGroup(strArr[1]) {
						capacity += v.Value()
					}

				}
				if volumeType == utils.LvmVolumeType {
					if version.CheckRawDeviceGroup(strArr[1]) {
						capacity += v.Value()
					}
				}

			} else if key == deviceGroup || key == utils.DeviceCapacityKeyPrefix+deviceGroup {
				strArr := strings.Split(key, "/")

				if volumeType == utils.RawVolumeType {
					if version.CheckRawDeviceGroup(strArr[1]) {
						capacity += v.Value()
					}

				}
				if volumeType == utils.LvmVolumeType {
					if version.CheckRawDeviceGroup(strArr[1]) {
						capacity += v.Value()
					}
				}
			}
		}
	}
	return capacity, nil
}

func (s NodeService) SelectDeviceGroup(ctx context.Context, request int64, nodeName string, volumeType string, exclusivityDisk bool) (string, error) {
	var selectDeviceGroup string

	nl, err := s.getNodes(ctx)
	if err != nil {
		return "", err
	}

	nsr, err := s.getNodeStorageResources(ctx)
	if err != nil {
		return "", err
	}

	type pairs struct {
		Key   string
		Value int64
	}

	preselectNode := []pairs{}

	for _, node := range nl.Items {
		if nodeName != node.Name {
			continue
		}
		status, exists := nsr[node.Name]
		if !exists {
			continue
		}
		// capacity selector
		// 经过上层过滤，这里只会有一个节点
		for key, value := range status.Allocatable {
			if strings.HasPrefix(key, utils.DeviceCapacityKeyPrefix) {
				strArr := strings.Split(key, "/")
				if volumeType == utils.RawVolumeType {
					if version.CheckRawDeviceGroup(strArr[1]) {

						device := disko.Disk{}
						utils.Fill(status.Disks, &device)
						//check freespace size
						if len(device.FreeSpacesWithMin(uint64(request))) < 1 {
							continue
						}

						//如果是独占磁盘，筛选没有分区的磁盘
						if exclusivityDisk && len(device.Partitions) > 1 {
							continue
						}
						preselectNode = append(preselectNode, pairs{
							Key:   key,
							Value: value.Value(),
						})
					}

				}
				if volumeType == utils.LvmVolumeType {
					if version.CheckRawDeviceGroup(strArr[1]) {
						preselectNode = append(preselectNode, pairs{
							Key:   key,
							Value: value.Value(),
						})
					}
				}

			}
		}
	}
	if len(preselectNode) < 1 {
		return "", ErrNodeNotFound
	}

	sort.Slice(preselectNode, func(i, j int) bool {
		return preselectNode[i].Value < preselectNode[j].Value
	})
	log.Info("select device grouplist ", preselectNode)
	// 这里只能选最小满足的，因为可能存在一个pod多个pv都需要落在这个节点
	for _, p := range preselectNode {
		if p.Value >= request {
			if volumeType == utils.LvmVolumeType {
				selectDeviceGroup = strings.Split(p.Key, "/")[1]
			}
			if volumeType == utils.RawVolumeType {
				selectDeviceGroup = strings.Split(p.Key, "/")[1] + "/" + strings.Split(p.Key, "/")[2]
			}

		}
	}
	return selectDeviceGroup, nil
}

func (s NodeService) HaveSelectedNode(ctx context.Context, namespace, name string) (string, error) {
	node := ""
	pvc := new(corev1.PersistentVolumeClaim)
	err := s.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, pvc)
	if err != nil {
		return node, err
	}
	node = pvc.Annotations[utils.AnnSelectedNode]
	if node == "" {
		node = pvc.Annotations[annAlphaSelectedNode]
	}

	return node, nil
}

func (s NodeService) SelectMultiVolumeNode(ctx context.Context, backendDeviceGroup, cacheDeviceGroup string, backendRequestGb, cacheRequestGb int64, requirement *csi.TopologyRequirement) (string, map[string]string, error) {
	// 在并发场景下，兼顾调度效率与调度公平，将pv分配到不同时间段
	time.Sleep(time.Duration(rand.Int63nRange(1, 30)) * time.Second)

	var nodeName string
	segments := map[string]string{}
	nl, err := s.getNodes(ctx)
	if err != nil {
		return "", segments, err
	}

	nsr, err := s.getNodeStorageResources(ctx)
	if err != nil {
		return "", segments, err
	}

	type pairs struct {
		Key   string
		Value int64
	}

	preselectNode := []pairs{}

	for _, node := range nl.Items {

		// topology selector
		// 若是sc配置了allowedTopologies，在此过滤出符合条件的node
		if requirement != nil {
			topologySelector := false
			for _, topo := range requirement.GetRequisite() {
				selector := labels.SelectorFromSet(topo.GetSegments())
				if selector.Matches(labels.Set(node.Labels)) {
					topologySelector = true
					break
				}
			}
			// 如果没有通过topology selector则节点不可用
			if !topologySelector {
				continue
			}
		}

		status, exists := nsr[node.Name]
		if !exists {
			continue
		}

		// capacity selector
		// 注册设备时有特殊前缀的，若是sc指定了设备组则过滤出所有节点上符合条件的设备组
		backendFilter := int64(0)
		cacheFilter := int64(0)
		for key, value := range status.Allocatable {
			if strings.HasPrefix(key, utils.DeviceCapacityKeyPrefix) {
				if strings.Contains(key, backendDeviceGroup) {
					if value.Value() >= backendRequestGb {
						backendFilter = value.Value()
					}
				}
				if strings.Contains(key, cacheDeviceGroup) {
					if value.Value() >= cacheRequestGb {
						cacheFilter = value.Value()
					}
				}
			}
		}

		if backendFilter > 0 && cacheFilter >= 0 {
			preselectNode = append(preselectNode, pairs{
				Key:   node.Name,
				Value: backendFilter,
			})
		}
	}
	if len(preselectNode) < 1 {
		return "", segments, ErrNodeNotFound
	}

	sort.Slice(preselectNode, func(i, j int) bool {
		return preselectNode[i].Value < preselectNode[j].Value
	})

	// 根据配置文件中设置算法进行节点选择
	if configuration.SchedulerStrategy() == configuration.SchedulerBinpack {
		nodeName = preselectNode[0].Key
	} else if configuration.SchedulerStrategy() == configuration.Schedulerspreadout {
		nodeName = preselectNode[len(preselectNode)-1].Key
	} else {
		return "", segments, errors.New(fmt.Sprintf("no support scheduler strategy %s", configuration.SchedulerStrategy()))
	}

	// 获取选择节点的label
	for _, node := range nl.Items {
		if node.Name == nodeName {
			for _, topo := range requirement.GetRequisite() {
				for k, _ := range topo.GetSegments() {
					segments[k] = node.Labels[k]
				}
			}
		}
	}

	return nodeName, segments, nil
}

//In the case of bare disk, it is preferential to match the partitioned disk, and if there is no one, then match the raw disk without partition
func (s NodeService) SelectDeviceNode(ctx context.Context, requestGb int64, deviceGroup string, requirement *csi.TopologyRequirement, exclusivityDisk bool) (string, string, map[string]string, error) {
	// Locate the disk that matches the current group
	//re := configuration.GetRawDeviceGroupRe(deviceGroup)
	time.Sleep(time.Duration(rand.Int63nRange(1, 30)) * time.Second)
	var nodeName, selectDeviceGroup string
	segments := map[string]string{}
	type paris struct {
		Key   string
		Value int64
	}
	preselectNode := []paris{}
	// ready nodestorage
	nsr, err := s.getNodeStorageResources(ctx)
	if err != nil {
		return "", "", segments, err
	}

	nl, err := s.getNodes(ctx)
	if err != nil {
		return "", "", segments, err
	}
	for _, node := range nl.Items {

		// topology selector
		// 若是sc配置了allowedTopologies，在此过滤出符合条件的node
		if requirement != nil {
			topologySelector := false
			for _, topo := range requirement.GetRequisite() {
				selector := labels.SelectorFromSet(topo.GetSegments())
				if selector.Matches(labels.Set(node.Labels)) {
					topologySelector = true
					break
				}
			}
			// 如果没有通过topology selector则节点不可用
			if !topologySelector {
				continue
			}
		}
		// Only ready nodes exist
		status, exists := nsr[node.Name]
		if !exists {
			continue
		}

		// capacity selector
		// 注册设备时有特殊前缀的，若是sc指定了设备组则过滤出所有节点上符合条件的设备组
		for key, value := range status.Allocatable {
			if strings.HasPrefix(key, utils.DeviceCapacityKeyPrefix) {
				if deviceGroup != "" && key != deviceGroup && key != utils.DeviceCapacityKeyPrefix+deviceGroup {
					continue
				}
				if value.Value() < requestGb {
					continue
				}
				//skip lvm
				if !version.CheckRawDeviceGroup(strings.Split(key, "/")[1]) {
					continue
				}
				//value值是磁盘可用分区最大容量，匹配节点磁盘分区满足后，创建分区基于磁盘可用空间最小满足创建
				device := disko.Disk{}
				utils.Fill(status.Disks, &device)
				//check freespace size
				if len(device.FreeSpacesWithMin(uint64(requestGb))) < 1 {
					continue
				}

				//如果是独占磁盘，筛选没有分区的磁盘
				if exclusivityDisk && len(device.Partitions) > 1 {
					continue
				}

				preselectNode = append(preselectNode, paris{
					Key:   node.Name + "-*-" + key,
					Value: value.Value(),
				})

			}
		}
	}

	if len(preselectNode) < 1 {
		return "", "", segments, ErrNodeNotFound
	}

	sort.Slice(preselectNode, func(i, j int) bool {
		return preselectNode[i].Value < preselectNode[j].Value
	})

	//根据配置文件中设置算法进行节点选择
	if configuration.SchedulerStrategy() == configuration.SchedulerBinpack {
		nodeName = strings.Split(preselectNode[0].Key, "-*-")[0]
		selectDeviceGroup = strings.Split(preselectNode[0].Key, "-*-")[1]
	} else if configuration.SchedulerStrategy() == configuration.Schedulerspreadout {
		nodeName = strings.Split(preselectNode[len(preselectNode)-1].Key, "-*-")[0]
		selectDeviceGroup = strings.Split(preselectNode[len(preselectNode)-1].Key, "-*-")[1]
	} else {
		return "", "", segments, fmt.Errorf("no support scheduler strategy %s", configuration.SchedulerStrategy())
	}

	node := new(corev1.Node)
	err = s.Get(ctx, client.ObjectKey{Name: nodeName}, node)
	if err != nil {
		log.Error(err, "unable get node ")
		return "", "", segments, err
	}

	// 获取选择节点的label
	for _, topo := range requirement.GetRequisite() {
		for k, _ := range topo.GetSegments() {
			segments[k] = node.Labels[k]
		}
	}

	return nodeName, selectDeviceGroup, segments, nil
}

func (s NodeService) selectParttionOrRaw(disks []api.Disk, requestGb int64, exclusivityDisk bool, re []string) (deviceName disko.Disk, free disko.FreeSpace, err error) {
	avaiableRawdevices := []disko.Disk{}
	avaiablePartitionDevices := []disko.Disk{}
	for _, tmp := range disks {

		device := disko.Disk{}
		utils.Fill(tmp, &device)
		diskSelector, err := regexp.Compile(strings.Join(re, "|"))
		if err != nil {
			log.Warnf("disk regex %s error %v ", strings.Join(re, "|"), err)
			return deviceName, free, err
		}
		if !diskSelector.MatchString(device.Name) {
			continue
		}

		if len(device.FreeSpacesWithMin(uint64(requestGb))) < 1 {
			continue
		}

		//如果是独占磁盘，筛选没有分区的磁盘，满足不同的调度策略
		if exclusivityDisk {
			if len(device.Partitions) > 1 {
				continue
			}
			avaiableRawdevices = append(avaiableRawdevices, device)
			continue
		}

		//如果不是独占磁盘，优先筛选有分区的磁盘去满足调度策略，如果没有匹配则筛选空白磁盘满足调度策略
		if len(device.Partitions) > 1 {
			//有满足有分区的可用磁盘
			avaiablePartitionDevices = append(avaiablePartitionDevices, device)
			continue
		}
		//空白磁盘
		avaiableRawdevices = append(avaiableRawdevices, device)
	}

	switch {
	case exclusivityDisk == true && len(avaiableRawdevices) < 1:
		log.Error(err, " has not match avaiable device for  exclusivity pod")
		return deviceName, free, errors.New(fmt.Sprintf("no support exclusivity scheduler strategy %s", configuration.SchedulerStrategy()))
	case (exclusivityDisk == true && len(avaiableRawdevices) > 1), (exclusivityDisk == false && len(avaiablePartitionDevices) < 1 && len(avaiableRawdevices) >= 1):
		//有可用独占磁盘使用无分区裸盘||无可用共享磁盘使用无分区裸盘
		sort.Slice(avaiableRawdevices, func(i, j int) bool {
			freei := avaiableRawdevices[i].FreeSpacesWithMin(uint64(requestGb))
			freej := avaiableRawdevices[j].FreeSpacesWithMin(uint64(requestGb))
			sort.Slice(freei, func(a, b int) bool {
				return freei[a].Size() < freei[b].Size()
			})
			sort.Slice(freej, func(a, b int) bool {
				return freej[a].Size() < freej[b].Size()
			})

			return freei[0].Size() < freej[0].Size()
		})
		// 根据配置文件中设置算法进行节点选择最小适配还是最大优选
		if configuration.SchedulerStrategy() == configuration.SchedulerBinpack {
			deviceName = avaiableRawdevices[0]
			free = avaiableRawdevices[0].FreeSpacesWithMin(uint64(requestGb))[0]
		} else if configuration.SchedulerStrategy() == configuration.Schedulerspreadout {
			deviceName = avaiableRawdevices[len(avaiableRawdevices)-1]
			free = avaiableRawdevices[0].FreeSpacesWithMin(uint64(requestGb))[len(avaiableRawdevices[0].FreeSpacesWithMin(uint64(requestGb)))-1]
		} else {
			deviceName = avaiableRawdevices[0]
			free = avaiableRawdevices[0].FreeSpacesWithMin(uint64(requestGb))[0]
		}
		return deviceName, free, nil

	case exclusivityDisk == false && len(avaiablePartitionDevices) < 1 && len(avaiableRawdevices) < 1:
		//无可用共享磁盘
		log.Error(err, " has not match avaiable device for  pod ")
		return deviceName, free, errors.New(fmt.Sprintf("no support  scheduler strategy %s", configuration.SchedulerStrategy()))

	case exclusivityDisk == false && len(avaiablePartitionDevices) > 1:
		//有可用共享磁盘使用有分区磁盘；
		sort.Slice(avaiablePartitionDevices, func(i, j int) bool {
			freei := avaiablePartitionDevices[i].FreeSpacesWithMin(uint64(requestGb))
			freej := avaiablePartitionDevices[j].FreeSpacesWithMin(uint64(requestGb))
			sort.Slice(freei, func(a, b int) bool {
				return freei[a].Size() < freei[b].Size()
			})
			sort.Slice(freej, func(a, b int) bool {
				return freej[a].Size() < freej[b].Size()
			})

			return freei[0].Size() < freej[0].Size()
		})
		if configuration.SchedulerStrategy() == configuration.SchedulerBinpack {
			deviceName = avaiablePartitionDevices[0]
			free = avaiablePartitionDevices[0].FreeSpacesWithMin(uint64(requestGb))[0]
		} else if configuration.SchedulerStrategy() == configuration.Schedulerspreadout {
			deviceName = avaiablePartitionDevices[len(avaiablePartitionDevices)-1]
			free = avaiablePartitionDevices[len(avaiablePartitionDevices)-1].FreeSpacesWithMin(uint64(requestGb))[len(avaiablePartitionDevices[len(avaiablePartitionDevices)-1].FreeSpacesWithMin(uint64(requestGb)))-1]
		} else {
			deviceName = avaiablePartitionDevices[0]
			free = avaiablePartitionDevices[0].FreeSpacesWithMin(uint64(requestGb))[0]
		}
		return deviceName, free, nil
	default:
		log.Error(err, " has not match avaiable device")
		return deviceName, free, fmt.Errorf("has not match avaiable device %s", configuration.SchedulerStrategy())
	}

}
