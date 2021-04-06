package controllers

import (
	"bocloud.com/cloudnative/carina/utils"
	"bocloud.com/cloudnative/carina/utils/exec"
	"bocloud.com/cloudnative/carina/utils/log"
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/workqueue"
	"path"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"strconv"
	"time"
)

// PodReconciler reconciles a Node object
type PodReconciler struct {
	client.Client
	NodeName string
	Executor exec.Executor
}

// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;delete

// Reconcile finalize Node
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	// your logic here
	pod := &corev1.Pod{}
	err := r.Client.Get(ctx, req.NamespacedName, pod)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	if pod.DeletionTimestamp == nil {
		return ctrl.Result{}, nil
	}

	if err := r.deviceSpeedLimit(ctx, pod); err != nil {

	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(podFilter{r.NodeName}).
		WithOptions(controller.Options{
			RateLimiter: workqueue.NewItemFastSlowRateLimiter(10*time.Second, 60*time.Second, 5),
		}).
		For(&corev1.Pod{}).
		Complete(r)
}

func (r *PodReconciler) deviceSpeedLimit(ctx context.Context, pod *corev1.Pod) error {

	throttle, err := r.extractPodBlkioResources(pod.Annotations)
	if err != nil {
		return err
	}

	for _, pv := range pod.Spec.Volumes {
		if pv.VolumeSource.CSI.Driver != utils.CSIPluginName {
			continue
		}

		deviceMajor := pv.CSI.VolumeAttributes[utils.VolumeDeviceMajor]
		deviceMinor := pv.CSI.VolumeAttributes[utils.VolumeDeviceMinor]

		// TODO 检查设备限制是否已经存在
		err = r.addBlkDeviceToCgroup(deviceMajor, deviceMinor, throttle)
		if err != nil {
			return err
		}

	}

	return nil
}

func (r *PodReconciler) extractPodBlkioResources(podAnnotations map[string]string) (map[string]int, error) {
	var err error
	throttle := map[string]int{
		utils.BlkIOThrottleReadBPS:   -1,
		utils.BlkIOThrottleReadIOPS:  -1,
		utils.BlkIOThrottleWriteBPS:  -1,
		utils.BlkIOThrottleWriteIOPS: -1,
	}

	if podAnnotations == nil {
		return throttle, nil
	}

	if str, found := (podAnnotations)[utils.BlkIOThrottleReadBPS]; found {
		if throttle[utils.BlkIOThrottleReadBPS], err = strconv.Atoi(str); err != nil {
			return throttle, err
		}
	}
	if str, found := (podAnnotations)[utils.BlkIOThrottleReadIOPS]; found {
		if throttle[utils.BlkIOThrottleReadIOPS], err = strconv.Atoi(str); err != nil {
			return throttle, err
		}
	}
	if str, found := (podAnnotations)[utils.BlkIOThrottleWriteBPS]; found {
		if throttle[utils.BlkIOThrottleWriteBPS], err = strconv.Atoi(str); err != nil {
			return throttle, err
		}
	}
	if str, found := (podAnnotations)[utils.BlkIOThrottleWriteIOPS]; found {
		if throttle[utils.BlkIOThrottleWriteIOPS], err = strconv.Atoi(str); err != nil {
			return throttle, err
		}
	}
	return throttle, nil
}

func (r *PodReconciler) addBlkDeviceToCgroup(major, minor string, throttle map[string]int) error {
	blkioCgroup := "/sys/fs/cgroup/blkio/"

	if throttle[utils.BlkIOThrottleReadBPS] > 0 {
		throttlePath := path.Join(blkioCgroup, "blkio.throttle.read_bps_device")
		//TODO 处理一下限制变更的情况

		if err := r.Executor.ExecuteCommand(fmt.Sprintf("echo %s:%s %d > %s", major, minor,
			throttle[utils.BlkIOThrottleReadBPS], throttlePath)); err != nil {
			log.Infof(
				"failed to throttle %d:%d blkio.throttle.read_bps_device, err %s.",
				major, minor, err)
			return err
		}
	}
	if throttle[utils.BlkIOThrottleReadIOPS] > 0 {
		throttlePath := path.Join(blkioCgroup, "blkio.throttle.read_iops_device")
		//TODO 处理一下限制变更的情况
		if err := r.Executor.ExecuteCommand(fmt.Sprintf("echo %s:%s %d > %s", major, minor,
			throttle[utils.BlkIOThrottleReadIOPS], throttlePath)); err != nil {
			log.Infof(
				"failed to throttle %d:%d blkio.throttle.read_iops_device, err %s.",
				major, minor, err)
			return err
		}

	}
	if throttle[utils.BlkIOThrottleWriteBPS] > 0 {
		throttlePath := path.Join(blkioCgroup, "blkio.throttle.write_bps_device")
		//TODO 处理一下限制变更的情况
		if err := r.Executor.ExecuteCommand(fmt.Sprintf("echo %s:%s %d > %s", major, minor,
			throttle[utils.BlkIOThrottleWriteBPS], throttlePath)); err != nil {
			log.Infof(
				"failed to throttle %d:%d blkio.throttle.write_bps_device, err %s.",
				major, minor, err)
			return err
		}
	}
	if throttle[utils.BlkIOThrottleWriteIOPS] > 0 {
		throttlePath := path.Join(blkioCgroup, "blkio.throttle.write_iops_device")
		//TODO 处理一下限制变更的情况
		if err := r.Executor.ExecuteCommand(fmt.Sprintf("echo %s:%s %d > %s", major, minor,
			throttle[utils.BlkIOThrottleWriteIOPS], throttlePath)); err != nil {
			log.Infof(
				"failed to throttle %d:%d blkio.throttle.write_iops_device, err %s.",
				major, minor, err)
			return err
		}
	}
	return nil
}

// filter carina pod
type podFilter struct {
	nodeName string
}

func (p podFilter) filter(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	if pod.Spec.NodeName == p.nodeName && pod.Spec.SchedulerName == utils.CarinaSchedule {
		return true
	}
	return false
}

func (p podFilter) Create(e event.CreateEvent) bool {
	return p.filter(e.Object.(*corev1.Pod))
}

func (p podFilter) Delete(e event.DeleteEvent) bool {
	return p.filter(e.Object.(*corev1.Pod))
}

func (p podFilter) Update(e event.UpdateEvent) bool {
	return p.filter(e.ObjectNew.(*corev1.Pod))
}

func (p podFilter) Generic(e event.GenericEvent) bool {
	return p.filter(e.Object.(*corev1.Pod))
}
