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

package deviceplugin

import (
	"github.com/carina-io/carina/pkg/configuration"
	"github.com/carina-io/carina/pkg/devicemanager/volume"
	"github.com/carina-io/carina/pkg/deviceplugin/v1beta1"
	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	// 依赖冲突，把整个proto目录挪移过来
	//pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

func Run(nodeName string, cache cache.Cache, volumeManager volume.LocalVolume, stopChan <-chan struct{}) {
	cache.WaitForCacheSync(context.Background())
	watcher, err := newFSWatcher(v1beta1.DevicePluginPath)
	if err != nil {
		log.Errorf("Failed to create FS watcher: %v", err)
		os.Exit(-1)
	}

	defer watcher.Close()

	var c = make(chan struct{}, 1)
	configuration.RegisterListenerChan(c)

	plugins := []*CarinaDevicePlugin{}
restart:
	for _, p := range plugins {
		_ = p.Stop()
	}

	log.Info("Retreiving plugins.")
	_, diskClass := getDiskGroup(nodeName, cache)
	log.Debug("diskClass:", diskClass)
	for _, d := range diskClass {

		c := make(chan struct{}, 5)
		plugins = append(plugins, NewCarinaDevicePlugin(
			utils.DeviceCapacityKeyPrefix+d,
			volumeManager,
			c,
			v1beta1.DevicePluginPath+d+".sock",
		))
		// 注册通知服务
		log.Info("register volume notice server.", d)
		volumeManager.RegisterNoticeServer(d, c)
	}

	started := 0
	pluginStartError := make(chan struct{})
	for _, p := range plugins {
		if err := p.Start(); err != nil {
			log.Error("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
			close(pluginStartError)
			goto events
		}
		started++
	}

	if started == 0 {
		log.Info("No devices found, Waiting indefinitely.")
	}

events:
	// Start an infinite loop, waiting for several indicators to either log
	// some messages, trigger a restart of the plugins, or exit the program.
	for {
		select {
		// If there was an error starting any plugins, restart them all.
		case <-pluginStartError:
			goto restart

		// Detect a kubelet restart by watching for a newly created
		// 'pluginapi.KubeletSocket' file. When this occurs, restart this loop,
		// restarting all of the plugins in the process.
		case event := <-watcher.Events:
			if event.Name == v1beta1.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				log.Infof("inotify: %s created, restarting.", v1beta1.KubeletSocket)
				goto restart
			}
		//configModifyNoticeToPlugin
		case <-c:
			log.Info("inotify: config change event")
			need, _ := getDiskGroup(nodeName, cache)
			if need {
				log.Info("Restart the service because the device plug-in has changed")
				goto restart
			}
		// Watch for any other fs errors and log them.
		case err := <-watcher.Errors:
			log.Infof("inotify: %s", err)

		case <-stopChan:
			for _, p := range plugins {
				_ = p.stop
			}
			return
		}
	}
}

func getDiskGroup(nodeName string, cache cache.Cache) (bool, []string) {
	diskClass := []string{}
	currentDiskSelector := configuration.DiskSelector()
	log.Debugf("get config disk %s", currentDiskSelector)
	node := &corev1.Node{}
	err := cache.Get(context.Background(), client.ObjectKey{Name: nodeName}, node)
	if err != nil {
		log.Errorf("get node %s error %s", node, err.Error())
		return false, diskClass
	}
	for _, v := range currentDiskSelector {
		if v.NodeLabel == "" {
			diskClass = append(diskClass, v.Name)
			continue
		}
		if _, ok := node.Labels[v.NodeLabel]; ok {
			diskClass = append(diskClass, v.Name)
			continue
		}
	}
	//
	currentClass := []string{}
	for key, _ := range node.Status.Capacity {
		if strings.HasPrefix(string(key), utils.DeviceCapacityKeyPrefix) {
			sf := strings.Split(string(key), "/")[1]
			currentClass = append(currentClass, sf)
		}
	}
	return !utils.SliceEqualSlice(currentClass, diskClass), diskClass
}
