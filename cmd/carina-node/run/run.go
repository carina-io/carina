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
	carinav1 "github.com/carina-io/carina/api/v1"
	"github.com/carina-io/carina/controllers"
	"github.com/carina-io/carina/pkg/csidriver/csi"
	"github.com/carina-io/carina/pkg/csidriver/driver"
	"github.com/carina-io/carina/pkg/csidriver/driver/k8s"
	"github.com/carina-io/carina/pkg/csidriver/runners"
	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	"github.com/carina-io/carina/pkg/deviceplugin"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(carinav1.AddToScheme(scheme))
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
		LeaderElection:     false,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	// pre-cache objects
	ctx := context.Background()
	if _, err := mgr.GetCache().GetInformer(ctx, &corev1.Pod{}); err != nil {
		return err
	}

	// 初始化磁盘管理服务
	stopChan := make(chan struct{})
	defer close(stopChan)
	dm := deviceManager.NewDeviceManager( mgr.GetClient(),nodeName, mgr.GetCache(), stopChan)

	podController := controllers.PodReconciler{
		Client:   mgr.GetClient(),
		NodeName: nodeName,
		Executor: dm.Executor,
		StopChan: stopChan,
	}
	if err := podController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller ", "controller", "podController")
		return err
	}

	lvController := controllers.NewLogicVolumeReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetEventRecorderFor("logicvolume-node"),
		nodeName,
		dm.VolumeManager,
	)

	if err := lvController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalVolume")
		return err
	}
	// +kubebuilder:scaffold:builder

	// Add metrics exporter to manager.
	// Note that grpc.ClientConn can be shared with multiple stubs/services.
	// https://github.com/grpc/grpc-go/tree/master/examples/features/multiplex
	if err := mgr.Add(runners.NewMetricsExporter(nodeName, dm.VolumeManager)); err != nil {
		return err
	}

	// Add gRPC server to manager.
	s, err := k8s.NewLogicVolumeService(mgr)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(driver.DeviceDirectory, 0755); err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	csi.RegisterIdentityServer(grpcServer, driver.NewIdentityService())
	csi.RegisterNodeServer(grpcServer, driver.NewNodeService(nodeName, dm.VolumeManager, s))
	err = mgr.Add(runners.NewGRPCRunner(grpcServer, config.csiSocket, false))
	if err != nil {
		return err
	}

	// 启动磁盘检查
	dm.DeviceCheckTask()
	// 启动volume一致性检查
	dm.VolumeConsistencyCheck()
	// 启动设备插件
	go deviceplugin.Run(dm.VolumeManager, stopChan)
	// http server
	e := newHttpServer(dm.VolumeManager, stopChan)
	go e.start()
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		close(stopChan)
		return err
	}

	return nil
}
