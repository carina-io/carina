package localstorage

import (
	"context"
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

var gvr = schema.GroupVersionResource{
	Group:    v1beta1.GroupVersion.Group,
	Version:  v1beta1.GroupVersion.Version,
	Resource: "nodestorageresources",
}

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

func getNodeStorageResource(client dynamic.Interface, node string) (*v1beta1.NodeStorageResource, error) {
	unstructObj, err := client.Resource(gvr).Namespace("").Get(context.TODO(), node, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	nsr := &v1beta1.NodeStorageResource{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstructObj.UnstructuredContent(), nsr)
	if err != nil {
		return nil, err
	}
	return nsr, nil
}

func listNodeStorageResources(client dynamic.Interface) (*v1beta1.NodeStorageResourceList, error) {
	unstrructObj, err := client.Resource(gvr).Namespace("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	nsr := &v1beta1.NodeStorageResourceList{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstrructObj.UnstructuredContent(), nsr)
	if err != nil {
		return nil, err
	}
	return nsr, nil
}

func listLogicVolumes(client dynamic.Interface, node string) (lvs []string, err error) {
	var gvr = schema.GroupVersionResource{
		Group:    v1.GroupVersion.Group,
		Version:  v1.GroupVersion.Version,
		Resource: "logicvolumes",
	}
	unstrructObj, err := client.Resource(gvr).Namespace("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	lvlist := &v1.LogicVolumeList{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(unstrructObj.UnstructuredContent(), lvlist)
	if err != nil {
		return nil, err
	}
	klog.V(3).Infof("Get lvlist:%v", lvlist)
	if len(lvlist.Items) == 0 {
		return lvs, nil
	}
	for _, lv := range lvlist.Items {
		klog.V(3).Infof("Get lv:%v,node:%s, exclusivity: %s", lv.Spec.NodeName, node, lv.Annotations[utils.ExclusivityDisk])
		if lv.Spec.NodeName == node && lv.Annotations[utils.ExclusivityDisk] == "true" {
			lvs = append(lvs, lv.Spec.DeviceGroup)
		}
	}
	return lvs, nil
}
