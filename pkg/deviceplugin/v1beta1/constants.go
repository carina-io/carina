package v1beta1

const (
	// Healthy means that the device is healthy
	Healthy = "Healthy"
	// UnHealthy means that the device is unhealthy
	Unhealthy = "Unhealthy"

	// Current version of the API supported by kubelet
	Version = "v1beta1"
	// DevicePluginPath is the folder the Device Plugin is expecting sockets to be on
	// Only privileged pods have access to this path
	// Note: Placeholder until we find a "standard path"
	DevicePluginPath = "/var/lib/kubelet/device-plugins/"
	// KubeletSocket is the path of the Kubelet registry socket
	KubeletSocket = DevicePluginPath + "kubelet.sock"
	// Timeout duration in secs for PreStartContainer RPC
	KubeletPreStartContainerRPCTimeoutInSecs = 30
)

var SupportedVersions = [...]string{"v1beta1"}
