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
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/carina-io/carina/utils"
	"github.com/carina-io/carina/utils/log"
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

var TestAssistDiskSelector []string
var configModifyNotice []chan<- struct{}
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
		diskConfig.DiskSelectors = []DiskSelectorItem{}
		mapstructure.Decode(data.(map[string]interface{})["diskselector"], &diskConfig.DiskSelectors)
		return data, nil
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

func init() {
	log.Info("Loading global configuration ...")
	GlobalConfig = initConfig()
	go dynamicConfig()

}

func initConfig() *viper.Viper {
	GlobalConfig := viper.New()
	GlobalConfig.AddConfigPath(configPath)
	GlobalConfig.SetConfigName("config")
	GlobalConfig.SetConfigType("json")
	if err := GlobalConfig.ReadInConfig(); err != nil {
		log.Errorf("Failed to get the configuration: %s", err)
		os.Exit(-1)
	}

	if err := GlobalConfig.Unmarshal(&diskConfig, opt); err != nil {
		log.Errorf("Failed to unmarshal the configuration： %s", err)
		os.Exit(-1)
	}

	if err := validate(diskConfig); err != nil {
		log.Errorf("Failed to validate the configuration: %s", err)
		os.Exit(-1)
	}

	return GlobalConfig
}

func dynamicConfig() {
	GlobalConfig.WatchConfig()
	GlobalConfig.OnConfigChange(func(event fsnotify.Event) {
		log.Infof("Detect config change: %s", event.String())
		if err := GlobalConfig.Unmarshal(&diskConfig, opt); err != nil {
			log.Errorf("Failed to unmarshal the configuration: %s, ignore this change", err)
			return
		}
		if err := validate(diskConfig); err != nil {
			log.Errorf("Failed to validate the configuration: %s, ignore this change", err)
			return
		}
		for _, c := range configModifyNotice {
			log.Info("Generates the configuration change event")
			c <- struct{}{}
		}
	})
}

func RegisterListenerChan(c chan<- struct{}) {
	configModifyNotice = append(configModifyNotice, c)
}

// DiskSelector 支持正则表达式
// 定时扫描本地磁盘，凡是匹配的将被加入到相应vg卷组
// 对于此配置的修改需要非常慎重，如果更改匹配条件，可能会移除正在使用的磁盘
func DiskSelector() []DiskSelectorItem {
	// 测试辅助变量，这里入侵了业务逻辑
	if len(TestAssistDiskSelector) > 0 {
		return []DiskSelectorItem{}
	}

	diskSelector := diskConfig.DiskSelectors
	if len(diskSelector) == 0 {
		log.Warn("No device is initialized because disk selector is no configuration")
	}
	return diskSelector
}

// DiskScanInterval 定时磁盘扫描时间间隔(秒),默认300s
func DiskScanInterval() int64 {
	diskScanInterval := GlobalConfig.GetInt64("diskScanInterval")
	if diskScanInterval == 0 {
		return 0
	}
	if diskScanInterval < 300 {
		diskScanInterval = 300
	}
	return diskScanInterval
}

// SchedulerStrategy pv调度策略binpack/spreadout，默认为binpack
func SchedulerStrategy() string {
	schedulerStrategy := GlobalConfig.GetString("schedulerStrategy")
	if utils.ContainsString([]string{SchedulerBinpack, Schedulerspreadout}, strings.ToLower(schedulerStrategy)) {
		schedulerStrategy = strings.ToLower(schedulerStrategy)
	} else {
		schedulerStrategy = Schedulerspreadout
	}
	return schedulerStrategy
}

func RuntimeNamespace() string {
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}
	return namespace
}

func validate(disk Disk) error {
	vgGroup := make(map[string]bool)
	var diskNameRegexp = regexp.MustCompile("^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$")
	var diskScanRegexp = regexp.MustCompile("(?i)^([0-9]*)?$")
	var schedulerStrategyRegexp = regexp.MustCompile("(?i)^(spreadout|binpack)?$")

	if !diskScanRegexp.MatchString(strconv.FormatInt(disk.DiskScanInterval, 10)) {
		return fmt.Errorf("diskScanInterval must be a number: %s", strconv.FormatInt(disk.DiskScanInterval, 10))
	}
	if !schedulerStrategyRegexp.MatchString(disk.SchedulerStrategy) {
		return fmt.Errorf("SchedulerStrategy must either binpack or spradout : %s", disk.SchedulerStrategy)
	}
	for _, dc := range disk.DiskSelectors {
		if len(dc.Name) == 0 {
			return errors.New("disk name should not be empty")
		}
		if !diskNameRegexp.MatchString(dc.Name) {
			return fmt.Errorf("disk name should consist of alphanumeric characters, '-', '_' or '.', and should start and end with an alphanumeric character: %s", dc.Name)
		}
		if len(dc.Re) == 0 {
			log.Warnf("disk regexp should not be empty: %s", dc.Re)
		}
		if vgGroup[dc.Name] {
			return fmt.Errorf("duplicate vg group: %s", dc.Name)
		}
		vgGroup[dc.Name] = true
	}
	return nil
}

func GetRawDeviceGroupRe(diskType string) []string {
	deviceGroup := strings.ToLower(diskType)
	currentDiskSelector := diskConfig.DiskSelectors
	if utils.ContainsString([]string{"ssd", "hdd"}, deviceGroup) {
		deviceGroup = fmt.Sprintf("carina-vg-%s", deviceGroup)
	}

	for _, v := range currentDiskSelector {
		if v.Name == deviceGroup && strings.ToLower(v.Policy) == "raw" {
			return v.Re
		}

	}
	return nil
}
