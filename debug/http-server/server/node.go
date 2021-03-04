package server

import (
	deviceManager "bocloud.com/cloudnative/carina/pkg/devicemanager"
	"github.com/labstack/echo/v4"
	"net/http"
	"strconv"
)

var dm *deviceManager.DeviceManager
var stopChan chan struct{}

func init() {
	stopChan = make(chan struct{})
	dm = deviceManager.NewDeviceManager("localhost", stopChan)
}

func Start(c echo.Context) error {
	dm.DeviceCheckTask()
	dm.LvmHealthCheck()
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

func CreateSnapshot(c echo.Context) error {
	lvName := c.FormValue("lv_name")
	vgName := c.FormValue("vg_name")
	snapName := c.FormValue("snap_name")
	err := dm.VolumeManager.CreateSnapshot(snapName, lvName, vgName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.JSON(http.StatusOK, "")
}

func DeleteSnapshot(c echo.Context) error {
	vgName := c.FormValue("vg_name")
	snapName := c.FormValue("snap_name")
	err := dm.VolumeManager.DeleteSnapshot(snapName, vgName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.JSON(http.StatusOK, "")
}

func RestoreSnapshot(c echo.Context) error {
	vgName := c.FormValue("vg_name")
	snapName := c.FormValue("snap_name")
	err := dm.VolumeManager.RestoreSnapshot(snapName, vgName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.JSON(http.StatusOK, "")
}

func CloneVolume(c echo.Context) error {
	lvName := c.FormValue("lv_name")
	vgName := c.FormValue("vg_name")
	newLvName := c.FormValue("new_lv_name")
	err := dm.VolumeManager.CloneVolume(lvName, vgName, newLvName)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}
	return c.JSON(http.StatusOK, "")
}
