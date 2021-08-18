
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

