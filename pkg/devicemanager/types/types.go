package types

// Represents a logical volume
type LogicalVolume struct {
	Name     string   `json:"name"`      // The logical volume name
	SizeGB   uint64   `json:"size_gb"`   // Volume size in GiB
	DevMajor uint32   `json:"dev_major"` // Device major number
	DevMinor uint32   `json:"dev_minor"` // Device minor number
	Tags     []string `json:"tags"`      // Tags to add to the volume during creation
}

// Represents the input for CreateLV
type CreateLVRequest struct {
	Name            string   `json:"name"`    // The logical volume name
	SizeGB          uint64   `json:"size_gb"` // volume size in GiB
	Tags            []string `json:"tags"`    // Tags to add to the volume during creation
	DeviceClassName string   `json:"device_class_name"`
}

// Represents the response of CreateLV
type CreateLVResponse struct {
	Volume *LogicalVolume `json:"volume"` // Information of the created volume
}

// Represents the input for RemoveLV
type RemoveLVRequest struct {
	Name            string `json:"name"` // The logical volume name
	DeviceClassName string `json:"device_class_name"`
}

// Represents the input for ResizeLV
//
// The volume must already exist
// The volume size will be set to exactly "size_gb"
type ResizeLVRequest struct {
	Name            string `json:"name"`    // The logical volume name
	SizeGB          uint64 `json:"size_gb"` // Volume size in GiB
	DeviceClassName string `json:"device_class_name"`
}

// Represents the input for GetLVList
type GetLVListRequest struct {
	DeviceClassName string `json:"device_class_name"`
}

// Represents the response of GetLVList
type GetLVListResponse struct {
	Volumes []*LogicalVolume `json:"volumes"` // Information of volumes
}

// Represents the input for GetFreeBytes
type GetFreeBytesRequest struct {
	DeviceClassName string `json:"device_class_name"`
}

// Represents the response of GetFreeBytes
type GetFreeBytesResponse struct {
	FreeBytes uint64 `json:"free_bytes"` // Free space of the volume group in bytes
}

// Represents the stream output from Watch
type WatchResponse struct {
	FreeBytes uint64       `json:"'free_bytes'"` // Free space of the default volume group in bytes
	Items     []*WatchItem `json:"items"`
}

type WatchItem struct {
	FreeBytes       uint64 `json:"free_bytes"` // Free space of the volume group in bytes
	DeviceClassName string `json:"device_class_name"`
}
