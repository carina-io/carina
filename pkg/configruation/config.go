package configruation

import (
	"carina/utils"
	"carina/utils/log"
	"github.com/spf13/viper"
	"strconv"
	"strings"
)

// 配置文件路径
const (
	configPath        = "/etc/carina/config.json"
	SchedulerBinpack  = "binpack"
	SchedulerSpradout = "spradout"
	diskGroupType     = "type"
)

// 全局配置
type Config struct {
	// 支持正则表达式
	// 定时扫描本地磁盘，凡是匹配的将被加入到相应vg卷组
	// 对于此配置的修改需要非常慎重，如果更改匹配条件，可能会移除正在使用的磁盘
	DiskSelector []string `json:"diskSelector"`
	// 定时磁盘扫描时间间隔(秒),默认60s
	DiskScanInterval string `json:"diskScanInterval"`
	// 磁盘分组策略，目前只支持根据磁盘类型分组
	DiskGroupPolicy string `json:"diskGroupPolicy"`
	// pv调度策略binpac/spradout，默认为binpac
	SchedulerStrategy string `json:"schedulerStrategy"`
}

// 提供给其他应用获取服务数据
// 这个configMap理论上应该由Node Server更新，为了实现简单改为有Control Server更新，遍历所有Node信息更新configmap
// 暂定这些参数字段，不排除会增加一些需要暴露的数据
type ConfigProvider struct {
	NodeIp string   `json:"nodeip"`
	Vg     []string `json:"vg"`
}

var GlobalConfig *Config

func LoadConfig() error {

	config := viper.New()
	config.AddConfigPath(configPath)
	config.SetConfigName("config")
	config.SetConfigType("json")
	if err := config.ReadInConfig(); err != nil {
		return err
	}
	var c Config
	if err := config.Unmarshal(&c); err != nil {
		return err
	}
	verifyConfig(&c)
	GlobalConfig = &c
	return nil
}

func verifyConfig(c *Config) {

	di, err := strconv.Atoi(c.DiskScanInterval)
	if err != nil {
		c.DiskScanInterval = "60"
	}
	if di < 60 {
		c.DiskScanInterval = "60"
	}

	if utils.IsContainsString([]string{SchedulerBinpack, SchedulerSpradout}, strings.ToLower(c.SchedulerStrategy)) {
		c.SchedulerStrategy = strings.ToLower(c.SchedulerStrategy)
	} else {
		c.SchedulerStrategy = SchedulerBinpack
	}
	// 目前只支持一种
	c.DiskGroupPolicy = diskGroupType

	if len(c.DiskSelector) == 0 {
		log.Warn("No device is initialized because there is no configuration")
	}
}
