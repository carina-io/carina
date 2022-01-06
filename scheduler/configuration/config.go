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
	"os"
	"strings"

	"github.com/carina-io/carina/scheduler/utils"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// 配置文件路径
const (
	configPath         = "/etc/carina/"
	SchedulerBinpack   = "binpack"
	Schedulerspreadout = "spreadout"
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

// pv调度策略binpac/spreadout，默认为binpac
func SchedulerStrategy() string {
	schedulerStrategy := GlobalConfig.GetString("schedulerStrategy")
	if utils.ContainsString([]string{SchedulerBinpack, Schedulerspreadout}, strings.ToLower(schedulerStrategy)) {
		schedulerStrategy = strings.ToLower(schedulerStrategy)
	} else {
		schedulerStrategy = SchedulerBinpack
	}
	return schedulerStrategy
}
