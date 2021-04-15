package troubleshoot

import (
	carinav1 "bocloud.com/cloudnative/carina/api/v1"
	"bocloud.com/cloudnative/carina/pkg/devicemanager/volume"
	"bocloud.com/cloudnative/carina/utils/log"
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type Trouble struct {
	volumeManager volume.LocalVolume
	cache         cache.Cache
	nodeName      string
}

func NewTroubleObject(volumeManager volume.LocalVolume, cache cache.Cache, nodeName string) *Trouble {

	err := cache.IndexField(context.Background(), &carinav1.LogicVolume{}, "nodeName", func(object client.Object) []string {
		return []string{object.(*carinav1.LogicVolume).Spec.NodeName}
	})

	if err != nil {
		log.Errorf("index node with logicVolume error %s", err.Error())
	}

	return &Trouble{
		volumeManager: volumeManager,
		cache:         cache,
		nodeName:      nodeName,
	}
}

func (t *Trouble) CleanupOrphanVolume() {

	// step.1 获取所有本地volume
	log.Infof("step.1 get all local volume")
	volumeList, err := t.volumeManager.VolumeList("", "")
	if err != nil {
		log.Errorf("step.1 get all local volume failed %s", err.Error())
	}

	// step.2 检查卷状态是否正常
	log.Infof("step.2 check volume status")
	for _, lv := range volumeList {
		if lv.LVActive != "active" {
			log.Warnf("lv %s current status %s", lv.LVName, lv.LVActive)
		}
	}

	// step.3 获取集群中logicVolume对象
	log.Infof("step.3 get all cluster logicVolume")
	lvList := &carinav1.LogicVolumeList{}
	err = t.cache.List(context.Background(), lvList, client.MatchingFields{"nodeName": t.nodeName})
	if err != nil {
		log.Errorf("list logic volume error %s", err.Error())
		return
	}

	// step.4 对比本地volume与logicVolume是否一致， 远程没有的便删除本地的
	log.Infof("step.4 cleanup orphan volume")
	mapLvList := map[string]bool{}
	for _, v := range lvList.Items {
		mapLvList[v.Name] = true
		mapLvList[fmt.Sprintf("thin-%s", v.Name)] = true
		mapLvList[fmt.Sprintf("volume-%s", v.Name)] = true
	}

	for _, v := range volumeList {
		if strings.Contains(v.VGName, "carina") {
			log.Infof("skip volume %s", v.LVName)
			continue
		}
		if _, ok := mapLvList[v.LVName]; !ok {
			log.Warnf("remove volume %s %s", v.VGName, v.LVName)
			if strings.HasPrefix(v.LVName, "volume-") {
				err := t.volumeManager.DeleteVolume(v.LVName, v.VGName)
				if err != nil {
					log.Errorf("delete volume vg %s lv %s error %s", v.VGName, v.LVName, err.Error())
				}
			}
		}
	}

	log.Info("volume check finished.")
}
