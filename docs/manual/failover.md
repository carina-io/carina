
#### failover

For most of the local storage projects, once pod has been scheduled to node and the pod will use the local pv from that node. That means the pod has been nailed to that node and if node fails, the pod will not migrate to other nodes.

With carina, user can allow the pod to migrate in such case with one annotation.

Using LVM backend engine, checking current LV first.

```shell
$ kubectl get lv
NAME                                       SIZE   GROUP           NODE          STATUS
pvc-177854eb-f811-4612-92c5-b8bb98126b94   5Gi    carina-vg-hdd   10.20.9.154   Success
pvc-1fed3234-ff89-4c58-8c65-e21ca338b099   5Gi    carina-vg-hdd   10.20.9.153   Success
pvc-527b5989-3ac3-4d7a-a64d-24e0f665788b   10Gi   carina-vg-hdd   10.20.9.154   Success
pvc-b987d27b-39f3-4e74-9465-91b3e6b13837   3Gi    carina-vg-hdd   10.20.9.154   Success

$ kubectl delete node 10.20.9.154
$ kubectl get lv
NAME                                       SIZE   GROUP           NODE          STATUS
pvc-177854eb-f811-4612-92c5-b8bb98126b94   5Gi    carina-vg-hdd   10.20.9.153   Success
pvc-1fed3234-ff89-4c58-8c65-e21ca338b099   5Gi    carina-vg-hdd   10.20.9.153   Success
pvc-527b5989-3ac3-4d7a-a64d-24e0f665788b   10Gi   carina-vg-hdd   10.20.9.153   Success
pvc-b987d27b-39f3-4e74-9465-91b3e6b13837   3Gi    carina-vg-hdd   10.20.9.153   Success
```

LV has one:one mapping with local volume.

* Carina will delete local volume if it does't have an associated LV every 600s. 
* Carina will delete LV if it doesn't have an associated PV every 600s.
* If node been deleted, all volumes will be rebuild on other nodes. 

#### Allowing pod to migrate in case of node failure

* Carina will track each node's status. If node enters NotReady state, carina will trigger pod migration policy.
* Carina will allow pod to migrate if it has annotation `carina.storage.io/allow-pod-migration-if-node-notready` with value of `true`.
* Carina will not copy data from failed node to other node. So the newly borned pod will have an empty PV.
* The middleware layer should trigger data migration. For example, master-slave mysql cluster should trigger master-slave replication.