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

package main

import (
	server2 "github.com/carina-io/carina/debug/lvm-http-server/server"
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

	e.POST("/volume/bcache/create", server2.CreateBcache)
	e.DELETE("/volume/bcache/delete", server2.DeleteBcache)
	e.GET("/volume/bcache", server2.GetBcache)

	e.Logger.Fatal(e.Start(":8080"))
}
