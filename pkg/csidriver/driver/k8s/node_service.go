package k8s

import (
	"carina/pkg/configruation"
	"carina/pkg/csidriver/csi"
	"carina/utils"
	"context"
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type nodeService interface {
	getNodes(ctx context.Context) (*corev1.NodeList, error)
	// 支持 volume size 及 topology match
	SelectVolumeNode(ctx context.Context, request int64, deviceGroup string, requirement *csi.TopologyRequirement) (string, string, map[string]string, error)
	GetCapacityByNodeName(ctx context.Context, nodeName, deviceGroup string) (int64, error)
	GetTotalCapacity(ctx context.Context, deviceGroup string) (int64, error)
	GetCapacityByTopologyLabel(ctx context.Context, topology, deviceGroup string) (int64, error)
}

// ErrNodeNotFound represents the error that node is not found.
var ErrNodeNotFound = errors.New("node not found")

// NodeService represents node service.
type NodeService struct {
	client.Client
}

// NewNodeService returns NodeService.
func NewNodeService(mgr manager.Manager) *NodeService {
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

func (s NodeService) SelectVolumeNode(ctx context.Context, requestGb int64, deviceGroup string, requirement *csi.TopologyRequirement) (string, string, map[string]string, error) {
	var nodeName, selectDeviceGroup string
	segments := map[string]string{}
	nl, err := s.getNodes(ctx)
	if err != nil {
		return "", "", segments, err
	}

	type paris struct {
		Key   string
		Value int64
	}

	preselectNode := []paris{}

	for _, node := range nl.Items {

		// topology selector
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

		// capacity selector
		for key, value := range node.Status.Allocatable {

			if strings.HasPrefix(string(key), utils.DeviceCapacityKeyPrefix) {
				if deviceGroup != "" && string(key) != deviceGroup && string(key) != utils.DeviceCapacityKeyPrefix+deviceGroup {
					continue
				}
				if value.Value() < requestGb {
					continue
				}
				preselectNode = append(preselectNode, paris{
					Key:   node.Name + "-" + string(key),
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

	if configruation.SchedulerStrategy() == configruation.SchedulerBinpack {
		nodeName = strings.Split(preselectNode[0].Key, "-")[0]
		selectDeviceGroup = strings.Split(preselectNode[0].Key, "/")[1]
	} else if configruation.SchedulerStrategy() == configruation.SchedulerSpradout {
		nodeName = strings.Split(preselectNode[len(preselectNode)-1].Key, "-")[0]
		selectDeviceGroup = strings.Split(preselectNode[0].Key, "/")[1]
	} else {
		return "", "", segments, errors.New(fmt.Sprintf("no support scheduler strategy %s", configruation.SchedulerStrategy()))
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
	node := new(corev1.Node)
	err := s.Get(ctx, client.ObjectKey{Name: name}, node)
	if err != nil {
		return 0, err
	}

	for key, v := range node.Status.Allocatable {
		if string(key) == deviceGroup || string(key) == utils.DeviceCapacityKeyPrefix+deviceGroup {
			return v.Value(), nil
		}
	}
	return 0, errors.New("device group not found")
}

// GetTotalCapacity returns total VG capacity of all nodes.
func (s NodeService) GetTotalCapacity(ctx context.Context, deviceGroup string) (int64, error) {
	nl, err := s.getNodes(ctx)
	if err != nil {
		return 0, err
	}

	capacity := int64(0)
	for _, node := range nl.Items {
		for key, v := range node.Status.Capacity {
			if deviceGroup == "" || string(key) == deviceGroup || string(key) == utils.DeviceCapacityKeyPrefix+deviceGroup {
				capacity += v.Value()
			}
		}
	}
	return capacity, nil
}

// GetCapacityByTopologyLabel returns VG capacity of specified node by utils's topology label.
func (s NodeService) GetCapacityByTopologyLabel(ctx context.Context, topology, deviceGroup string) (int64, error) {

	nl, err := s.getNodes(ctx)
	if err != nil {
		return 0, err
	}

	capacity := int64(0)
	for _, node := range nl.Items {
		if v, ok := node.Labels[utils.TopologyZoneKey]; ok {
			if v != topology {
				continue
			}
			for key, v := range node.Status.Allocatable {
				if string(key) == deviceGroup || string(key) == utils.DeviceCapacityKeyPrefix+deviceGroup {
					capacity += v.Value()
				}
			}
		}
	}

	return 0, ErrNodeNotFound
}
