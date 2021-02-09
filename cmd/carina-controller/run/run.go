package run

import (
	carinav1 "carina/api/v1"
	"carina/controllers"
	"carina/pkg/csidriver/csi"
	"carina/pkg/csidriver/driver"
	"carina/pkg/csidriver/driver/k8s"
	"carina/pkg/csidriver/runners"
	"carina/utils"
	"context"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
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

// Run builds and starts the manager with leader election.
func subMain() error {
	ctrl.SetLogger(zap.New(zap.UseDevMode(config.development)))

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return err
	}

	//hookHost, portStr, err := net.SplitHostPort(config.webhookAddr)
	//if err != nil {
	//	return fmt.Errorf("invalid webhook addr: %v", err)
	//}
	//hookPort, err := net.LookupPort("tcp", portStr)
	//if err != nil {
	//	return fmt.Errorf("invalid webhook port: %v", err)
	//}
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      config.metricsAddr,
		LeaderElection:          true,
		LeaderElectionID:        utils.PluginName + "-carina-controller",
		LeaderElectionNamespace: "kube-system",
		//Host:               hookHost,
		//Port:               hookPort,
		//CertDir:            config.certDir,
	})
	if err != nil {
		return err
	}

	// register webhook handlers
	// admissoin.NewDecoder never returns non-nil error
	//dec, _ := admission.NewDecoder(scheme)
	//wh := mgr.GetWebhookServer()
	//wh.Register("/pod/mutate", hook.PodMutator(mgr.GetClient(), dec))
	//wh.Register("/pvc/mutate", hook.PVCMutator(mgr.GetClient(), dec))

	// register controllers
	//nodecontroller := &controllers.NodeReconciler{
	//	Client: mgr.GetClient(),
	//	Log:    ctrl.Log.WithName("controllers").WithName("Node"),
	//}
	//if err := nodecontroller.SetupWithManager(mgr); err != nil {
	//	setupLog.Error(err, "unable to create controller", "controller", "Node")
	//	return err
	//}

	pvcontroller := &controllers.PersistentVolumeReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
		Log:       ctrl.Log.WithName("controllers").WithName("PersistentVolume"),
	}
	if err := pvcontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PersistentVolumeClaim")
		return err
	}

	// +kubebuilder:scaffold:builder

	// pre-cache objects
	ctx := context.Background()
	//if _, err := mgr.GetCache().GetInformer(ctx, &storagev1.StorageClass{}); err != nil {
	//	return err
	//}
	//if _, err := mgr.GetCache().GetInformer(ctx, &corev1.Pod{}); err != nil {
	//	return err
	//}
	if _, err := mgr.GetCache().GetInformer(ctx, &corev1.PersistentVolume{}); err != nil {
		return err
	}
	if _, err := mgr.GetCache().GetInformer(ctx, &carinav1.LogicVolume{}); err != nil {
		return err
	}

	// Add health checker to manager
	//check := func() error {
	//	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	//	defer cancel()
	//
	//	var drv storagev1beta1.CSIDriver
	//	return mgr.GetAPIReader().Get(ctx, types.NamespacedName{Name: utils.PluginName}, &drv)
	//}
	//checker := runners.NewChecker(check, 1*time.Minute)
	//if err := mgr.Add(checker); err != nil {
	//	return err
	//}

	// Add gRPC server to manager.
	s, err := k8s.NewLogicVolumeService(mgr)
	if err != nil {
		return err
	}
	n := k8s.NewNodeService(mgr)

	grpcServer := grpc.NewServer()
	csi.RegisterIdentityServer(grpcServer, driver.NewIdentityService())
	csi.RegisterControllerServer(grpcServer, driver.NewControllerService(s, n))

	// gRPC service itself should run even when the manager is *not* a leader
	// because CSI sidecar containers choose a leader.
	err = mgr.Add(runners.NewGRPCRunner(grpcServer, config.csiSocket, false))
	if err != nil {
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}
	return nil
}
