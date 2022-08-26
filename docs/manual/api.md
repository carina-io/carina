#### API

- Carina-node uses host network and listens port to `8080`. The port number can be changed in yaml.  

  ```shell
          - "--metrics-addr=:8080"
  ```

- carina-controller listens to `8080 8443`, those ports can be changed in yaml

  ```shell
          - "--metrics-addr=:8080"
          - "--webhook-addr=:8443"
  ```