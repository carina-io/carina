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

package run

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	carinav1 "github.com/carina-io/carina/api/v1"
	carinav1beta1 "github.com/carina-io/carina/api/v1beta1"
	"github.com/carina-io/carina/controllers"
	"github.com/carina-io/carina/pkg/csidriver/driver"
	"github.com/carina-io/carina/pkg/csidriver/driver/k8s"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	carinaMetrics "github.com/carina-io/carina/pkg/metrics"
	"github.com/carina-io/carina/runners"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(carinav1.AddToScheme(scheme))
	utilruntime.Must(carinav1beta1.AddToScheme(scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// +kubebuilder:scaffold:scheme
}

func subMain() error {
	nodeName := os.Getenv("NODE_NAME")
	if len(nodeName) == 0 {
		return errors.New("env NODE_NAME is not given")
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&config.zapOpts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: config.metricsAddr,
		NewCache: cache.BuilderWithOptions(cache.Options{
			Scheme: scheme,
			SelectorsByObject: cache.SelectorsByObject{
				&corev1.Node{}: {
					Field: fields.SelectorFromSet(fields.Set{"metadata.name": nodeName}),
				},
				&corev1.Pod{}: {
					Field: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName}),
				},
				&carinav1beta1.NodeStorageResource{}: {
					Field: fields.SelectorFromSet(fields.Set{"metadata.name": nodeName}),
				},
			},
		}),
		LeaderElection: false,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	// 初始化磁盘管理服务
	dm := deviceManager.NewDeviceManager(nodeName, mgr.GetCache())

	// pod io controller
	podIOController := controllers.NewPodIOReconciler(
		mgr.GetClient(),
		nodeName,
		dm.Partition,
	)
	if err = podIOController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller ", "controller", "podController")
		return err
	}
	// logic volume controller
	lvController := controllers.NewLogicVolumeReconciler(
		mgr.GetClient(),
		mgr.GetEventRecorderFor("logicvolume-node"),
		dm,
	)
	if err = lvController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalVolume")
		return err
	}

	//+kubebuilder:scaffold:builder

	// Add health checker to manager
	checker := runners.NewChecker(checkFunc(dm, mgr.GetAPIReader()), 1*time.Minute)
	if err = mgr.Add(checker); err != nil {
		return err
	}

	// Add metrics exporter to manager.
	// Note that grpc.ClientConn can be shared with multiple stubs/services.
	// https://github.com/grpc/grpc-go/tree/master/examples/features/multiplex
	lvService, err := k8s.NewLogicVolumeService(mgr)
	if err != nil {
		return err
	}
	carinaCollector, err := carinaMetrics.NewCarinaCollector(dm, lvService)
	if err != nil {
		return err
	}
	metrics.Registry.Register(carinaCollector)

	// Add gRPC server to manager.
	if err = os.MkdirAll(driver.DeviceDirectory, 0755); err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	csi.RegisterIdentityServer(grpcServer, driver.NewIdentityService(checker.Ready))
	csi.RegisterNodeServer(grpcServer, driver.NewNodeService(dm, lvService))
	if err = mgr.Add(runners.NewGRPCRunner(grpcServer, config.csiSocket, false)); err != nil {
		return err
	}

	// add cleanupOrphan to manager
	if err = mgr.Add(runners.NewTroubleShoot(dm)); err != nil {
		return err
	}

	// add device check to manager, add or delete device
	if err = mgr.Add(runners.NewDeviceCheck(dm)); err != nil {
		return err
	}

	// add nsr reconciler to manager
	if err = mgr.Add(runners.NewNodeStorageResourceReconciler(mgr, dm)); err != nil {
		return err
	}

	setupLog.Info("starting manager")
	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=storage.k8s.io,resources=csidrivers,verbs=get;list;watch

func checkFunc(dm *deviceManager.DeviceManager, c client.Reader) func() error {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var drv storagev1.CSIDriver
		return c.Get(ctx, types.NamespacedName{Name: carina.CSIPluginName}, &drv)
	}
}
