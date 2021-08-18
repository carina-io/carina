

#### 本次磁盘管理方案

#### 前言

- 服务器挂载磁盘后，需要对磁盘进行分区格式化才能正常使用，为了更简便的使用存储服务，该项目的节点服务启动后会自动对磁盘进行管理，将SSD磁盘和HDD磁盘分组并创建`pv vg`卷组
- 目前只支持管理SSD磁盘和HDD磁盘

#### 功能设计

- 定时扫描本地磁盘，将符合条件的裸盘加入vg卷组
- 定时扫描本地磁盘，将不符合条件的磁盘从vg卷组中移除
- 通过配置文件获取扫描时间间隔和磁盘的匹配条件
- 本地lvm卷管理

#### 实现细节

- 配置文件，支持动态更新

  ```json
  config.json: |-
      {
      "diskSelector": ["loop*", "vd*"], # 磁盘匹配策略，支持正则表达式
        "diskScanInterval": "300", # 300s 磁盘扫描间隔，0表示关闭本地磁盘扫描
        "diskGroupPolicy": "type", # 磁盘分组策略，只支持按照磁盘类型分组，更改成其他值无效
        "schedulerStrategy": "spradout" # binpack，spradout支持这两个参数
      }
  ```
  
- 磁盘结构

  ```go
  type LocalDisk struct {
  	// Name is the device name
  	Name string `json:"name"`
  	// mount point
  	MountPoint string `json:"mountPoint"`
  	// Size is the device capacity in byte
  	Size uint64 `json:"size"`
  	// status
  	State string `json:"state"`
  	// Type is disk type
  	Type string `json:"type"`
  	// 1 for hdd, 0 for ssd and nvme
  	Rotational string `json:"rotational"`
  	// ReadOnly is the boolean whether the device is readonly
  	Readonly bool `json:"readOnly"`
  	// Filesystem is the filesystem currently on the device
  	Filesystem string `json:"filesystem"`
  	// has used
  	Used uint64 `json:"used"`
  	// parent Name
  	ParentName string `json:"parentName"`
  }
  ```

- volume卷组

  ```go
  type LvInfo struct {
  	LVName        string `json:"lvName"`
  	VGName        string `json:"vgName"`
  	LVPath        string `json:"lvPath"`
  	LVSize        uint64 `json:"lvSize"`
  	LVKernelMajor uint32 `json:"lvKernelMajor"`
  	LVKernelMinor uint32 `json:"lvKernelMinor"`
  	Origin        string `json:"origin"`
  	OriginSize    uint64 `json:"originSize"`
  	PoolLV        string `json:"poolLv"`
  	ThinCount     uint64 `json:"thinCount"`
  	LVTags        string `json:"lvTags"`
  	DataPercent   string `json:"dataPercent"`
  	LVAttr        string `json:"lvAttr"`
  	LVActive      string `json:"lvActive"`
  }
  ```

- 创建volume，从VG卷组分配容量创建`thin pool`，然后在`thin pool`中创建lvm卷

  ```text
  +-----------------+                        +--------------------+                   
  |     VG          |                        |    thin pool       |
  | +-----+ +-----+ |----------------------> |  +-------------+   | 
  | | ssd | | ssd | | distribution capacity  |  | lvm volume  |   |
  | +-----+ +-----+ |----------------------> |  |             |   |
  |                 |                        |  +-------------+   | 
  +-----------------+                        +--------------------+
  ```

- 磁盘实际volume示意图

  ```
  vdc                                                                           
  |-carina-thin--pvc--xxx_tmeta   252:0    0    8M  0 lvm  
  | `-carina-thin--pvc--xxx-tpool 252:2    0    7G  0 lvm  
  |   |-carina-thin--pvc--xxx     252:3    0    7G  1 lvm  
  |   `-carina-volume--pvc--xxx   252:4    0    7G  0 lvm  /var/lib/kubelet/pods/xxx/volumes/kubernetes.io~csi/pvc-xxx/mount
  ```

  

