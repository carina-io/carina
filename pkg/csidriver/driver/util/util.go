package util

import (
	"fmt"
	"github.com/carina-io/carina/pkg/configuration"
	"github.com/carina-io/carina/utils"
	"strings"
)

// 处理磁盘类型参数，支持carina.storage.io/disk-group-name:ssd书写方式
func GetDeviceGroup(diskType string) string {
	deviceGroup := strings.ToLower(diskType)
	currentDiskSelector := configuration.DiskSelector()
	var diskClass = []string{}
	for _, v := range currentDiskSelector {
		if strings.ToLower(v.Policy) == "raw" {
			continue
		}
		diskClass = append(diskClass, strings.ToLower(v.Name))
	}
	//diskClass := configuration.GetDiskGroups()
	//如果sc 配置的磁盘组在配置里就默认返回配置的磁盘组，老板本的磁盘组如果在新配置文件里配置了，就采用新的配置
	if utils.ContainsString(diskClass, deviceGroup) {
		return deviceGroup
	}
	//这里是为了兼容旧版本的sc
	if utils.ContainsString([]string{"ssd", "hdd"}, deviceGroup) {
		deviceGroup = fmt.Sprintf("carina-vg-%s", deviceGroup)
	}
	return deviceGroup

}

func CheckRawDeviceGroup(diskType string) bool {
	deviceGroup := strings.ToLower(diskType)
	currentDiskSelector := configuration.DiskSelector()
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
