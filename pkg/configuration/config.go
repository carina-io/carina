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
	configPath        = "/etc/carina/"
	SchedulerBinpack  = "binpack"
	Schedulerspreadout = "spreadout"
)

var TestAssistDiskSelector []string
var configModifyNotice []chan<- struct{}
var err error
var GlobalConfig *viper.Viper
var DiskConfig Disk
var opt = viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
	mapstructure.StringToTimeDurationHookFunc(),
	mapstructure.StringToSliceHookFunc(","),
	// Custom Decode Hook Function
	func(rf reflect.Kind, rt reflect.Kind, data interface{}) (interface{}, error) {
		if rf != reflect.Map || rt != reflect.Struct {
			return data, nil
		}
		mapstructure.Decode(data.(map[string]interface{}), &DiskConfig)
		mapstructure.Decode(data.(map[string]interface{})["diskselector"], &DiskConfig.DiskSelectors)
		return data, err
	},
))

// ConfigProvider 提供给其他应用获取服务数据
// 这个configMap理论上应该由Node Server更新，为了实现简单改为有Control Server更新，遍历所有Node信息更新configmap
// 暂定这些参数字段，不排除会增加一些需要暴露的数据
type ConfigProvider struct {
	NodeName string   `json:"nodeName"`
	Vg       []string `json:"vg"`
}

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
	log.Info("Loading global configuration ...")
	GlobalConfig = initConfig()
	Validate(DiskConfig)
	go dynamicConfig()

}

func initConfig() *viper.Viper {
	GlobalConfig := viper.New()
	GlobalConfig.AddConfigPath(configPath)
	GlobalConfig.SetConfigName("config")
	GlobalConfig.SetConfigType("json")
	err := GlobalConfig.ReadInConfig()
	if err != nil {
		log.Error("Failed to get the configuration", err)
		os.Exit(-1)
	}
	err = GlobalConfig.Unmarshal(&DiskConfig, opt)
	if err != nil {
		log.Error("Failed to unmarshal the configuration")
		os.Exit(-1)
	}

	return GlobalConfig
}

func dynamicConfig() {
	GlobalConfig.WatchConfig()
	GlobalConfig.OnConfigChange(func(event fsnotify.Event) {
		log.Infof("Detect config change: %s", event.String())
		for _, c := range configModifyNotice {
			log.Info("generates the configuration change event")
			err = GlobalConfig.Unmarshal(&DiskConfig, opt)
			if err != nil {
				log.Errorf("Failed to unmarshal the configuration:%s", err)
			}
			err = Validate(DiskConfig)
			if err != nil {
				log.Errorf("Failed to validate the configuration%s", err)
			}
			c <- struct{}{}
		}
	})
}

func RegisterListenerChan(c chan<- struct{}) {
	configModifyNotice = append(configModifyNotice, c)
}

func NewDiskClass(diskSelectors []DiskSelectorItem) *DiskClass {
	disk := DiskClass{}
	disk.DiskClassByName = make(map[string]DiskSelectorItem)
	for _, d := range diskSelectors {
        if d.Policy == "RAW" {
			continue
		}
		disk.DiskClassByName[d.Name] = d
	}
	return &disk
}

// DiskSelector 支持正则表达式
// 定时扫描本地磁盘，凡是匹配的将被加入到相应vg卷组
// 对于此配置的修改需要非常慎重，如果更改匹配条件，可能会移除正在使用的磁盘
func DiskSelector() []DiskSelectorItem {
	// 测试辅助变量，这里入侵了业务逻辑
	if len(TestAssistDiskSelector) > 0 {
		return []DiskSelectorItem{}
	}

	diskSelector := DiskConfig.DiskSelectors
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

// SchedulerStrategy pv调度策略binpac/spreadout，默认为binpac
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

func Validate(disk Disk) error {
	dcNames := make(map[string]bool)
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
			return fmt.Errorf("disk regexp should not be empty: %s", dc.Re)
		}

		if dcNames[dc.Name] {
			return fmt.Errorf("duplicate disk name: %s", dc.Name)
		}
		dcNames[dc.Name] = true

	}
	return nil
}
