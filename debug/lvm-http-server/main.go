package main

import (
	server2 "bocloud.com/cloudnative/carina/debug/lvm-http-server/server"
	"github.com/labstack/echo/v4"
)

func main() {

	e := echo.New()
	// node
	e.PUT("/server/start", server2.Start)
	e.PUT("/server/stop", server2.Stop)
	e.POST("/volume/create", server2.CreateVolume)
	e.PUT("/volume/resize", server2.ResizeVolume)
	e.DELETE("/volume/delete", server2.DeleteVolume)
	e.GET("/volume/list", server2.GetVolume)
	e.GET("/volume/group", server2.GetVolumeGroup)
	e.POST("/volume/snapshot/create", server2.CreateSnapshot)
	e.DELETE("/volume/snapshot/delete", server2.DeleteSnapshot)
	e.POST("/volume/snapshot/restore", server2.RestoreSnapshot)
	e.POST("/volume/clone", server2.CloneVolume)

	e.Logger.Fatal(e.Start(":8080"))
}
