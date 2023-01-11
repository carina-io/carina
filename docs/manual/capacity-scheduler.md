#### capacity scheduling

carina-scheduler can scheduling pods based on usage and capacity of nodes' disks.

```yaml
 config.json: |-
    {
      "diskScanInterval": "300", # disk scan intervals in seconds. Zero will disable scanning. 
      "diskGroupPolicy": "type", # disk group policy
      "schedulerStrategy": "spreadout" # scheduler policy, supports binpack and spreadout.
    }
```

For schedulerStrategy,

- In case of `storageclass volumeBindingMode:Immediate`, the scheduler will only consider nodes' disk usage. For example, with `spreadout` policy, carina scheduler will pick the node with the largest disk capacity to create volume.
- In case of `schedulerStrategy`在`storageclass volumeBindingMode:WaitForFirstConsumer`, carina scheduler only affects the pod scheduleing by providing its rank. Kube-scheduler will pick a node finally. User can learn detailed messages in carina-scheduler's log.
- When multiples nodes have valid capacity ten times larger than requested, those node will share the same rank. 

Note：there is an carina webhook that will change the pod scheduler to carina-scheduler if it uses carina PVC. 