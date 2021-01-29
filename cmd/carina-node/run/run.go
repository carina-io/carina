package run

import (
	carinav1 "carina/api/v1"
	"carina/controllers"
	deviceManager "carina/pkg/devicemanager"
	"carina/utils/log"
	"errors"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	gitCommitID = "dev"
	scheme      = runtime.NewScheme()
	setupLog    = ctrl.Log.WithName("setup")
)

func init() {
	if err := carinav1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		panic(err)
	}

	// +kubebuilder:scaffold:scheme
}

func subMain() error {
	nodeName := viper.GetString("nodename")
	if len(nodeName) == 0 {
		return errors.New("node name is not given")
	}
	printWelcome()

	ctrl.SetLogger(zap.New(zap.UseDevMode(config.development)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: config.metricsAddr,
		LeaderElection:     false,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	// 初始化磁盘管理服务
	stopChan := make(chan struct{})
	dm := deviceManager.NewDeviceManager(nodeName, stopChan)

	lvController := controllers.NewLogicVolumeReconciler(
		mgr.GetClient(),
		ctrl.Log.WithName("controllers").WithName("LogicVolume"),
		nodeName,
		dm.VolumeManager,
	)

	if err := lvController.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "LogicalVolume")
		return err
	}
	// +kubebuilder:scaffold:builder

	// Add health checker to manager
	//checker := runners.NewChecker(checkFunc(conn, mgr.GetAPIReader()), 1*time.Minute)
	//if err := mgr.Add(checker); err != nil {
	//	return err
	//}

	// Add metrics exporter to manager.
	// Note that grpc.ClientConn can be shared with multiple stubs/services.
	// https://github.com/grpc/grpc-go/tree/master/examples/features/multiplex
	//if err := mgr.Add(runners.NewMetricsExporter(conn, mgr, nodeName)); err != nil {
	//	return err
	//}
	//
	//// Add gRPC server to manager.
	//s, err := k8s.NewLogicalVolumeService(mgr)
	//if err != nil {
	//	return err
	//}
	//if err := os.MkdirAll(driver.DeviceDirectory, 0755); err != nil {
	//	return err
	//}
	//grpcServer := grpc.NewServer()
	//csi.RegisterIdentityServer(grpcServer, driver.NewIdentityService(checker.Ready))
	//csi.RegisterNodeServer(grpcServer, driver.NewNodeService(nodeName, conn, s))
	//err = mgr.Add(runners.NewGRPCRunner(grpcServer, config.csiSocket, false))
	//if err != nil {
	//	return err
	//}

	// 启动磁盘检查
	dm.DeviceCheckTask()
	// 启动lvm卷健康检查
	dm.LvmHealthCheck()
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		close(stopChan)
		return err
	}

	return nil
}

func printWelcome() {
	if gitCommitID == "" {
		gitCommitID = "dev"
	}
	log.Info("-------- Welcome to use Carina Node Server --------")
	log.Infof("Git Commit ID : %s", gitCommitID)
	log.Info("------------------------------------")
}
