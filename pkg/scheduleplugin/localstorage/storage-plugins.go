package localstorage

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// 插件名称
const Name = "carina-schedule"

type LocalStorage struct {
	handle framework.Handle
}

//type PluginFactory = func(configuration *runtime.Unknown, f FrameworkHandle) (Plugin, error)
func New(_ runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	return &LocalStorage{
		handle: handle,
	}, nil
}

func (ls *LocalStorage) Name() string {
	return Name
}

func (ls *LocalStorage) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) *framework.Status {
	klog.V(3).Infof("prefilter pod: %v", pod.Name)
	return framework.NewStatus(framework.Success, "")
}

// 过滤掉不符合当前 Pod 运行条件的Node（相当于旧版本的 predicate）
func (ls *LocalStorage) Filter(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod, node *framework.NodeInfo) *framework.Status {

	klog.V(3).Infof("filter pod: %v, node: %v", pod.Name, node.Node().Name)

	for _, p := range pod.Spec.Containers {
		if p.Resources.Requests.Memory().Value() > node.Allocatable.Memory {
			return framework.NewStatus(framework.Unschedulable, "out of memory")
		}
	}
	return framework.NewStatus(framework.Success, "")
}
