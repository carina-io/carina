package main

import (
	"carina/cmd/http-server/server"
	"github.com/labstack/echo/v4"
)

func main() {

	e := echo.New()
	// node
	e.PUT("/server/start", server.Start)
	e.PUT("/server/stop", server.Stop)
	e.POST("/volume/create", server.CreateVolume)
	e.PUT("/volume/resize", server.ResizeVolume)
	e.DELETE("/volume/delete", server.DeleteVolume)
	e.GET("/volume/list", server.GetVolume)
	e.GET("/volume/group", server.GetVolumeGroup)
	e.POST("/volume/snapshot/create", server.CreateSnapshot)
	e.DELETE("/volume/snapshot/delete", server.DeleteSnapshot)
	e.POST("/volume/snapshot/restore", server.RestoreSnapshot)
	e.POST("/volume/clone", server.CloneVolume)

	e.Logger.Fatal(e.Start(":8080"))
}
