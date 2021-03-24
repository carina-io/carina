package configuration

import (
	"bocloud.com/cloudnative/carina/utils"
	"bocloud.com/cloudnative/carina/utils/log"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"os"
	"strings"
)

// 配置文件路径
const (
	configPath        = "/etc/carina/"
	SchedulerBinpack  = "binpack"
	SchedulerSpradout = "spradout"
	diskGroupType     = "type"
)

var TestAssistDiskSelector []string

// 提供给其他应用获取服务数据
// 这个configMap理论上应该由Node Server更新，为了实现简单改为有Control Server更新，遍历所有Node信息更新configmap
// 暂定这些参数字段，不排除会增加一些需要暴露的数据
type ConfigProvider struct {
	NodeName string   `json:"nodeName"`
	Vg       []string `json:"vg"`
}

var GlobalConfig *viper.Viper

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
	err := GlobalConfig.ReadInConfig()
	if err != nil {
		log.Error("Failed to get the configuration")
		os.Exit(-1)
	}
	return GlobalConfig
}

func dynamicConfig() {
	GlobalConfig.WatchConfig()
	GlobalConfig.OnConfigChange(func(event fsnotify.Event) {
		log.Infof("Detect config change: %s", event.String())
	})
}

// 支持正则表达式
// 定时扫描本地磁盘，凡是匹配的将被加入到相应vg卷组
// 对于此配置的修改需要非常慎重，如果更改匹配条件，可能会移除正在使用的磁盘
func DiskSelector() []string {
	// 测试辅助变量，这里入侵了业务逻辑
	if len(TestAssistDiskSelector) > 0 {
		return TestAssistDiskSelector
	}
	diskSelector := GlobalConfig.GetStringSlice("diskSelector")
	if len(diskSelector) == 0 {
		log.Warn("No device is initialized because there is no configuration")
	}
	return diskSelector
}

// 定时磁盘扫描时间间隔(秒),默认300s
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

// 磁盘分组策略，目前只支持根据磁盘类型分组
func DiskGroupPolicy() string {
	diskGroupPolicy := GlobalConfig.GetString("diskGroupPolicy")
	diskGroupPolicy = "type"
	return diskGroupPolicy

}

// pv调度策略binpac/spradout，默认为binpac
func SchedulerStrategy() string {
	schedulerStrategy := GlobalConfig.GetString("schedulerStrategy")
	if utils.ContainsString([]string{SchedulerBinpack, SchedulerSpradout}, strings.ToLower(schedulerStrategy)) {
		schedulerStrategy = strings.ToLower(schedulerStrategy)
	} else {
		schedulerStrategy = SchedulerSpradout
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
