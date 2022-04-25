#### API接口

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

carina-node和carina-controller均暴露了http接口，共提供两个方法

```shell
# 获取所有vg信息：http://carina-controller:8089/devicegroup
# 获取所有volume信息：http://carina-controller:8089/volume
```

- 备注1：carina-node获取的是当前节点的所有vg及volume信息
- 备注2：carina-controller接口是收集所有carina-node的vg及volume的汇总信息
- 备注3：carina-controller服务的svc名称为carina-controller