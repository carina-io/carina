package deviceplugin

import (
	"bocloud.com/cloudnative/carina/pkg/devicemanager/types"
	"bocloud.com/cloudnative/carina/pkg/devicemanager/volume"
	"bocloud.com/cloudnative/carina/pkg/deviceplugin/v1beta1"
	"bocloud.com/cloudnative/carina/utils"
	"bocloud.com/cloudnative/carina/utils/log"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	//pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// CarinaDevicePlugin implements the Kubernetes device plugin API
type CarinaDevicePlugin struct {
	resourceName string
	socket       string

	volumeManager volume.LocalVolume
	server        *grpc.Server
	update        chan struct{}
	stop          chan interface{}
}

// NewCarinaDevicePlugin returns an initialized CarinaDevicePlugin
func NewCarinaDevicePlugin(resourceName string, volumeManager volume.LocalVolume, update chan struct{}, socket string) *CarinaDevicePlugin {
	return &CarinaDevicePlugin{
		resourceName:  resourceName,
		volumeManager: volumeManager,
		socket:        socket,

		// These will be reinitialized every
		// time the plugin server is restarted.
		server: nil,
		update: update,
		stop:   nil,
	}
}

func (dp *CarinaDevicePlugin) cleanup() {
	close(dp.stop)
	close(dp.update)
	dp.server = nil
	dp.update = nil
	dp.stop = nil
}

// Start starts the gRPC server, registers the device plugin with the Kubelet,
// and starts the device healthchecks.
func (dp *CarinaDevicePlugin) Start() error {
	dp.server = grpc.NewServer([]grpc.ServerOption{}...)
	dp.stop = make(chan interface{})

	err := dp.Serve()
	if err != nil {
		log.Errorf("Could not start device plugin for '%s': %s", dp.resourceName, err)
		dp.cleanup()
		return err
	}
	log.Infof("Starting to serve '%s' on %s", dp.resourceName, dp.socket)

	err = dp.Register()
	if err != nil {
		log.Errorf("Could not register device plugin: %s", err)
		_ = dp.Stop()
		return err
	}
	log.Infof("Registered device plugin for '%s' with Kubelet", dp.resourceName)

	return nil
}

// Stop stops the gRPC server.
func (dp *CarinaDevicePlugin) Stop() error {
	if dp == nil || dp.server == nil {
		return nil
	}
	log.Infof("Stopping to serve '%s' on %s", dp.resourceName, dp.socket)
	dp.server.Stop()
	if err := os.Remove(dp.socket); err != nil && !os.IsNotExist(err) {
		return err
	}
	dp.cleanup()
	return nil
}

// Serve starts the gRPC server of the device plugin.
func (dp *CarinaDevicePlugin) Serve() error {
	_ = os.Remove(dp.socket)
	sock, err := net.Listen("unix", dp.socket)
	if err != nil {
		return err
	}

	v1beta1.RegisterDevicePluginServer(dp.server, dp)

	go func() {
		lastCrashTime := time.Now()
		restartCount := 0
		for {
			log.Infof("Starting GRPC server for '%s'", dp.resourceName)
			err := dp.server.Serve(sock)
			if err == nil {
				break
			}
			log.Infof("GRPC server for '%s' crashed with error: %v", dp.resourceName, err)

			// restart if it has not been too often
			// i.e. if server has crashed more than 5 times and it didn't last more than one hour each time
			if restartCount > 5 {
				// quit
				log.Fatalf("GRPC server for '%s' has repeatedly crashed recently. Quitting", dp.resourceName)
			}
			timeSinceLastCrash := time.Since(lastCrashTime).Seconds()
			lastCrashTime = time.Now()
			if timeSinceLastCrash > 3600 {
				// it has been one hour since the last crash.. reset the count
				// to reflect on the frequency
				restartCount = 1
			} else {
				restartCount++
			}
		}
	}()

	// Wait for server to start by launching a blocking connexion
	conn, err := dp.dial(dp.socket, 5*time.Second)
	if err != nil {
		return err
	}
	_ = conn.Close()

	return nil
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (dp *CarinaDevicePlugin) Register() error {
	conn, err := dp.dial(v1beta1.KubeletSocket, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := v1beta1.NewRegistrationClient(conn)
	reqt := &v1beta1.RegisterRequest{
		Version:      v1beta1.Version,
		Endpoint:     path.Base(dp.socket),
		ResourceName: dp.resourceName,
		Options: &v1beta1.DevicePluginOptions{
			GetPreferredAllocationAvailable: true,
		},
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

// GetDevicePluginOptions returns the values of the optional settings for this plugin
func (dp *CarinaDevicePlugin) GetDevicePluginOptions(context.Context, *v1beta1.Empty) (*v1beta1.DevicePluginOptions, error) {
	options := &v1beta1.DevicePluginOptions{
		GetPreferredAllocationAvailable: true,
	}
	return options, nil
}

// ListAndWatch lists devices and update that list according to the health status
func (dp *CarinaDevicePlugin) ListAndWatch(e *v1beta1.Empty, s v1beta1.DevicePlugin_ListAndWatchServer) error {

	request, err := dp.getDeviceCapacity()
	if err != nil {
		log.Errorf("get device capacity error: %s", err.Error())
		return err
	}

	_ = s.Send(&v1beta1.ListAndWatchResponse{Devices: request})

	for {
		select {
		case <-dp.stop:
			return nil
		case <-dp.update:
			request, err := dp.getDeviceCapacity()
			if err != nil {
				log.Errorf("get device capacity error: %s", err.Error())
				return err
			}
			log.Infof("update device capacity: %s", dp.resourceName)
			_ = s.Send(&v1beta1.ListAndWatchResponse{Devices: request})
		}
	}
}

// GetPreferredAllocation returns the preferred allocation from the set of devices specified in the request
func (m *CarinaDevicePlugin) GetPreferredAllocation(ctx context.Context, r *v1beta1.PreferredAllocationRequest) (*v1beta1.PreferredAllocationResponse, error) {
	response := &v1beta1.PreferredAllocationResponse{}
	for _, req := range r.ContainerRequests {
		resp := &v1beta1.ContainerPreferredAllocationResponse{
			DeviceIDs: req.MustIncludeDeviceIDs,
		}
		response.ContainerResponses = append(response.ContainerResponses, resp)
	}
	return response, nil
}

// Allocate which return list of devices.
func (m *CarinaDevicePlugin) Allocate(ctx context.Context, reqs *v1beta1.AllocateRequest) (*v1beta1.AllocateResponse, error) {
	responses := v1beta1.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		log.Infof("ignore  implement %s", req.DevicesIDs)
		response := v1beta1.ContainerAllocateResponse{}
		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}
	return &responses, nil
}

// PreStartContainer is unimplemented for this plugin
func (m *CarinaDevicePlugin) PreStartContainer(context.Context, *v1beta1.PreStartContainerRequest) (*v1beta1.PreStartContainerResponse, error) {
	return &v1beta1.PreStartContainerResponse{}, nil
}

// dial establishes the gRPC communication with the registered device plugin.
func (m *CarinaDevicePlugin) dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	c, err := grpc.DialContext(ctx, unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithContextDialer(func(i context.Context, addr string) (conn net.Conn, e error) {
			return net.DialTimeout("unix", addr, timeout)
		}))

	if err != nil {
		return nil, err
	}

	return c, nil
}

func (dp *CarinaDevicePlugin) getDeviceCapacity() ([]*v1beta1.Device, error) {
	var pdevs []*v1beta1.Device
	vgs, err := dp.volumeManager.GetCurrentVgStruct()
	if err != nil {
		return pdevs, err
	}
	// 对于此种存储设备实际上不适合采用device plugin模式
	// device plugin 通过ListAndWatch方法上报数据只包含设备ID，而对于存储来说，只上报一个设备ID显然不满足我们需求，我们需要总容量及可用量
	// 关于Kubelet如何计算设备数量参考，kubernetes/pkg/kubelet/cm/devicemanager/manager.go：GetCapacity():536
	// Kubelet实质只是获取一下设备数量len(v1betal.Device)，就像8个gpu
	// 基于此需要展示我们的黑科技了，基于容量构建一个数组，为了避免数组太大，以G为单位

	var capacity types.VgGroup
	for _, v := range vgs {
		if strings.HasSuffix(dp.resourceName, v.VGName) {
			capacity = v
		}
	}

	if capacity.VGName == "" {
		return pdevs, nil
	}

	sizeGb := capacity.VGSize>>30 + 1
	freeGb := uint64(0)
	if capacity.VGFree > utils.DefaultReservedSpace {
		freeGb = (capacity.VGFree - utils.DefaultReservedSpace) >> 30
	}

	// Capacity 这个是设备总资源数
	// Allocatable 这个是资源可使用数，调度器使用这个指标，它的值是总量-预留-已使用
	// 我们将已经使用的磁盘容量标记为unhealthy状态，如此在Node信息中看到allocatable在不断减少
	for i := uint64(1); i <= sizeGb; i++ {
		health := v1beta1.Healthy
		if i > freeGb {
			health = v1beta1.Unhealthy
		}
		pdevs = append(pdevs, &v1beta1.Device{
			ID:     fmt.Sprintf("%d", i),
			Health: health,
		})
	}

	return pdevs, nil
}
