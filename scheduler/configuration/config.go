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

package configuration

import (
	"fmt"
	"k8s.io/klog/v2"
	"os"
	"reflect"
	"strings"

	"github.com/carina-io/carina/scheduler/utils"
	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// 配置文件路径
const (
	configPath         = "/etc/carina/"
	SchedulerBinpack   = "binpack"
	Schedulerspreadout = "spreadout"
)

var testAssistDiskSelector []string
var configModifyNotice []chan<- struct{}
var err error
var GlobalConfig *viper.Viper
var diskConfig Disk
var opt = viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
	mapstructure.StringToTimeDurationHookFunc(),
	mapstructure.StringToSliceHookFunc(","),
	// Custom Decode Hook Function
	func(rf reflect.Kind, rt reflect.Kind, data interface{}) (interface{}, error) {
		if rf != reflect.Map || rt != reflect.Struct {
			return data, nil
		}
		mapstructure.Decode(data.(map[string]interface{}), &diskConfig)
		mapstructure.Decode(data.(map[string]interface{})["diskselector"], &diskConfig.DiskSelectors)
		return data, err
	},
))

type DiskSelectorItem struct {
	Name      string   `json:"name"`
	Re        []string `json:"re"`
	Policy    string   `json:"policy"`
	NodeLabel string   `json:"nodeLabel"`
}

type Disk struct {
	DiskSelectors     []DiskSelectorItem `json:"diskSelectors"`
	DiskScanInterval  int64              `json:"diskScanInterval"`
	SchedulerStrategy string             `json:"schedulerStrategy"`
}

type DiskClass struct {
	DiskClassByName map[string]DiskSelectorItem `json:"diskClassByName"`
}

func init() {
	GlobalConfig = initConfig()
	go dynamicConfig()

}

func initConfig() *viper.Viper {
	GlobalConfig := viper.New()
	GlobalConfig.AddConfigPath(configPath)
	GlobalConfig.SetConfigName("config")
	GlobalConfig.SetConfigType("json")
	if err := GlobalConfig.ReadInConfig(); err != nil {
		klog.Errorf("Failed to get the configuration: %s", err)
		os.Exit(-1)
	}

	if err := GlobalConfig.Unmarshal(&diskConfig, opt); err != nil {
		klog.Errorf("Failed to unmarshal the configuration： %s", err)
		os.Exit(-1)
	}
	return GlobalConfig
}

func dynamicConfig() {
	GlobalConfig.WatchConfig()
	GlobalConfig.OnConfigChange(func(event fsnotify.Event) {
		if err := GlobalConfig.Unmarshal(&diskConfig, opt); err != nil {
			klog.Errorf("Failed to unmarshal the configuration： %s", err)
		}
	})
}

// SchedulerStrategy pv调度策略binpac/spreadout，默认为binpac
func SchedulerStrategy() string {
	schedulerStrategy := GlobalConfig.GetString("schedulerStrategy")
	if utils.ContainsString([]string{SchedulerBinpack, Schedulerspreadout}, strings.ToLower(schedulerStrategy)) {
		schedulerStrategy = strings.ToLower(schedulerStrategy)
	} else {
		schedulerStrategy = SchedulerBinpack
	}
	return schedulerStrategy
}

// GetDeviceGroup 处理磁盘类型参数，支持carina.storage.io/disk-group-name:ssd书写方式
func GetDeviceGroup(diskType string) string {
	deviceGroup := strings.ToLower(diskType)
	diskSelector := diskConfig.DiskSelectors
	for _, d := range diskSelector {
		if strings.ToLower(d.Policy) == "raw" {
			continue
		}
		//如果sc 配置的磁盘组在配置里就默认返回配置的磁盘组，老板本的磁盘组如果在新配置文件里配置了，就采用新的配置
		if d.Name == diskType {
			return diskType
		}
	}

	//这里是为了兼容旧版本的sc
	if utils.ContainsString([]string{"ssd", "hdd"}, deviceGroup) {
		deviceGroup = fmt.Sprintf("carina-vg-%s", deviceGroup)
	}
	return deviceGroup

}

func CheckRawDeviceGroup(diskType string) bool {
	deviceGroup := strings.ToLower(diskType)
	currentDiskSelector := diskConfig.DiskSelectors
	if utils.ContainsString([]string{"ssd", "hdd"}, deviceGroup) {
		deviceGroup = fmt.Sprintf("carina-vg-%s", deviceGroup)
	}

	for _, v := range currentDiskSelector {
		if v.Name == deviceGroup && strings.ToLower(v.Policy) == "raw" {
			return true
		}

	}
	return false
}
