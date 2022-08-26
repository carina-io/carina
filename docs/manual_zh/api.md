#### API接口

- carina-node 为host网络模式部署并监听 `8080`端口，其中8080为metrics，可通过如下配置进行修改

  ```shell
          - "--metrics-addr=:8080"
  ```

- carina-controller 监听`8080 8443`，其中8080为metrics、8443为webhook，可通过如下配置进行修改

  ```shell
          - "--metrics-addr=:8080"
          - "--webhook-addr=:8443"
  ```