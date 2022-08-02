
#### 故障转移

对于本地存储来说，一旦pv调度到某个节点其存储卷便创建到了某个节点，这也意味着POD也只能调度到那个节点，当节点故障时，这些POD将无法成功调度成功，因此该功能的目的主要是为了清理已经被删除节点，并在其他节点重建PVC

查看一下当前 lv

```shell
# 示例
$ kubectl get lv
NAME                                       SIZE   GROUP           NODE          STATUS
pvc-177854eb-f811-4612-92c5-b8bb98126b94   5Gi    carina-vg-hdd   10.20.9.154   Success
pvc-1fed3234-ff89-4c58-8c65-e21ca338b099   5Gi    carina-vg-hdd   10.20.9.153   Success
pvc-527b5989-3ac3-4d7a-a64d-24e0f665788b   10Gi   carina-vg-hdd   10.20.9.154   Success
pvc-b987d27b-39f3-4e74-9465-91b3e6b13837   3Gi    carina-vg-hdd   10.20.9.154   Success

$ kubectl delete node 10.20.9.154
# volume进行重建，重建会丢失原先的volume数据
$ kubectl get lv
NAME                                       SIZE   GROUP           NODE          STATUS
pvc-177854eb-f811-4612-92c5-b8bb98126b94   5Gi    carina-vg-hdd   10.20.9.153   Success
pvc-1fed3234-ff89-4c58-8c65-e21ca338b099   5Gi    carina-vg-hdd   10.20.9.153   Success
pvc-527b5989-3ac3-4d7a-a64d-24e0f665788b   10Gi   carina-vg-hdd   10.20.9.153   Success
pvc-b987d27b-39f3-4e74-9465-91b3e6b13837   3Gi    carina-vg-hdd   10.20.9.153   Success
```

lv与本地存储卷一一对应，当lv卷与本地存储卷不一致时会清理本地lvm卷

- 每十分钟会遍历本地volume，然后检查k8s中是否有对应的logicvolume，若是没有则删除本地volume

- 每十分钟会遍历k8s中logicvolume，然后检查logicvolume是否有对应的pv，若是没有则删除logicvolume

- 当节点被删除时，在这个节点的上的所有volume将在其他节点重建

#### 节点NotReady，容器迁移

- Carina会检测节点状态，当节点进入NotReady状态时，将会触发容器迁移策略
- 该迁移策略会检查容器是否存在注解`carina.storage.io/allow-pod-migration-if-node-notready`并且其值为"true"则表示该容器希望carina对其进行迁移
- 众所周知作为本地存储，carina所创建的存储卷全都存在于本地磁盘，如果发生容器迁移则必然的容器所使用的PVC会在其他节点重建，数据是无法跟随；
- 所以如果想迁移POD依赖于应用本身的数据高可用功能，比如Mysql迁移的话节点重建后会通过binlog日志同步数据

