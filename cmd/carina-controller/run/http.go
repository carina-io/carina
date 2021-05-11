package run

import (
	"bocloud.com/cloudnative/carina/pkg/configuration"
	"bocloud.com/cloudnative/carina/utils/log"
	"context"
	"errors"
	"fmt"
	"github.com/labstack/echo/v4"
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
	c cache.Cache
)

type eHttpServer struct {
	e        *echo.Echo
	stopChan <-chan struct{}
}

func newHttpServer(c cache.Cache, stopChan <-chan struct{}) *eHttpServer {
	c = c
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
			c = nil
			_ = h.e.Close()
		default:

		}
		h.e.Logger.Fatal(h.e.Start(":8089"))
	}
}

func vgList(c echo.Context) error {
	endpoints, err := getEndpoints()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	for _, ep := range endpoints {
		resp, err := http.Get(fmt.Sprintf("http://%s:%d/devicegroup", ep.Ip, ep.Port))
		if err != nil {
			log.Infof("error %s", err.Error())
		}
		if resp != nil && resp.StatusCode != http.StatusOK {
			continue
		}
		return c.JSON(http.StatusOK, resp)

	}
	return c.JSON(http.StatusOK, "ok")
}

func volumeList(c echo.Context) error {
	endpoints, err := getEndpoints()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err.Error())
	}
	for _, ep := range endpoints {
		resp, err := http.Get(fmt.Sprintf("http://%s:%d/volume", ep.Ip, ep.Port))
		if err != nil {
			log.Infof("error %s", err.Error())
		}
		if resp != nil && resp.StatusCode != http.StatusOK {
			continue
		}
		return c.JSON(http.StatusOK, resp)
	}

	return c.JSON(http.StatusOK, "ok")
}

func getEndpoints() ([]carinaNode, error) {
	result := []carinaNode{}
	endpoints := corev1.Endpoints{}
	err := c.Get(context.Background(), client.ObjectKey{configuration.RuntimeNamespace(), "carina-node"}, &endpoints)
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
