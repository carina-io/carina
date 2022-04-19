#### FAQ

- What are the main conponents of Carina and their resposibility? 
  
  - Carina has three main components: carina-scheduler、carina-controller、carina-node.
    User can get detailed runtime information by checking each components' logs.

  - carina-scheduler：all pods using PVC backed by Carina will be scheduled by carina-scheduler.
  - carina-controller：Watching the events of PVC and creates LogicVolume internally.
  - carina-node：Managing local disks and watching events of LogicVolume and create local LVM or raw volumes. 
  


- Known issue, PV creation may fail if the local disks' performance is really poor. 

  - Carina will try to create LVM volume every ten seconds. The creation will be failed if retries 10 times. User can learn more details by using `kubectl get lv`. 

- Once the PV has been created successfully, can the Pod migrate to other nodes. 

  - For typical local volume solutions, if node failes, the pod using local disks can't migrate to other nodes. But Carina can detect the node status and let pod migrate. The newly-borned Pod will have an empty carina volume however. 

- How to run a pod using an specified PV on one of the nodes? 

  - using `spec.nodeName` to bypass the scheduler.
  - For StorageClass with `WaitForFirstConsumer`, user can add one annotation `volume.kubernetes.io/selected-node: ${nodeName}` to PVC and then the pod will be scheduled to specified node. 
  - This is not recommanded unless knowning the machanisums very clearly. 

- How to deal with the PVs if it's node been deleted from K8S cluster?

  - Just delete the PVC and rebuild. 

- How to create local disks for testing usage? 

  - user can create loop device if there are not enough physical disks. 

  ```shell
  for i in $(seq 1 5); do
    truncate --size=200G /tmp/disk$i.device && \
    losetup -f /tmp/disk$i.device
  done
  ```

- How to simulate local SSD disks? 

  ```shell
  $ echo 0 > /sys/block/loop0/queue/rotational
  $ lsblk -d -o name,rota
   NAME ROTA
   loop1     1
   loop0     0
  ```

- About bcache of each node. 
  - bcache is an kernel module. Some the Linux distributions may not enable bcache, you can disable carina's bcache suppport by below methods. 

  ```shell
  # install bcache
  $ modprobe bcache
  $ lsmod | grep bcache
  bcache                233472  0
  crc64                  16384  1 bcache
  # When there is no bache module, you need to delete the bcache segment from deploy/kuernetes/csi-carina-node.yaml 
  # delete loading bcache module in init-container. 
  # delete bcache diectory in csi-carina-node.yaml 
  - name: host-bcache
    mountPath: /sys/fs/bcache
    
  - name: host-bcache
    hostPath:
      path: /sys/fs/bcache            
  ```

- Enjoy Carina!