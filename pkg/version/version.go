package version

import (
	"fmt"
	"strings"

	"github.com/carina-io/carina/pkg/configuration"
	"github.com/carina-io/carina/utils"
)

// 处理磁盘类型参数，支持carina.storage.io/disk-type:ssd书写方式
func GetdeviceGroup(diskType string) string {
	deviceGroup := strings.ToLower(diskType)
	currentDiskSelector := configuration.DiskSelector()
	diskClass := configuration.NewDiskClass(currentDiskSelector)
	//如果sc 配置的磁盘组在配置里就默认返回配置的磁盘组，老板本的磁盘组如果在新配置文件里配置了，就采用新的配置
	if _, ok := diskClass.DiskClassByName[diskType]; ok {
		return deviceGroup
	}
	//这里是为了兼容旧版本的sc
	if utils.ContainsString([]string{"ssd", "hdd"}, deviceGroup) {
		deviceGroup = fmt.Sprintf("carina-vg-%s", deviceGroup)
	}
	return deviceGroup

}
