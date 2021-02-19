package k8s

import (
	"carina/pkg/configruation"
	"carina/utils"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type nodeService interface {
	getNodes(ctx context.Context) (*corev1.NodeList, error)
	SelectVolumeNode(ctx context.Context, request int64, deviceGroup string) (string, string, error)
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

func (s NodeService) SelectVolumeNode(ctx context.Context, requestGb int64, deviceGroup string) (string, string, error) {
	var nodeName, selectDeviceGroup string
	nl, err := s.getNodes(ctx)
	if err != nil {
		return "", "", err
	}

	type paris struct {
		Key   string
		Value int64
	}

	preselectNode := []paris{}

	for _, node := range nl.Items {
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
		return "", "", ErrNodeNotFound
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
		return "", "", errors.New(fmt.Sprintf("no support scheduler strategy %s", configruation.SchedulerStrategy()))
	}

	return nodeName, selectDeviceGroup, nil
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
			if string(key) == deviceGroup || string(key) == utils.DeviceCapacityKeyPrefix+deviceGroup {
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

	for _, node := range nl.Items {
		if v, ok := node.Labels[utils.TopologyNodeKey]; ok {
			if v != topology {
				continue
			}
			for key, v := range node.Status.Allocatable {
				if string(key) == deviceGroup || string(key) == utils.DeviceCapacityKeyPrefix+deviceGroup {
					return v.Value(), nil
				}
			}
		}
	}

	return 0, ErrNodeNotFound
}
