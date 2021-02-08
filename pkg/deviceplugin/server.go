package deviceplugin

import (
	"carina/pkg/devicemanager/types"
	"carina/pkg/devicemanager/volume"
	"carina/pkg/deviceplugin/v1beta1"
	"carina/utils"
	"carina/utils/log"
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
	update        <-chan struct{}
	stop          chan interface{}
}

// NewCarinaDevicePlugin returns an initialized CarinaDevicePlugin
func NewCarinaDevicePlugin(resourceName string, volumeManager volume.LocalVolume, socket string) *CarinaDevicePlugin {
	return &CarinaDevicePlugin{
		resourceName:  resourceName,
		volumeManager: volumeManager,
		socket:        socket,

		// These will be reinitialized every
		// time the plugin server is restarted.
		server: nil,
		//health:   nil,
		stop: nil,
	}
}

func (m *CarinaDevicePlugin) cleanup() {
	close(m.stop)
	m.server = nil
	m.stop = nil
}

// Start starts the gRPC server, registers the device plugin with the Kubelet,
// and starts the device healthchecks.
func (m *CarinaDevicePlugin) Start() error {
	m.server = grpc.NewServer([]grpc.ServerOption{}...)
	m.stop = make(chan interface{})

	err := m.Serve()
	if err != nil {
		log.Errorf("Could not start device plugin for '%s': %s", m.resourceName, err)
		m.cleanup()
		return err
	}
	log.Infof("Starting to serve '%s' on %s", m.resourceName, m.socket)

	err = m.Register()
	if err != nil {
		log.Errorf("Could not register device plugin: %s", err)
		_ = m.Stop()
		return err
	}
	log.Infof("Registered device plugin for '%s' with Kubelet", m.resourceName)

	//go m.CheckHealth(m.stopChan, m.cachedDevices, m.health)

	return nil
}

// Stop stops the gRPC server.
func (m *CarinaDevicePlugin) Stop() error {
	if m == nil || m.server == nil {
		return nil
	}
	log.Infof("Stopping to serve '%s' on %s", m.resourceName, m.socket)
	m.server.Stop()
	if err := os.Remove(m.socket); err != nil && !os.IsNotExist(err) {
		return err
	}
	m.cleanup()
	return nil
}

// Serve starts the gRPC server of the device plugin.
func (m *CarinaDevicePlugin) Serve() error {
	_ = os.Remove(m.socket)
	sock, err := net.Listen("unix", m.socket)
	if err != nil {
		return err
	}

	v1beta1.RegisterDevicePluginServer(m.server, m)

	go func() {
		lastCrashTime := time.Now()
		restartCount := 0
		for {
			log.Infof("Starting GRPC server for '%s'", m.resourceName)
			err := m.server.Serve(sock)
			if err == nil {
				break
			}
			log.Infof("GRPC server for '%s' crashed with error: %v", m.resourceName, err)

			// restart if it has not been too often
			// i.e. if server has crashed more than 5 times and it didn't last more than one hour each time
			if restartCount > 5 {
				// quit
				log.Fatalf("GRPC server for '%s' has repeatedly crashed recently. Quitting", m.resourceName)
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
	conn, err := m.dial(m.socket, 5*time.Second)
	if err != nil {
		return err
	}
	_ = conn.Close()

	return nil
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (m *CarinaDevicePlugin) Register() error {
	conn, err := m.dial(v1beta1.KubeletSocket, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := v1beta1.NewRegistrationClient(conn)
	reqt := &v1beta1.RegisterRequest{
		Version:      v1beta1.Version,
		Endpoint:     path.Base(m.socket),
		ResourceName: m.resourceName,
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
func (m *CarinaDevicePlugin) GetDevicePluginOptions(context.Context, *v1beta1.Empty) (*v1beta1.DevicePluginOptions, error) {
	options := &v1beta1.DevicePluginOptions{
		GetPreferredAllocationAvailable: true,
	}
	return options, nil
}

// ListAndWatch lists devices and update that list according to the health status
func (m *CarinaDevicePlugin) ListAndWatch(e *v1beta1.Empty, s v1beta1.DevicePlugin_ListAndWatchServer) error {

	request, err := m.getDeviceCapacity()
	if err != nil {
		log.Errorf("get device capacity error: %s", err.Error())
		return err
	}

	_ = s.Send(&v1beta1.ListAndWatchResponse{Devices: request})

	for {
		select {
		case <-m.stop:
			return nil
		case <-m.update:
			request, err := m.getDeviceCapacity()
			if err != nil {
				log.Errorf("get device capacity error: %s", err.Error())
				return err
			}
			log.Info("update device capacity: %s", m.resourceName)
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

func (m *CarinaDevicePlugin) getDeviceCapacity() ([]*v1beta1.Device, error) {
	var pdevs []*v1beta1.Device
	vgs, err := m.volumeManager.GetCurrentVgStruct()
	if err != nil {
		return pdevs, err
	}
	// 对于此种存储设备实际上不适合采用device plugin模式
	// device plugin 通过ListAndWatch方法上报对只包含设备ID，而对于存储来说，只上报一个设备者显然不满足我们对需求，我们需要的是设备总容量
	// 关于如何计算设备数量参考，kubernetes/pkg/kubelet/cm/devicemanager/manager.go：GetCapacity():536，只是获取了一下数组长度
	// 基于此展示我们黑科技了，根基容量构建一个数组，为了避免数组太大，以G为单位

	var capacity types.VgGroup
	for _, v := range vgs {
		if strings.HasSuffix(m.resourceName, v.VGName) {
			capacity = v
		}
	}

	if capacity.VGName == "" {
		return pdevs, nil
	}

	sizeGb := 1 + capacity.VGSize>>30
	freeGb := utils.DefaultReservedSpace + capacity.VGFree>>30

	// Capacity 为总容量，这个是硬件总资源数
	// Allocatable 为可用容量这个是资源可使用数，调度器使用对这个指标
	// 我们将已经使用对磁盘容量标记为unhealthy状态，如此变成在Node信息中看到allocatable在不断减少
	for i := uint64(0); i < sizeGb; i++ {
		health := v1beta1.Healthy
		if i > freeGb {
			health = v1beta1.Unhealthy
		}
		pdevs = append(pdevs, &v1beta1.Device{
			ID:     string(i),
			Health: health,
		})
	}

	return pdevs, nil
}
