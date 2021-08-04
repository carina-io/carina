package configuration

import (
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"os"
	"github.com/bocloud/carina/scheduler/utils"
	"strings"
)

// 配置文件路径
const (
	configPath        = "/etc/carina/"
	SchedulerBinpack  = "binpack"
	SchedulerSpradout = "spradout"
)

var GlobalConfig *viper.Viper

func init() {
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
		os.Exit(-1)
	}
	return GlobalConfig
}

func dynamicConfig() {
	GlobalConfig.WatchConfig()
	GlobalConfig.OnConfigChange(func(event fsnotify.Event) {
	})
}

// pv调度策略binpac/spradout，默认为binpac
func SchedulerStrategy() string {
	schedulerStrategy := GlobalConfig.GetString("schedulerStrategy")
	if utils.ContainsString([]string{SchedulerBinpack, SchedulerSpradout}, strings.ToLower(schedulerStrategy)) {
		schedulerStrategy = strings.ToLower(schedulerStrategy)
	} else {
		schedulerStrategy = SchedulerBinpack
	}
	return schedulerStrategy
}