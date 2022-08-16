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

package server

import (
	"net/http"
	"strconv"

	deviceManager "github.com/carina-io/carina/pkg/devicemanager"
	"github.com/labstack/echo/v4"
)

var dm *deviceManager.DeviceManager
var stopChan chan struct{}

func init() {
	stopChan = make(chan struct{})

	dm = deviceManager.NewDeviceManager("localhost", nil)
}

func Start(c echo.Context) error {
	dm.GetNodeDiskSelectGroup()
	return c.JSON(http.StatusOK, "")
}

func Stop(c echo.Context) error {
	close(stopChan)
	return c.JSON(http.StatusOK, "")
}

func CreateVolume(c echo.Context) error {
	lvName := c.FormValue("lv_name")
	vgName := c.FormValue("vg_name")
	size := c.FormValue("size")
	req, _ := strconv.ParseUint(size, 10, 64)
	err := dm.VolumeManager.CreateVolume(lvName, vgName, req, 1)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, "")
}

func ResizeVolume(c echo.Context) error {
	lvName := c.FormValue("lv_name")
	vgName := c.FormValue("vg_name")
	size := c.FormValue("size")
	req, _ := strconv.ParseUint(size, 10, 64)
	err := dm.VolumeManager.ResizeVolume(lvName, vgName, req, 1)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, "")
}

func DeleteVolume(c echo.Context) error {
	lvName := c.FormValue("lv_name")
	vgName := c.FormValue("vg_name")
	err := dm.VolumeManager.DeleteVolume(lvName, vgName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, "")
}

func GetVolume(c echo.Context) error {
	lvName := c.QueryParam("lv_name")
	vgName := c.QueryParam("vg_name")
	lvInfo, err := dm.VolumeManager.VolumeList(lvName, vgName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, lvInfo)
}

func GetVolumeGroup(c echo.Context) error {

	info, err := dm.VolumeManager.GetCurrentVgStruct()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.JSON(http.StatusOK, info)
}

func CreateBcache(c echo.Context) error {
	dev := c.FormValue("dev")
	cacheDev := c.FormValue("cache_dev")
	block := c.FormValue("block")
	bucket := c.FormValue("bucket")
	mode := c.FormValue("mode")
	devicePath, err := dm.VolumeManager.CreateBcache(dev, cacheDev, block, bucket, mode)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.JSON(http.StatusOK, devicePath)
}

func DeleteBcache(c echo.Context) error {
	dev := c.FormValue("dev")
	cacheDev := c.FormValue("cache_dev")
	err := dm.VolumeManager.DeleteBcache(dev, cacheDev)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.JSON(http.StatusOK, "")
}

func GetBcache(c echo.Context) error {
	dev := c.FormValue("dev")
	info, err := dm.VolumeManager.BcacheDeviceInfo(dev)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.JSON(http.StatusOK, info)
}
