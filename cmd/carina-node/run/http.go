package run

import (
	"bocloud.com/cloudnative/carina/pkg/devicemanager/volume"
	"github.com/labstack/echo/v4"
	"net/http"
)

var (
	volumeManager volume.LocalVolume
)

type eHttpServer struct {
	e        *echo.Echo
	stopChan <-chan struct{}
}

func newHttpServer(v volume.LocalVolume, stopChan <-chan struct{}) *eHttpServer {
	volumeManager = v
	e := echo.New()
	e.GET("/devicegroup", vgList)
	e.GET("/volume", volumeList)

	return &eHttpServer{
		e:        e,
		stopChan: stopChan,
	}
}

func (h *eHttpServer) start() {
	for {
		select {
		case <-h.stopChan:
			volumeManager = nil
			_ = h.e.Close()
		default:

		}
		h.e.Logger.Fatal(h.e.Start(config.httpAddr))
	}
}

func vgList(c echo.Context) error {
	vgList, err := volumeManager.GetCurrentVgStruct()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, vgList)
}

func volumeList(c echo.Context) error {
	lvList, err := volumeManager.VolumeList("", "")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, lvList)
}
