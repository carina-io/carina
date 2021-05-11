package run

import (
	carinav1 "bocloud.com/cloudnative/carina/api/v1"
	"bocloud.com/cloudnative/carina/controllers"
	"bocloud.com/cloudnative/carina/hook"
	"bocloud.com/cloudnative/carina/pkg/configuration"
	"bocloud.com/cloudnative/carina/pkg/csidriver/csi"
	"bocloud.com/cloudnative/carina/pkg/csidriver/driver"
	"bocloud.com/cloudnative/carina/pkg/csidriver/driver/k8s"
	"bocloud.com/cloudnative/carina/pkg/csidriver/runners"
	"bocloud.com/cloudnative/carina/utils"
	"context"
	"fmt"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"net"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
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

// Run builds and starts the manager with leader election.
func subMain() error {
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&config.zapOpts)))

	cfg, err := ctrl.GetConfig()
	if err != nil {
		return err
	}

	hookHost, portStr, err := net.SplitHostPort(config.webhookAddr)
	if err != nil {
		return fmt.Errorf("invalid webhook addr: %v", err)
	}
	hookPort, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return fmt.Errorf("invalid webhook port: %v", err)
	}
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                  scheme,
		MetricsBindAddress:      config.metricsAddr,
		LeaderElection:          true,
		LeaderElectionID:        utils.CSIPluginName + "-carina-controller",
		LeaderElectionNamespace: configuration.RuntimeNamespace(),
		Host:                    hookHost,
		Port:                    hookPort,
		CertDir:                 config.certDir,
	})
	if err != nil {
		return err
	}

	// register webhook handlers
	// admissoin.NewDecoder never returns non-nil error
	dec, _ := admission.NewDecoder(scheme)
	wh := mgr.GetWebhookServer()
	wh.Register("/pod/mutate", hook.PodMutator(mgr.GetClient(), dec))
	//wh.Register("/pvc/mutate", hook.PVCMutator(mgr.GetClient(), dec))

	stopChan := make(chan struct{})
	defer close(stopChan)

	// register controllers
	nodecontroller := &controllers.NodeReconciler{
		Client:   mgr.GetClient(),
		StopChan: stopChan,
	}
	if err := nodecontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		return err
	}

	pvcontroller := &controllers.PersistentVolumeReconciler{
		Client:    mgr.GetClient(),
		APIReader: mgr.GetAPIReader(),
	}
	if err := pvcontroller.SetupWithManager(mgr, stopChan); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PersistentVolumeClaim")
		return err
	}

	// +kubebuilder:scaffold:builder

	// pre-cache objects
	ctx := context.Background()
	if _, err := mgr.GetCache().GetInformer(ctx, &storagev1.StorageClass{}); err != nil {
		return err
	}
	if _, err := mgr.GetCache().GetInformer(ctx, &corev1.Pod{}); err != nil {
		return err
	}
	if _, err := mgr.GetCache().GetInformer(ctx, &corev1.PersistentVolume{}); err != nil {
		return err
	}
	if _, err := mgr.GetCache().GetInformer(ctx, &carinav1.LogicVolume{}); err != nil {
		return err
	}

	if _, err := mgr.GetCache().GetInformer(ctx, &corev1.Node{}); err != nil {
		return err
	}

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

	// Http Server
	e := newHttpServer(mgr.GetCache(), stopChan)
	go e.start()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}
	return nil
}
