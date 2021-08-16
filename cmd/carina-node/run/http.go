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
package run

import (
	"github.com/bocloud/carina/pkg/devicemanager/volume"
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
