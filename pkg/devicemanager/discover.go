package deviceManager

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"regexp"
	"strings"
	"syscall"
	"time"

	"carina/utils/log"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	discoverDaemonUdev = "DISCOVER_DAEMON_UDEV_BLACKLIST"
)

var (
	// AppName is the name of the pod
	AppName = "rook-discover"
	// NodeAttr is the attribute of that node
	NodeAttr = "rook.io/node"
	// LocalDiskCMData is the data name of the config map storing devices
	LocalDiskCMData = "devices"
	// LocalDiskCMName is name of the config map storing devices
	LocalDiskCMName = "local-device-%s"
	nodeName        string
	namespace       string
	lastDevice      string
	cmName          string
	cm              *v1.ConfigMap
	udevEventPeriod = time.Duration(5) * time.Second
	useCVInventory  bool
)

type CephVolumeInventory struct {
	Path            string          `json:"path"`
	Available       bool            `json:"available"`
	RejectedReasons json.RawMessage `json:"rejected_reasons"`
	SysAPI          json.RawMessage `json:"sys_api"`
	LVS             json.RawMessage `json:"lvs"`
}

// Run is the entry point of that package execution
func Run(context *Context, probeInterval time.Duration, useCV bool) error {
	if context == nil {
		return fmt.Errorf("nil context")
	}
	log.Debugf("device discovery interval is %q", probeInterval.String())
	log.Debugf("use ceph-volume inventory is %t", useCV)
	nodeName = os.Getenv(k8sutil.NodeNameEnvVar)
	namespace = os.Getenv(k8sutil.PodNamespaceEnvVar)
	cmName = k8sutil.TruncateNodeName(LocalDiskCMName, nodeName)
	useCVInventory = useCV
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGTERM)

	err := updateDeviceCM(context)
	if err != nil {
		log.Infof("failed to update device configmap: %v", err)
		return err
	}

	udevEvents := make(chan string)
	go udevBlockMonitor(udevEvents, udevEventPeriod)
	for {
		select {
		case <-sigc:
			log.Infof("shutdown signal received, exiting...")
			return nil
		case <-time.After(probeInterval):
			if err := updateDeviceCM(context); err != nil {
				log.Errorf("failed to update device configmap during probe interval. %v", err)
			}
		case _, ok := <-udevEvents:
			if ok {
				log.Info("trigger probe from udev event")
				if err := updateDeviceCM(context); err != nil {
					log.Errorf("failed to update device configmap triggered from udev event. %v", err)
				}
			} else {
				log.Warnf("disabling udev monitoring")
				udevEvents = nil
			}
		}
	}
}

func matchUdevEvent(text string, matches, exclusions []string) (bool, error) {
	for _, match := range matches {
		matched, err := regexp.MatchString(match, text)
		if err != nil {
			return false, fmt.Errorf("failed to search string: %v", err)
		}
		if matched {
			hasExclusion := false
			for _, exclusion := range exclusions {
				matched, err = regexp.MatchString(exclusion, text)
				if err != nil {
					return false, fmt.Errorf("failed to search string: %v", err)
				}
				if matched {
					hasExclusion = true
					break
				}
			}
			if !hasExclusion {
				log.Infof("udevadm monitor: matched event: %s", text)
				return true, nil
			}
		}
	}
	return false, nil
}

// Scans `udevadm monitor` output for block sub-system events. Each line of
// output matching a set of substrings is sent to the provided channel. An event
// is returned if it passes any matches tests, and passes all exclusion tests.
func rawUdevBlockMonitor(c chan string, matches, exclusions []string) {
	defer close(c)

	// stdbuf -oL performs line buffered output
	cmd := exec.Command("stdbuf", "-oL", "udevadm", "monitor", "-u", "-k", "-s", "block")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Warnf("Cannot open udevadm stdout: %v", err)
		return
	}

	err = cmd.Start()
	if err != nil {
		log.Warnf("Cannot start udevadm monitoring: %v", err)
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		text := scanner.Text()
		log.Debugf("udevadm monitor: %s", text)
		match, err := matchUdevEvent(text, matches, exclusions)
		if err != nil {
			log.Warnf("udevadm filtering failed: %v", err)
			return
		}
		if match {
			c <- text
		}
	}

	if err := scanner.Err(); err != nil {
		log.Warnf("udevadm monitor scanner error: %v", err)
	}

	log.Info("udevadm monitor finished")
}

// Monitors udev for block device changes, and collapses these events such that
// only one event is emitted per period in order to deal with flapping.
func udevBlockMonitor(c chan string, period time.Duration) {
	defer close(c)
	var udevFilter []string

	// return any add or remove events, but none that match device mapper
	// events. string matching is case-insensitive
	events := make(chan string)

	// get discoverDaemonUdevBlacklist from the environment variable
	// if user doesn't provide any regex; generate the default regex
	// else use the regex provided by user
	discoverUdev := os.Getenv(discoverDaemonUdev)
	if discoverUdev == "" {
		discoverUdev = "(?i)dm-[0-9]+,(?i)rbd[0-9]+,(?i)nbd[0-9]+"
	}
	udevFilter = strings.Split(discoverUdev, ",")
	log.Infof("using the regular expressions %q", udevFilter)

	go rawUdevBlockMonitor(events,
		[]string{"(?i)add", "(?i)remove"},
		udevFilter)

	for {
		event, ok := <-events
		if !ok {
			return
		}
		timeout := time.NewTimer(period)
		for {
			select {
			case <-timeout.C:
				break
			case _, ok := <-events:
				if !ok {
					return
				}
				continue
			}
			break
		}
		c <- event
	}
}

func ignoreDevice(dev LocalDisk) bool {
	return strings.Contains(strings.ToUpper(dev.DevLinks), "USB")
}

func checkMatchingDevice(checkDev LocalDisk, devices []LocalDisk) *LocalDisk {
	for i, dev := range devices {
		if ignoreDevice(dev) {
			continue
		}
		// check if devices should be considered the same. the uuid can be
		// unstable, so we also use the reported serial and device name, which
		// appear to be more stable.
		if checkDev.UUID == dev.UUID {
			return &devices[i]
		}

		// on virt-io devices in libvirt, the serial is reported as an empty
		// string, so also account for that.
		if checkDev.Serial == dev.Serial && checkDev.Serial != "" {
			return &devices[i]
		}

		if checkDev.Name == dev.Name {
			return &devices[i]
		}
	}
	return nil
}

// note that the idea of equality here may not be intuitive. equality of device
// sets refers to a state in which no change has been observed between the sets
// of devices that would warrant changes to their consumption by storage
// daemons. for example, if a device appears to have been wiped vs a device
// appears to now be in use.
func checkDeviceListsEqual(oldDevs, newDevs []LocalDisk) bool {
	for _, oldDev := range oldDevs {
		if ignoreDevice(oldDev) {
			continue
		}
		match := checkMatchingDevice(oldDev, newDevs)
		if match == nil {
			// device has been removed
			return false
		}
		if !oldDev.Empty && match.Empty {
			// device has changed from non-empty to empty
			return false
		}
		if oldDev.Partitions != nil && match.Partitions == nil {
			return false
		}
		if string(oldDev.CephVolumeData) == "" && string(match.CephVolumeData) != "" {
			// return ceph volume inventory data was not enabled before
			return false
		}
	}

	for _, newDev := range newDevs {
		if ignoreDevice(newDev) {
			continue
		}
		match := checkMatchingDevice(newDev, oldDevs)
		if match == nil {
			// device has been added
			return false
		}
		// the matching case is handled in the previous join
	}

	return true
}

// DeviceListsEqual checks whether 2 lists are equal or not
func DeviceListsEqual(old, new string) (bool, error) {
	var oldDevs []LocalDisk
	var newDevs []LocalDisk

	err := json.Unmarshal([]byte(old), &oldDevs)
	if err != nil {
		return false, fmt.Errorf("cannot unmarshal devices: %+v", err)
	}

	err = json.Unmarshal([]byte(new), &newDevs)
	if err != nil {
		return false, fmt.Errorf("cannot unmarshal devices: %+v", err)
	}

	return checkDeviceListsEqual(oldDevs, newDevs), nil
}

func updateDeviceCM(clusterdContext *Context) error {
	ctx := context.TODO()
	log.Infof("updating device configmap")
	devices, err := probeDevices(clusterdContext)
	if err != nil {
		log.Infof("failed to probe devices: %v", err)
		return err
	}
	deviceJSON, err := json.Marshal(devices)
	if err != nil {
		log.Infof("failed to marshal: %v", err)
		return err
	}

	deviceStr := string(deviceJSON)
	if cm == nil {
		cm, err = clusterdContext.Clientset.CoreV1().ConfigMaps(namespace).Get(ctx, cmName, metav1.GetOptions{})
	}
	if err == nil {
		lastDevice = cm.Data[LocalDiskCMData]
		log.Debugf("last devices %s", lastDevice)
	} else {
		if !kerrors.IsNotFound(err) {
			log.Infof("failed to get configmap: %v", err)
			return err
		}

		data := make(map[string]string, 1)
		data[LocalDiskCMData] = deviceStr

		// the map doesn't exist yet, create it now
		cm = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmName,
				Namespace: namespace,
				Labels: map[string]string{
					k8sutil.AppAttr: AppName,
					NodeAttr:        nodeName,
				},
			},
			Data: data,
		}

		// Get the discover daemon pod details to attach the owner reference to the config map
		discoverPod, err := k8sutil.GetRunningPod(clusterdContext.Clientset)
		if err != nil {
			log.Warnf("failed to get discover pod to set ownerref. %+v", err)
		} else {
			k8sutil.SetOwnerRefsWithoutBlockOwner(&cm.ObjectMeta, discoverPod.OwnerReferences)
		}

		cm, err = clusterdContext.Clientset.CoreV1().ConfigMaps(namespace).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			log.Infof("failed to create configmap: %v", err)
			return fmt.Errorf("failed to create local device map %s: %+v", cmName, err)
		}
		lastDevice = deviceStr
	}
	devicesEqual, err := DeviceListsEqual(lastDevice, deviceStr)
	if err != nil {
		return fmt.Errorf("failed to compare device lists: %v", err)
	}
	if !devicesEqual {
		data := make(map[string]string, 1)
		data[LocalDiskCMData] = deviceStr
		cm.Data = data
		cm, err = clusterdContext.Clientset.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
		if err != nil {
			log.Infof("failed to update configmap %s: %v", cmName, err)
			return err
		}
	}
	return nil
}

func logDevices(devices []*LocalDisk) {
	var devicesList []string
	for _, device := range devices {
		log.Debugf("localdevice %q: %+v", device.Name, device)
		devicesList = append(devicesList, device.Name)
	}
	log.Infof("localdevices: %q", strings.Join(devicesList, ", "))
}

func probeDevices(context *Context) ([]LocalDisk, error) {
	devices := make([]LocalDisk, 0)
	localDevices, err := DiscoverDevices(context.Executor)
	if err != nil {
		return devices, fmt.Errorf("failed initial hardware discovery. %+v", err)
	}

	logDevices(localDevices)

	// ceph-volume inventory command takes a little time to complete.
	// Get this data only if it is needed and once by function execution
	var cvInventory *map[string]string = nil
	if useCVInventory {
		log.Infof("Getting ceph-volume inventory information")
		cvInventory, err = getCephVolumeInventory(context)
		if err != nil {
			log.Errorf("error getting ceph-volume inventory: %v", err)
		}
	}

	for _, device := range localDevices {
		if device == nil {
			continue
		}
		if device.Type == PartType {
			continue
		}

		partitions, _, err := GetDevicePartitions(device.Name, context.Executor)
		if err != nil {
			log.Infof("failed to check device partitions %s: %v", device.Name, err)
			continue
		}

		// check if there is a file system on the device
		fs, err := GetDeviceFilesystems(device.Name, context.Executor)
		if err != nil {
			log.Infof("failed to check device filesystem %s: %v", device.Name, err)
			continue
		}
		device.Partitions = partitions
		device.Filesystem = fs
		device.Empty = GetDeviceEmpty(device)

		// Add the information provided by ceph-volume inventory
		if cvInventory != nil {
			CVData, deviceExists := (*cvInventory)[path.Join("/dev/", device.Name)]
			if deviceExists {
				device.CephVolumeData = CVData
			} else {
				log.Errorf("ceph-volume information for device %q not found", device.Name)
			}
		} else {
			device.CephVolumeData = ""
		}

		devices = append(devices, *device)
	}

	log.Infof("available devices: %+v", devices)
	return devices, nil
}

// getCephVolumeInventory: Return a map of strings indexed by device with the
// information about the device returned by the command <ceph-volume inventory>
func getCephVolumeInventory(context *Context) (*map[string]string, error) {
	inventory, err := context.Executor.ExecuteCommandWithOutput("ceph-volume", "inventory", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to execute ceph-volume inventory. %+v", err)
	}

	// Return a map with the information of each device indexed by path
	CVDevices := make(map[string]string)

	// No data retrieved from ceph-volume
	if inventory == "" {
		return &CVDevices, nil
	}

	// Get a slice to store the json data
	bInventory := []byte(inventory)
	var CVInventory []CephVolumeInventory
	err = json.Unmarshal(bInventory, &CVInventory)
	if err != nil {
		return &CVDevices, fmt.Errorf("error unmarshalling json data coming from ceph-volume inventory. %v", err)
	}

	for _, device := range CVInventory {
		jsonData, err := json.Marshal(device)
		if err != nil {
			log.Errorf("error marshaling json data for device: %v", device.Path)
		} else {
			CVDevices[device.Path] = string(jsonData)
		}
	}

	return &CVDevices, nil
}
