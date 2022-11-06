package localstorage

import (
	"context"
	carina "github.com/carina-io/carina/scheduler"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"path/filepath"

	v1 "github.com/carina-io/carina-api/api/v1"
	"github.com/carina-io/carina-api/api/v1beta1"
	"github.com/carina-io/carina/scheduler/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
)

func newDynamicClientFromConfig() dynamic.Interface {
	var kubeconfig string
	var config *rest.Config
	var err error

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	if utils.Exists(kubeconfig) {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		panic(err.Error())
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return dynamicClient
}

func getNodeStorageResource(client dynamic.Interface, nsrLister cache.GenericLister, nodeName string) (*v1beta1.NodeStorageResource, error) {
	var gvr = schema.GroupVersionResource{
		Group:    v1beta1.GroupVersion.Group,
		Version:  v1beta1.GroupVersion.Version,
		Resource: "nodestorageresources",
	}
	var workloadUnstructured *unstructured.Unstructured
	workloadObj, err := nsrLister.Get(nodeName)
	if err != nil {
		// fall back to call api server in case the cache has not been synchronized yet
		klog.Warningf("Failed to get nsr from cache, name: %s. Error: %v. Fall back to call api server", nodeName, err)
		workloadUnstructured, err = client.Resource(gvr).Namespace("").Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Failed to get workload from api server, name: %s. Error: %v", nodeName, err)
			return nil, err
		}
	} else {
		workloadUnstructured, err = utils.ToUnstructured(workloadObj)
		if err != nil {
			klog.Errorf("Failed to convert unstructured from runtime object, name: %s. Error: %v", nodeName, err)
			return nil, err
		}
	}
	nsr := &v1beta1.NodeStorageResource{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(workloadUnstructured.UnstructuredContent(), nsr)
	if err != nil {
		return nil, err
	}
	return nsr, nil
}

func getLvExclusivityDisks(client dynamic.Interface, lvLister cache.GenericLister, nodeName string) (lvDeviceGroups []string, err error) {
	var gvr = schema.GroupVersionResource{
		Group:    v1.GroupVersion.Group,
		Version:  v1.GroupVersion.Version,
		Resource: "logicvolumes",
	}

	workloadObjs, err := lvLister.List(labels.Everything())
	var lvs []v1.LogicVolume
	if err != nil {
		// fall back to call api server in case the cache has not been synchronized yet
		klog.Warningf("Failed to get lvs from cache, name: %s. Error: %v. Fall back to call api server", nodeName, err)
		workloadUnstructureds, err := client.Resource(gvr).Namespace("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			klog.Errorf("Failed to get workload from api server, name: %s. Error: %v", nodeName, err)
			return nil, err
		}

		lvList := &v1.LogicVolumeList{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(workloadUnstructureds.UnstructuredContent(), lvList)
		if err != nil {
			return nil, err
		}
		lvs = lvList.Items
	} else {
		if workloadObjs == nil || len(workloadObjs) == 0 {
			return nil, nil
		}
		for _, workloadObj := range workloadObjs {
			unstructured, err := utils.ToUnstructured(workloadObj)
			if err != nil {
				klog.Errorf("Failed to convert unstructured from runtime object, name: %s. Error: %v", nodeName, err)
				return nil, err
			}
			lv := &v1.LogicVolume{}
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructured.UnstructuredContent(), lv)
			if err != nil {
				return nil, err
			}
			lvs = append(lvs, *lv)
		}
	}
	klog.V(3).Infof("Get logic volumes: %v", lvs)
	if lvs == nil || len(lvs) == 0 {
		return nil, nil
	}
	for _, lv := range lvs {
		if lv.Annotations == nil {
			continue
		}
		klog.V(3).Infof("Get lv: %v, exclusivity: %s", lv.Spec.NodeName, lv.Annotations[carina.ExclusivityDisk])
		if lv.Spec.NodeName == nodeName && lv.Annotations[carina.ExclusivityDisk] == "true" {
			lvDeviceGroups = append(lvDeviceGroups, lv.Spec.DeviceGroup)
		}
	}
	return lvDeviceGroups, nil
}
