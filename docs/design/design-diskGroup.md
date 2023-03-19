## Background

Currently, as of v0.9.0, Carina separates node's local disks into different
groups based on its type.  User can request different storage disks using
differents storageclasses. This works fine in general, but in some cases,
user may prefer more flexiable usage. For example,

1#, Though all the disks are with the same type, workloads may prefer
exclusively using disk groups, rather than racing with other workloads in the
same pool, for better isolation.

2#, For really high performance disk types, like NVMe or Pmem, LVM and RAID
are both not needed. Carina should provide raw disk usage.

## Design

For now, user can configure disk groups via `diskSelector` and `diskGroupPolicy`
in configmap. We should extend the current instruction and may add new one
to get more flexibility.

For reference, as of v0.9.0, carina configmap has below structure.

```
data:
  config.json: |-
    {
      "diskSelector": ["loop*", "vd*"],
      "diskScanInterval": "300",
      "diskGroupPolicy": "type",
      "schedulerStrategy": "spreadout"
    }
```

### Related instructions' semantics

```yaml
diskSelectors:
  - name: group1
    re:
      - sd[b-f]
      - sd[m-o]
    policy: LVM
    nodeLabel: node-label
  - name: group2
    re:
      - sd[h-g]
    policy: RAW
    nodeLabel: node-label
```

#### diskSelector

The `diskSelector` is a list of diskGroups.  Each diskGroup has three
parameters, which are all required, not optional.

* name

  User can assign a name to each diskGroup and then create one storageclass
  with a matching name for daily usage.

  Each diskGroup should have unique naming.

* re

  re is a list of reguare expression strings. Each element selects some of
  the local disks. All the disks selected by all re strings are grouped in
  this diskGroup.

  If a disk is selected by multiple re stings in one diskGroup, the disk will
  finially appears in this diskGroup. However, if one disk is selected by
  multiple re strings from multiple diskGroups, this disk is ignored by those
  diskGroups.

  Different types of disks can be grouped into one diskGroup by re strings. It's
  not RECOMMENDED, but carina allows user to do that.

* policy

  Policy specifies the way how to manage those disks in one diskGroup. Currently
  user can specify two policies, LVM or RAW.

  For LVM policy, it works as always. Disks from one diskGroup are treated as
  LVM-PVs and then grouped into one LVM-VG. When user requests PV from this
  diskGroup, carina allocates one LVM-LV as its real data backend.

  Raw policy is a new policy that user can comsume disks. Those disks may have
  different size.  Assume user is requesting a PV with size of S, the procedure
  that carina picks up one disk works below:

  * find out all unempty disks(already have partitions) and choose the disk
  with the minimum requirement that its free space is larger than S.
  * If all unempty disks are not suitable, then choose the disk with the minimum
  requirement that its capacity is larger than S.
  * If multile disks with same size is selected, then randomly choose one.

  If a disk is selected from above procedure, then carina create a partition as
  the PV's really data backend. Else, the PV binding will failed.

  User can specify an annotation in the PVC to claim a physical disk with
  exclusive usage. If this annotation is been set and its value is true,
  then carina will try to bind one empty disk(with the minimum requirement)
  as its data backend.

  ```
  carina.storage.io/allow-pod-migration-if-node-notready: true
  ```
* nodeLabel

  The configuration takes effect on all nodes. If the configuration is empty, 
   the configuration takes effect on all nodes

#### diskGroupPolicy

Deprecated. Carina will not automatically group disks by their type.

### Side effects on other components

#### carina-scheduler

For raw disk usage, to avoid the total free disk space is enough for one PV,
but can't hold it in single disk device, carina need to maintain the largest
single PV size it can allocate for each node.

#### Node's capacity and allocatble

For LVM diskGroups, carina can still works as before.

For raw diskGroups, carina should add extra informantion. For example,
largest PV size each node can hold, total empty disks each node has, and
so on.

#### A good default setting

Although carina will not use any disks or partitions with any filesystem on it,
eliminating misusage as much as possible, there is still possibility that some
workloads are using raw devices directly. Carina now doesn't have a good default
setting. User should group disks explicitly.

As a typical environment, carina will use below setting as its default configmap.
User should double check before put it in production environment.

```yaml
diskSelectors:
  - name: defaultGroup
    re:
      - sd*
    policy: LVM
```

### Migrating from v0.9.0 or below

When carina starts, Old volumes can still be identified

#### The old configmap setting

```
data:
  config.json: |-
    {
      "diskSelector": ["loop*", "vd*"],
      "diskScanInterval": "300",
      "diskGroupPolicy": "type",
      "schedulerStrategy": "spreadout"
    }
```

#### The new configmap setting

```
data:
  config.json: |-
    {
      "diskSelector": [
        {
          "name": hdd ## if there are HDDs
          "re": ["loop*", "vd*"]
          "policy": "LVM"
          "nodeLabel": "node-label"
        },
        {
          "name": ssd
          "re": ["loop*", "vd*"]
          "policy": "LVM"
          "nodeLabel": "node-label"
        }
      ],
      "diskScanInterval": "300",
      "schedulerStrategy": "spreadout"    
    }

```
