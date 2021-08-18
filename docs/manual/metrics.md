指标监控

- carina-node 为host网络模式部署并监听 `8080 8089`端口，其中8080为metrics、8089为http，可通过如下配置进行修改

  ```shell
          - "--metrics-addr=:8080"
          - "--http-addr=:8089"
  ```

  备注：若是修改监听端口，务必同步修改`service：csi-carina-node`

- carina-controller 监听`8080 8443 8089`，其中8080为metrics、8443为webhook、8089为http，可通过如下配置进行修改

  ```shell
          - "--metrics-addr=:8080"
          - "--webhook-addr=:8443"
          - "--http-addr=:8089"
  ```

- carina-node和carina-controller，自定义指标

  ```shell
  	# vg剩余容量:  carina-devicegroup-vg_free_bytes
  	# vg总容量:  carina-devicegroup-vg_total_bytes
  	# volume容量:  carina-volume-volume_total_bytes
  	# volume使用量:  carina-volume-volume_used_bytes
  ```

  - 备注1：volume使用量lvm统计与`df -h`统计不同，误差在几十兆
  - 备注2：carina-controller实际是收集的所有carina-node的数据，实际只要通过carina-controller获取监控指标便可
  - 备注3：如果要使用prometheus收集监控指标，可部署servicemonitor(deployment/kubernetes/prometheus.yaml.tmpl)

- 虽然carina提供了卷存储指标，但是也可以使用kubelet暴露的pvc存储指标，在grafana kubernetes内置视图中可以看到此模板，这个内置模板只有在pvc被挂载到节点并被POD使用时才会看到指标

