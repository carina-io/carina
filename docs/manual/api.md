#### API

- Carina-node uses host network and listens port to `8080 8089`. The port number can be changed in yaml.  

  ```shell
          - "--metrics-addr=:8080"
          - "--http-addr=:8089"
  ```
  Note：if changing the ports, please ensure change the csi-carina-node service too.

- carina-controller listens to `8080 8443 8089`, thoes ports can be changed in yaml

  ```shell
          - "--metrics-addr=:8080"
          - "--webhook-addr=:8443"
          - "--http-addr=:8089"
  ```

carina-node and carina-controller both provides http interfaces

```shell
# get all LVM VG information：http://carina-controller:8089/devicegroup
# get all volume information：http://carina-controller:8089/volume
```

- Note：carina-node provides local nodes' vg and volume information.
- Note：carina-controller provides all nodes' vg and volume information.