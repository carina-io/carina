package server

import (
	"carina/pkg/csidriver/csi"
	"carina/utils"
	"carina/utils/log"
	"context"
	"github.com/golang/protobuf/ptypes/wrappers"
)

// NewIdentityService returns a new IdentityServer.

func NewIdentityServer() csi.IdentityServer {
	return &IdentityServer{}
}

type IdentityServer struct {
	csi.UnimplementedIdentityServer
}

func (s IdentityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	log.Info("GetPluginInfo req ", req.String())
	return &csi.GetPluginInfoResponse{
		Name:          utils.PluginName,
		VendorVersion: utils.Version,
	}, nil
}

func (s IdentityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	log.Info("GetPluginCapabilities req ", req.String())
	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
			{
				Type: &csi.PluginCapability_VolumeExpansion_{
					VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
						Type: csi.PluginCapability_VolumeExpansion_ONLINE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_VolumeExpansion_{
					VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
						Type: csi.PluginCapability_VolumeExpansion_OFFLINE,
					},
				},
			},
		},
	}, nil
}

func (s IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	log.Info("Probe req ", req.String())
	return &csi.ProbeResponse{Ready: &wrappers.BoolValue{Value: true}}, nil
}
