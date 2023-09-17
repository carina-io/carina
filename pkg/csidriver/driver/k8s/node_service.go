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
	"sort"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/carina-io/carina"
	carinav1beta1 "github.com/carina-io/carina/api/v1beta1"
	"github.com/carina-io/carina/getter"
	"github.com/carina-io/carina/pkg/configuration"
	"github.com/carina-io/carina/pkg/csidriver/driver/util"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
)

// This annotation is present on K8s 1.11 release.
const annAlphaSelectedNode = "volume.alpha.kubernetes.io/selected-node"

// ErrNodeNotFound represents the error that node is not found.
var ErrNodeNotFound = errors.New("node not found")

// NodeService represents node service.
type NodeService struct {
	client.Client
	getter    *getter.RetryGetter
	lvService *LogicVolumeService
}

type groupPair struct {
	nodeName    string
	group       string
	allocatable int64
}

// +kubebuilder:rbac:groups=carina.storage.io,resources=NodeStorageResources,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch

// NewNodeService returns NodeService.
func NewNodeService(mgr manager.Manager, lvService *LogicVolumeService) *NodeService {
	return &NodeService{
		Client:    mgr.GetClient(),
		getter:    getter.NewRetryGetter(mgr),
		lvService: lvService,
	}
}

func (n NodeService) getLvExclusivityDisks(ctx context.Context, nodeName string) ([]string, error) {
	lvExclusivityDisks := []string{}
	lvs, err := n.lvService.GetLogicVolumesByNodeName(ctx, nodeName, true)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "can not get logic volumes for nodeName %s", nodeName)
	}
	for _, lv := range lvs {
		if lv.Annotations[carina.ExclusivityDisk] == "true" {
			lvExclusivityDisks = append(lvExclusivityDisks, lv.Spec.DeviceGroup)
		}
	}
	return lvExclusivityDisks, nil
}

func (n NodeService) getNodes(ctx context.Context, labels labels.Selector) (*corev1.NodeList, error) {
	nodeList := new(corev1.NodeList)
	var err error
	if labels == nil || labels.Empty() {
		err = n.List(ctx, nodeList)
	} else {
		err = n.List(ctx, nodeList, client.MatchingLabelsSelector{Selector: labels})
	}
	if err != nil {
		return nil, err
	}
	return nodeList, nil
}

func (n NodeService) HaveSelectedNode(ctx context.Context, namespace, name string) (string, error) {
	node := ""
	pvc := new(corev1.PersistentVolumeClaim)
	err := n.getter.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, pvc)
	if err != nil {
		return node, err
	}
	node = pvc.Annotations[carina.AnnSelectedNode]
	if node == "" {
		node = pvc.Annotations[annAlphaSelectedNode]
	}

	return node, nil
}

func (n NodeService) SelectDeviceGroup(ctx context.Context, requestGb int64, exclusivityDisk bool, nodeName, volumeType, scDeviceGroup string) (string, error) {
	if volumeType == carina.LvmVolumeType && scDeviceGroup != "" {
		return scDeviceGroup, nil
	}
	var preselectNode []groupPair
	var lvExclusivityDisks []string
	var err error

	nsr := new(carinav1beta1.NodeStorageResource)
	if err = n.getter.Get(ctx, client.ObjectKey{Name: nodeName}, nsr); err != nil {
		return "", err
	}

	if volumeType == carina.RawVolumeType {
		if lvExclusivityDisks, err = n.getLvExclusivityDisks(ctx, nodeName); err != nil {
			return "", err
		}
	}

	for groupDetail, allocatable := range nsr.Status.Allocatable {
		if allocatable.Value() < requestGb {
			continue
		}

		if !strings.HasPrefix(groupDetail, carina.DeviceCapacityKeyPrefix) {
			continue
		}
		group := strings.TrimPrefix(groupDetail, carina.DeviceCapacityKeyPrefix)
		isRawDevice := util.CheckRawDeviceGroup(strings.Split(group, "/")[0])

		if volumeType == carina.RawVolumeType && isRawDevice {
			if scDeviceGroup != "" && scDeviceGroup != strings.Split(group, "/")[0] {
				continue
			}

			//skip exclusivityDisk
			if utils.ContainsString(lvExclusivityDisks, group) {
				log.Infof("skip exclusivity disk: %s", group)
				continue
			}
			var existPartition bool
			//if it is an exclusive disk, filter the disks that have partitions
			for _, disk := range nsr.Status.Disks {
				if strings.Contains(group, disk.Name) && exclusivityDisk && len(disk.Partitions) > 1 {
					existPartition = true
					break
				}
			}
			if existPartition {
				continue
			}
			preselectNode = append(preselectNode, groupPair{
				group:       group,
				allocatable: allocatable.Value(),
			})
		}
		if volumeType == carina.LvmVolumeType && !isRawDevice {
			preselectNode = append(preselectNode, groupPair{
				group:       group,
				allocatable: allocatable.Value(),
			})
		}
	}

	log.Info("select device grouplist ", preselectNode)
	if len(preselectNode) < 1 {
		return "", ErrNodeNotFound
	}

	if len(preselectNode) == 1 {
		return preselectNode[0].group, nil
	}

	sort.Slice(preselectNode, func(i, j int) bool {
		return preselectNode[i].allocatable < preselectNode[j].allocatable
	})

	// 这里只能选最小满足的，因为可能存在一个pod多个pv都需要落在这个节点
	var selectDeviceGroup string
	if configuration.SchedulerStrategy() == configuration.SchedulerBinpack {
		selectDeviceGroup = preselectNode[0].group
	} else if configuration.SchedulerStrategy() == configuration.Schedulerspreadout {
		selectDeviceGroup = preselectNode[len(preselectNode)-1].group
	} else {
		return "", errors.New(fmt.Sprintf("Unsupported scheduling policies %s", configuration.SchedulerStrategy()))
	}

	return selectDeviceGroup, nil
}

func (n NodeService) SelectNode(ctx context.Context, requestGb int64, volumeType, scDeviceGroup string, requirement *csi.TopologyRequirement, exclusivityDisk bool) (string, string, error) {
	nodeList, err := n.getNodes(ctx, nil)
	if err != nil {
		return "", "", err
	}

	var preselectNode []groupPair

	for _, node := range nodeList.Items {
		if !readyNode(&node) {
			continue
		}

		// topology filter
		if !topologyMatchNodeLabels(node.Labels, requirement) {
			continue
		}

		// ensure nsr exist
		nsr := new(carinav1beta1.NodeStorageResource)
		if err := n.getter.Get(ctx, client.ObjectKey{Name: node.Name}, nsr); err != nil {
			continue
		}

		var lvExclusivityDisks []string
		if volumeType == carina.RawVolumeType {
			if lvExclusivityDisks, err = n.getLvExclusivityDisks(ctx, node.Name); err != nil {
				log.Warnf("Failed to get lv exclusivity disks for node %s, err: %s, ignore it", node.Name, err.Error())
				continue
			}
		}

		for groupDetail, allocatable := range nsr.Status.Allocatable {
			if allocatable.Value() < requestGb {
				continue
			}

			if !strings.HasPrefix(groupDetail, carina.DeviceCapacityKeyPrefix) {
				continue
			}
			group := strings.TrimPrefix(groupDetail, carina.DeviceCapacityKeyPrefix)
			if scDeviceGroup != "" && scDeviceGroup != strings.Split(group, "/")[0] {
				continue
			}
			isRawDevice := util.CheckRawDeviceGroup(strings.Split(group, "/")[0])

			if volumeType == carina.RawVolumeType && isRawDevice {
				//skip exclusivityDisk
				if utils.ContainsString(lvExclusivityDisks, group) {
					log.Infof("skip exclusivity disk: %s", group)
					continue
				}
				var existPartition bool
				//if it is an exclusive disk, filter the disks that have partitions
				for _, disk := range nsr.Status.Disks {
					if strings.Contains(group, disk.Name) && exclusivityDisk && len(disk.Partitions) > 1 {
						existPartition = true
						break
					}
				}
				if existPartition {
					continue
				}
				preselectNode = append(preselectNode, groupPair{
					nodeName:    node.Name,
					group:       group,
					allocatable: allocatable.Value(),
				})
			}
			if volumeType == carina.LvmVolumeType && !isRawDevice {
				preselectNode = append(preselectNode, groupPair{
					nodeName:    node.Name,
					group:       group,
					allocatable: allocatable.Value(),
				})
			}
		}
	}
	if len(preselectNode) < 1 {
		return "", "", ErrNodeNotFound
	}

	if len(preselectNode) == 1 {
		return preselectNode[0].nodeName, preselectNode[0].group, nil
	}

	sort.Slice(preselectNode, func(i, j int) bool {
		return preselectNode[i].allocatable < preselectNode[j].allocatable
	})

	// 根据配置文件中设置算法进行节点选择
	var nodeName, selectDeviceGroup string
	if configuration.SchedulerStrategy() == configuration.SchedulerBinpack {
		nodeName = preselectNode[0].nodeName
		selectDeviceGroup = preselectNode[0].group
	} else if configuration.SchedulerStrategy() == configuration.Schedulerspreadout {
		nodeName = preselectNode[len(preselectNode)-1].nodeName
		selectDeviceGroup = preselectNode[len(preselectNode)-1].group
	} else {
		return "", "", errors.New(fmt.Sprintf("Unsupported scheduling policies %s", configuration.SchedulerStrategy()))
	}

	return nodeName, selectDeviceGroup, nil
}

// GetCapacityByNodeName returns capacity of specified node by name.
func (n NodeService) GetCapacityByNodeName(ctx context.Context, nodeName, lvDeviceGroup string) (int64, error) {
	nsr := new(carinav1beta1.NodeStorageResource)
	err := n.getter.Get(ctx, client.ObjectKey{Name: nodeName}, nsr)
	if err != nil {
		return 0, err
	}

	for groupDetail, allocatable := range nsr.Status.Allocatable {
		if groupDetail == carina.DeviceCapacityKeyPrefix+lvDeviceGroup {
			return allocatable.Value(), nil
		}
	}
	return 0, errors.New("device group not found")
}

// GetTotalCapacity returns total capacity of all nodes.
func (n NodeService) GetTotalCapacity(ctx context.Context, scDeviceGroup string, topology *csi.Topology, exclusivityDisk bool) (int64, error) {
	var nodeLabels labels.Selector
	if topology != nil && len(topology.GetSegments()) != 0 {
		nodeLabels = labels.SelectorFromSet(topology.GetSegments())
	}

	nodes, err := n.getNodes(ctx, nodeLabels)
	if err != nil {
		return 0, err
	}

	var volumeType string
	if util.CheckRawDeviceGroup(scDeviceGroup) {
		volumeType = carina.RawVolumeType
	} else {
		volumeType = carina.LvmVolumeType
	}

	capacity := int64(0)
	for _, node := range nodes.Items {
		if !readyNode(&node) {
			continue
		}

		// ensure nsr exist
		nsr := new(carinav1beta1.NodeStorageResource)
		if err := n.getter.Get(ctx, client.ObjectKey{Name: node.Name}, nsr); err != nil {
			continue
		}

		var lvExclusivityDisks []string
		if volumeType == carina.RawVolumeType {
			if lvExclusivityDisks, err = n.getLvExclusivityDisks(ctx, node.Name); err != nil {
				log.Warnf("Failed to get lv exclusivity disks for node %s, err: %s, ignore it", node.Name, err.Error())
				continue
			}
		}

		for groupDetail, allocatable := range nsr.Status.Capacity {
			if !strings.HasPrefix(groupDetail, carina.DeviceCapacityKeyPrefix) {
				continue
			}
			group := strings.TrimPrefix(groupDetail, carina.DeviceCapacityKeyPrefix)
			if scDeviceGroup != "" && scDeviceGroup != strings.Split(group, "/")[0] {
				continue
			}
			isRawDevice := util.CheckRawDeviceGroup(strings.Split(group, "/")[0])

			if volumeType == carina.RawVolumeType && isRawDevice {
				//skip exclusivityDisk
				if utils.ContainsString(lvExclusivityDisks, group) {
					log.Infof("skip exclusivity disk: %s", group)
					continue
				}
				var existPartition bool
				//if it is an exclusive disk, filter the disks that have partitions
				for _, disk := range nsr.Status.Disks {
					if strings.Contains(group, disk.Name) && exclusivityDisk && len(disk.Partitions) > 1 {
						existPartition = true
						break
					}
				}
				if existPartition {
					continue
				}

				capacity += allocatable.Value()
			}
			if volumeType == carina.LvmVolumeType && !isRawDevice {
				capacity += allocatable.Value()
			}
		}
	}
	return capacity, nil
}

func (n NodeService) SelectMultiVolumeNode(ctx context.Context, backendDeviceGroup, cacheDeviceGroup string, backendRequestGb, cacheRequestGb int64, requirement *csi.TopologyRequirement) (string, error) {
	nodeList, err := n.getNodes(ctx, nil)
	if err != nil {
		return "", err
	}

	var preselectNode []groupPair

	for _, node := range nodeList.Items {
		// topology selector
		if !topologyMatchNodeLabels(node.Labels, requirement) {
			continue
		}

		// ensure nsr exist
		nsr := new(carinav1beta1.NodeStorageResource)
		if err := n.getter.Get(ctx, client.ObjectKey{Name: node.Name}, nsr); err != nil {
			continue
		}

		var cacheFit bool
		// TODO: consider exclusive raw disk
		for groupDetail, allocatable := range nsr.Status.Allocatable {
			if !strings.HasPrefix(groupDetail, carina.DeviceCapacityKeyPrefix) || !strings.Contains(groupDetail, cacheDeviceGroup) {
				continue
			}
			if allocatable.Value() >= cacheRequestGb {
				cacheFit = true
				break
			}
		}
		if !cacheFit {
			continue
		}
		for groupDetail, allocatable := range nsr.Status.Allocatable {
			if !strings.HasPrefix(groupDetail, carina.DeviceCapacityKeyPrefix) || !strings.Contains(groupDetail, backendDeviceGroup) {
				continue
			}
			if allocatable.Value() < backendRequestGb {
				continue
			}
			preselectNode = append(preselectNode, groupPair{
				nodeName:    node.Name,
				allocatable: allocatable.Value(),
			})
		}

	}

	if len(preselectNode) < 1 {
		return "", ErrNodeNotFound
	}

	sort.Slice(preselectNode, func(i, j int) bool {
		return preselectNode[i].allocatable < preselectNode[j].allocatable
	})

	// 根据配置文件中设置算法进行节点选择
	var nodeName string
	if configuration.SchedulerStrategy() == configuration.SchedulerBinpack {
		nodeName = preselectNode[0].nodeName
	} else if configuration.SchedulerStrategy() == configuration.Schedulerspreadout {
		nodeName = preselectNode[len(preselectNode)-1].nodeName
	} else {
		return "", errors.New(fmt.Sprintf("Unsupported scheduling policies %s", configuration.SchedulerStrategy()))
	}

	return nodeName, nil
}

func readyNode(node *corev1.Node) bool {
	for _, s := range node.Status.Conditions {
		if s.Type == corev1.NodeReady && s.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func topologyMatchNodeLabels(nodeLabels map[string]string, requirement *csi.TopologyRequirement) bool {
	if nodeLabels == nil || requirement == nil {
		return false
	}
	// topology filter
	topologyMatch := false
	for _, topo := range requirement.GetPreferred() {
		if labels.SelectorFromSet(topo.GetSegments()).Matches(labels.Set(nodeLabels)) {
			topologyMatch = true
			break
		}
	}
	if !topologyMatch {
		for _, topo := range requirement.GetRequisite() {
			if labels.SelectorFromSet(topo.GetSegments()).Matches(labels.Set(nodeLabels)) {
				topologyMatch = true
				break
			}
		}
	}

	return topologyMatch
}
