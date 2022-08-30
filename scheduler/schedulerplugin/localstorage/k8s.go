package localstorage

import (
	"context"
	carina "github.com/carina-io/carina/scheduler"
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

func getNodeStorageResource(client dynamic.Interface, nodeName string) (*v1beta1.NodeStorageResource, error) {
	var gvr = schema.GroupVersionResource{
		Group:    v1beta1.GroupVersion.Group,
		Version:  v1beta1.GroupVersion.Version,
		Resource: "nodestorageresources",
	}
	unstructObj, err := client.Resource(gvr).Namespace("").Get(context.TODO(), nodeName, metav1.GetOptions{})
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

func getLvExclusivityDisks(client dynamic.Interface, nodeName string) (lvs []string, err error) {
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
	klog.V(3).Infof("Get lvlist: %v", lvlist)
	if len(lvlist.Items) == 0 {
		return lvs, nil
	}
	for _, lv := range lvlist.Items {
		if lv.Annotations == nil {
			continue
		}
		klog.V(3).Infof("Get lv: %v, exclusivity: %s", lv.Spec.NodeName, lv.Annotations[carina.ExclusivityDisk])
		if lv.Spec.NodeName == nodeName && lv.Annotations[carina.ExclusivityDisk] == "true" {
			lvs = append(lvs, lv.Spec.DeviceGroup)
		}
	}
	return lvs, nil
}
