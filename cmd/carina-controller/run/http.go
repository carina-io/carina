/*
   Copyright @ 2021 fushaosong <fushaosong@beyondlet.com>.

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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bocloud/carina/pkg/devicemanager/types"
	"github.com/bocloud/carina/utils/log"
	"github.com/labstack/echo/v4"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type carinaNode struct {
	Ip       string
	NodeName string
	Port     int32
}

var (
	kCache cache.Cache
)

type eHttpServer struct {
	e        *echo.Echo
	stopChan <-chan struct{}
}

func newHttpServer(c cache.Cache, stopChan <-chan struct{}) *eHttpServer {
	kCache = c
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
			kCache = nil
			_ = h.e.Close()
		default:

		}
		h.e.Logger.Fatal(h.e.Start(config.httpAddr))
	}
}

func vgList(c echo.Context) error {
	endpoints, err := getEndpoints()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	result := map[string][]types.VgGroup{}
	for _, ep := range endpoints {
		resp, err := http.Get(fmt.Sprintf("http://%s:%d/devicegroup", ep.Ip, ep.Port))
		if err != nil {
			log.Infof("error %s", err.Error())
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		r := []types.VgGroup{}
		err = json.Unmarshal(body, &r)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, err.Error())
		}
		result[ep.NodeName] = r
	}
	return c.JSON(http.StatusOK, result)
}

func volumeList(c echo.Context) error {
	endpoints, err := getEndpoints()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	result := map[string][]types.LvInfo{}
	for _, ep := range endpoints {
		resp, err := http.Get(fmt.Sprintf("http://%s:%d/volume", ep.Ip, ep.Port))
		if err != nil {
			log.Infof("error %s", err.Error())
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			continue
		}
		r := []types.LvInfo{}
		err = json.Unmarshal(body, &r)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, err.Error())
		}
		result[ep.NodeName] = r
	}

	return c.JSON(http.StatusOK, result)
}

func getEndpoints() ([]carinaNode, error) {
	result := []carinaNode{}
	endpoints := corev1.Endpoints{}
	err := kCache.Get(context.Background(), client.ObjectKey{"kube-system", "carina-node"}, &endpoints)
	if err != nil {
		return result, err
	}
	if len(endpoints.Subsets) == 0 {
		return result, errors.New("no endpoints")
	}
	es := endpoints.Subsets[0]

	port := int32(0)
	for _, p := range es.Ports {
		if p.Name == "http" {
			port = p.Port
		}
	}
	for _, a := range es.Addresses {
		result = append(result, carinaNode{
			Ip:       a.IP,
			NodeName: *a.NodeName,
			Port:     port,
		})
	}
	return result, nil
}
