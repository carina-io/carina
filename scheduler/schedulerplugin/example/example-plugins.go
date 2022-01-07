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

package example

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"math"
	"time"
)

const ExampleName = "Example-schedule"

type ExamplePlugin struct {
	handle framework.Handle
	//cache  cache.Cache
}

// StateStorage 存储状态，插件间数据传输
type StateStorage struct {
	framework.Resource
}

// Clone the preFilter state.
func (s *StateStorage) Clone() framework.StateData {
	return s
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, handle framework.Handle) (framework.Plugin, error) {

	//mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
	//	MetricsBindAddress: "",
	//	LeaderElection:     false,
	//	Port:               9443,
	//})
	//if err != nil {
	//	klog.Error(err)
	//	return nil, err
	//}
	//go func() {
	//	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
	//		klog.Error(err)
	//		panic(err)
	//	}
	//}()
	return &ExamplePlugin{
		handle: handle,
		//cache:  mgr.GetCache(),
	}, nil
}

func (ep *ExamplePlugin) Name() string {
	return ExampleName
}

// Less Schedule Pod Queue排序规则
func (ep *ExamplePlugin) Less(podInfo1, podInfo2 *framework.QueuedPodInfo) bool {
	// 在此可以通过特殊字段进行排序，例如让同一组对Pod排在一起调度
	return podInfo1.Timestamp.Before(podInfo1.Timestamp)
}

// PreFilter Pod 调度前的条件检查，如果检查不通过，直接结束本调度周期（过滤带有某些标签、annotation的pod）
func (ep *ExamplePlugin) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) *framework.Status {

	v := &StateStorage{}
	for _, c := range pod.Spec.Containers {
		v.Add(c.Resources.Requests)
	}

	// 多个插件间数据传递使用
	state.Write("example", v)

	if pod.Spec.SchedulerName != ExampleName {
		return framework.NewStatus(framework.Unschedulable, fmt.Sprintf("rejected schedule %s", pod.Spec.SchedulerName))
	}
	return framework.NewStatus(framework.Success, "")
}

// PreFilterExtensions prefilter扩展功能，评估add/remove pod的影响，如果不实现可返回nil
// PreFilter之后调用
func (ep *ExamplePlugin) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

// AddPod 评估添加pod到node的影响
func (ep *ExamplePlugin) AddPod(ctx context.Context, cycleState *framework.CycleState, podToSchedule *v1.Pod, podToAdd *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {

	// PreFilterExtensions 需要实现的方法
	return framework.NewStatus(framework.Success, "")
}

// RemovePod 评估删除pod到node的影响
func (ep *ExamplePlugin) RemovePod(ctx context.Context, cycleState *framework.CycleState, podToSchedule *v1.Pod, podToRemove *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {

	// PreFilterExtensions 需要实现的方法
	return framework.NewStatus(framework.Success, "")
}

// Filter 过滤掉不符合当前 Pod 运行条件的Node（相当于旧版本的 predicate）
func (ep *ExamplePlugin) Filter(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod, node *framework.NodeInfo) *framework.Status {

	klog.V(3).Infof("filter pod: %v, node: %v", pod.Name, node.Node().Name)

	for _, p := range pod.Spec.Containers {
		if p.Resources.Requests.Memory().Value() > node.Allocatable.Memory {
			return framework.NewStatus(framework.Unschedulable, "out of memory")
		}
	}
	return framework.NewStatus(framework.Success, "")
}

// PostFilter 在预选后被调用，通常用来记录日志和监控信息。也可以当做 “Pre-scoring” 插件的扩展点
// Filter插件执行完后执行（一般用于 Pod 抢占逻辑的处理）
func (ep *ExamplePlugin) PostFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, filteredNodeStatusMap framework.NodeToStatusMap) (*framework.PostFilterResult, *framework.Status) {
	klog.V(3).Infof("collect info for scheduling pod: %v", pod.Name)

	_, err := state.Read("example")
	if err == nil {
		state.Delete("example")
		return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable)
	}
	return &framework.PostFilterResult{}, framework.NewStatus(framework.Success)
}

// PreScore 打分前的状态处理
func (ep *ExamplePlugin) PreScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodes []*v1.Node) *framework.Status {
	return framework.NewStatus(framework.Success, "")
}

// Score 对节点进行打分（相当于旧版本的 priorities）
func (ep *ExamplePlugin) Score(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) (int64, *framework.Status) {
	// Get Node Info
	// 节点信息快照在执行调度时创建，并在在整个调度周期内不变
	nodeInfo, err := ep.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
	}
	// example 1
	ep.handle.ClientSet().NodeV1().RESTClient().Get().Name(nodeName)

	// example 2

	//err = ep.cache.Get(ctx, types.NamespacedName{Name: nodeName}, &v1.Node{})
	//if err != nil {
	//	klog.Errorf("Get CRD Error: %v", err)
	//	return 0, framework.NewStatus(framework.Error, fmt.Sprintf("Score Node Error: %v", err))
	//}
	// 分数计算
	score := int64(len(nodeInfo.Pods))
	return score, framework.NewStatus(framework.Success, "")
}

// ScoreExtensions 打分后扩展，由此方法则可调用NormalizeScore
func (ep *ExamplePlugin) ScoreExtensions() framework.ScoreExtensions {
	return ep
}

// NormalizeScore 在调度器为节点计算最终排名前修改节点排名。配合 Scoring 插件使用，为了平衡插件中的打分情况
func (ep *ExamplePlugin) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	// Find highest and lowest scores.
	var highest int64 = -math.MaxInt64
	var lowest int64 = math.MaxInt64
	for _, nodeScore := range scores {
		if nodeScore.Score > highest {
			highest = nodeScore.Score
		}
		if nodeScore.Score < lowest {
			lowest = nodeScore.Score
		}
	}

	// Transform the highest to lowest score range to fit the framework's min to max node score range.
	oldRange := highest - lowest
	newRange := framework.MaxNodeScore - framework.MinNodeScore
	for i, nodeScore := range scores {
		if oldRange == 0 {
			scores[i].Score = framework.MinNodeScore
		} else {
			scores[i].Score = ((nodeScore.Score - lowest) * newRange / oldRange) + framework.MinNodeScore
		}
	}
	return framework.NewStatus(framework.Success, "")
}

// Reserve 与Unreserve成对出现
// 为给定的 Pod 预留节点上的资源，目的是为了防止资源竞争，并且是在绑定前做的；
func (ep *ExamplePlugin) Reserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {

	// 预留资源
	return framework.NewStatus(framework.Success, "")
}

func (ep *ExamplePlugin) Unreserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) {
	// bind失败释放资源
}

// Permit Pod 绑定之前的准入控制
func (ep *ExamplePlugin) Permit(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) (*framework.Status, time.Duration) {
	// 创建时间小于六分钟的等着
	t := time.Now().Sub(pod.CreationTimestamp.Local())
	if t.Seconds() < float64(360) {
		return framework.NewStatus(framework.Wait, ""), t
	}
	return framework.NewStatus(framework.Success, ""), 0
}

// PreBind 绑定 Pod 之前的逻辑，如：先预挂载共享存储，查看是否正常挂载
func (ep *ExamplePlugin) PreBind(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) *framework.Status {
	return framework.NewStatus(framework.Success, "")
}

// Bind 节点和 Pod 绑定
func (ep *ExamplePlugin) Bind(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) *framework.Status {
	return framework.NewStatus(framework.Success, "")
}

// PostBind Pod绑定成功后的资源清理逻辑
func (ep *ExamplePlugin) PostBind(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) {
	klog.V(5).Infof("PostBind pod: %v", pod.Name)
}
