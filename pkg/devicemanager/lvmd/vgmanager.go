package lvmd

import (
	"bytes"
	"carina/utils/log"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Control verbose output of all LVM CLI commands
var Verbose bool

// isInsufficientSpace returns true if the error is due to insufficient space
func isInsufficientSpace(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "insufficient free space")
}

// isInsufficientDevices returns true if the error is due to insufficient underlying devices
func isInsufficientDevices(err error) bool {
	return strings.Contains(err.Error(), "Insufficient suitable allocatable extents for logical volume")
}

// MaxSize states that all available space should be used by the
// create operation.
const MaxSize uint64 = 0

type simpleError string

func (s simpleError) Error() string { return string(s) }

const ErrNoSpace = simpleError("lvm: not enough free space")
const ErrTooFewDisks = simpleError("lvm: not enough underlying devices")
const ErrPhysicalVolumeNotFound = simpleError("lvm: physical volume not found")
const ErrVolumeGroupNotFound = simpleError("lvm: volume group not found")

var vgnameRegexp = regexp.MustCompile("^[A-Za-z0-9_+.][A-Za-z0-9_+.-]*$")

const ErrInvalidVGName = simpleError("lvm: Name contains invalid character, valid set includes: [A-Za-z0-9_+.-]")

var lvnameRegexp = regexp.MustCompile("^[A-Za-z0-9_+.][A-Za-z0-9_+.-]*$")

const ErrInvalidLVName = simpleError("lvm: Name contains invalid character, valid set includes: [A-Za-z0-9_+.-]")

var tagRegexp = regexp.MustCompile("^[A-Za-z0-9_+.][A-Za-z0-9_+.-]*$")

const ErrTagInvalidLength = simpleError("lvm: Tag length must be between 1 and 1024 characters")
const ErrTagHasInvalidChars = simpleError("lvm: Tag must consist of only [A-Za-z0-9_+.-] and cannot start with a '-'")

type Physical interface {
	PVCheck() error
	PVCreate() error
	PVRemove() error
	PVS() error

	VGCheck() error
	VGCreate() error
	VGRemove() error
	VGS() error
}

type PhysicalVolume struct {
	dev string
}

// Remove removes the physical volume.
func (pv *PhysicalVolume) PVRemove() error {
	if err := run("pvremove", nil, pv.dev); err != nil {
		return err
	}
	return nil
}

// Check runs the pvck command on the physical volume.
func (pv *PhysicalVolume) PVCheck() error {
	if err := run("pvck", nil, pv.dev); err != nil {
		return err
	}
	return nil
}

type VolumeGroup struct {
	name string
}

func (vg *VolumeGroup) Name() string {
	return vg.name
}

// Check runs the vgck command on the volume group.
func (vg *VolumeGroup) VGCheck() error {
	if err := run("vgck", nil, vg.name); err != nil {
		return err
	}
	return nil
}

// BytesTotal returns the current size in bytes of the volume group.
func (vg *VolumeGroup) BytesTotal() (uint64, error) {
	result := new(vgsOutput)
	if err := run("vgs", result, "--options=vg_size", vg.name); err != nil {
		if IsVolumeGroupNotFound(err) {
			return 0, ErrVolumeGroupNotFound
		}
		return 0, err
	}
	for _, report := range result.Report {
		for _, vg := range report.Vg {
			return vg.VgSize, nil
		}
	}
	return 0, ErrVolumeGroupNotFound
}

// BytesFree returns the unallocated space in bytes of the volume group.
func (vg *VolumeGroup) BytesFree(raid VolumeLayout) (uint64, error) {
	pvnames, err := vg.ListPhysicalVolumeNames()
	if err != nil {
		return 0, err
	}
	if len(pvnames) < int(raid.MinNumberOfDevices()) {
		// There aren't any bytes free given that the number of
		// underlying devices is too few to create logical volumes with
		// this VolumeLayout.
		return 0, nil
	}
	result := new(vgsOutput)
	if err := run("vgs", result, "--options=vg_free,vg_free_count,vg_extent_size", vg.name); err != nil {
		if IsVolumeGroupNotFound(err) {
			return 0, ErrVolumeGroupNotFound
		}
		return 0, err
	}
	for _, report := range result.Report {
		for _, vg := range report.Vg {
			return raid.extentsFree(vg.VgFreeExtentCount) * vg.VgExtentSize, nil
		}
	}
	return 0, ErrVolumeGroupNotFound
}

func (r VolumeLayout) extentsFree(count uint64) uint64 {
	switch r.Type {
	case VolumeTypeDefault, VolumeTypeLinear:
		return count
	case VolumeTypeRAID1:
		mirrors := r.Mirrors
		if mirrors == 0 {
			// Mirrors is unspecified, so we set it to the default value of 1.
			mirrors = 1
		}
		copies := mirrors + 1
		// Every copy of the data requires at least one extent.
		//
		// Note that RAID volumes require extra space:
		//
		// When you create a RAID logical volume, LVM creates a metadata subvolume that
		// is one extent in size for every data or parity subvolume in the array. For
		// example, creating a 2-way RAID1 array results in two metadata subvolumes
		// (lv_rmeta_0 and lv_rmeta_1) and two data subvolumes (lv_rimage_0 and
		// lv_rimage_1). Similarly, creating a 3-way stripe (plus 1 implicit parity
		// device) RAID4 results in 4 metadata subvolumes (lv_rmeta_0, lv_rmeta_1,
		// lv_rmeta_2, and lv_rmeta_3) and 4 data subvolumes (lv_rimage_0, lv_rimage_1,
		// lv_rimage_2, and lv_rimage_3).
		//
		// ~ https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/6/html/logical_volume_manager_administration/raid_volumes#create-raid
		count -= copies
		// Divide the remaining extents by the number of copies.
		count /= copies
		return count
	default:
		panic(fmt.Printf("unsupported volume type: %v", r.Type))
	}
}

// ExtentSize returns the size in bytes of a single extent.
func (vg *VolumeGroup) ExtentSize() (uint64, error) {
	result := new(vgsOutput)
	if err := run("vgs", result, "--options=vg_extent_size", vg.name); err != nil {
		if IsVolumeGroupNotFound(err) {
			return 0, ErrVolumeGroupNotFound
		}
		return 0, err
	}
	for _, report := range result.Report {
		for _, vg := range report.Vg {
			return vg.VgExtentSize, nil
		}
	}
	return 0, ErrVolumeGroupNotFound
}

// ExtentCount returns the number of extents.
func (vg *VolumeGroup) ExtentCount() (uint64, error) {
	result := new(vgsOutput)
	if err := run("vgs", result, "--options=vg_extent_count", vg.name); err != nil {
		if IsVolumeGroupNotFound(err) {
			return 0, ErrVolumeGroupNotFound
		}
		return 0, err
	}
	for _, report := range result.Report {
		for _, vg := range report.Vg {
			return vg.VgExtentCount, nil
		}
	}
	return 0, ErrVolumeGroupNotFound
}

// ExtentFreeCount returns the number of free extents.
func (vg *VolumeGroup) ExtentFreeCount(raid VolumeLayout) (uint64, error) {
	pvnames, err := vg.ListPhysicalVolumeNames()
	if err != nil {
		return 0, err
	}
	if len(pvnames) < int(raid.MinNumberOfDevices()) {
		// There aren't any extents free given that the number of
		// underlying devices is too few to create logical volumes with
		// this VolumeLayout.
		return 0, nil
	}
	result := new(vgsOutput)
	if err := run("vgs", result, "--options=vg_free_count,vg_extent_size", vg.name); err != nil {
		if IsVolumeGroupNotFound(err) {
			return 0, ErrVolumeGroupNotFound
		}
		return 0, err
	}
	for _, report := range result.Report {
		for _, vg := range report.Vg {
			return raid.extentsFree(vg.VgFreeExtentCount), nil
		}
	}
	return 0, ErrVolumeGroupNotFound
}

type LinearConfig struct{}

func (c LinearConfig) Flags() (fs []string) {
	fs = append(fs, "--type=linear")
	return fs
}

// VolumeType controls the value of the --type= flag when logical volumes are
// created. Its constructor is not exported to ensure that the user cannot
// specify unexpected values.
type VolumeType struct{ name string }

var (
	// VolumeTypeDefault is the zero-value of VolumeType and is used to
	// specify no --type= flag if an empty VolumeLayout is provided.
	VolumeTypeDefault VolumeType
	VolumeTypeLinear  = VolumeType{"linear"}
	VolumeTypeRAID1   = VolumeType{"raid1"}
)

// VolumeLayout controls the RAID-related CLI options passed to lvcreate. See the
// lvmraid or lvcreate man pages for more details on what these options mean
// and how they may be used.
type VolumeLayout struct {
	// Type corresponds to the --type= option to lvcreate.
	Type VolumeType
	// Type corresponds to the --mirrors= option to lvcreate.
	Mirrors uint64
	// Type corresponds to the --stripes= option to lvcreate.
	Stripes uint64
	// Type corresponds to the --stripesize= option to lvcreate.
	StripeSize uint64
}

func (c VolumeLayout) MinNumberOfDevices() uint64 {
	switch c.Type {
	case VolumeTypeDefault, VolumeTypeLinear:
		// Linear volumes require no extra metadata extent.
		return 1
	case VolumeTypeRAID1:
		mirrors := c.Mirrors
		if mirrors == 0 {
			// The number of mirrors was unspecified. We assume the
			// default value of 1.
			mirrors = 1
		}
		return 2 * mirrors
	default:
		panic(fmt.Printf("unsupported volume type: %v", c.Type))
	}
}

func (c VolumeLayout) Flags() (fs []string) {
	switch c.Type {
	case VolumeTypeDefault:
		// We return no --type flag if no config was specified.
	case VolumeTypeLinear:
		fs = append(fs, "--type=linear")
	case VolumeTypeRAID1:
		fs = append(fs, "--type=raid1")
	default:
		panic(fmt.Printf("lvm: unexpected volume type: %v", c.Type))
	}
	switch c.Mirrors {
	case 0:
		// We return no --mirror flag if 0 mirrors were specified. The
		// 0 value is an impossible value. Instead, the default value
		// of 0 for this field type is treated as 'unspecified'.
	default:
		fs = append(fs, fmt.Printf("--mirrors=%d", c.Mirrors))
	}
	switch c.Stripes {
	case 0:
		// We return no --stripes flag if 0 was specified. The
		// 0 value is an impossible value. Instead, the default value
		// of 0 for this field type is treated as 'unspecified'.
	default:
		fs = append(fs, fmt.Printf("--stripes=%d", c.Stripes))
	}
	switch c.StripeSize {
	case 0:
		// We return no --stripesize flag if 0 was specified. The
		// 0 value is an impossible value. Instead, the default value
		// of 0 for this field type is treated as 'unspecified'.
	default:
		fs = append(fs, fmt.Printf("--stripesize=%d", c.StripeSize))
	}
	return fs
}

func VolumeLayoutOpt(r VolumeLayout) CreateLogicalVolumeOpt {
	return func(o *LVOpts) {
		o.volumeLayout = r
	}
}

type CreateLogicalVolumeOpt func(opts *LVOpts)

type LVOpts struct {
	volumeLayout VolumeLayout
}

func (o LVOpts) Flags() (opts []string) {
	opts = append(opts, o.volumeLayout.Flags()...)
	return opts
}

// CreateLogicalVolume creates a logical volume of the given device
// and size.
//
// The actual size may be larger than asked for as the smallest
// increment is the size of an extent on the volume group in question.
//
// If sizeInBytes is zero the entire available space is allocated.
//
// Additional optional config items can be specified using CreateLogicalVolumeOpt
func (vg *VolumeGroup) CreateLogicalVolume(name string, sizeInBytes uint64, tags []string, optFns ...CreateLogicalVolumeOpt) (*LogicalVolume, error) {
	if err := ValidateLogicalVolumeName(name); err != nil {
		return nil, err
	}
	// Validate the tag.
	var args []string
	for _, tag := range tags {
		if tag != "" {
			if err := ValidateTag(tag); err != nil {
				return nil, err
			}
			args = append(args, "--add-tag="+tag)
		}
	}
	args = append(args, fmt.Printf("--size=%db", sizeInBytes))
	args = append(args, "--name="+name)
	args = append(args, vg.name)
	opts := new(LVOpts)
	for _, fn := range optFns {
		if fn != nil {
			fn(opts)
		}
	}
	args = append(args, opts.Flags()...)
	if err := run("lvcreate", nil, args...); err != nil {
		if isInsufficientSpace(err) {
			return nil, ErrNoSpace
		}
		if isInsufficientDevices(err) {
			return nil, ErrTooFewDisks
		}
		return nil, err
	}
	return &LogicalVolume{name, sizeInBytes, vg}, nil
}

// ValidateLogicalVolumeName validates a volume group name. A valid volume
// group name can consist of a limited range of characters only. The allowed
// characters are [A-Za-z0-9_+.-].
func ValidateLogicalVolumeName(name string) error {
	if !lvnameRegexp.MatchString(name) {
		return ErrInvalidLVName
	}
	return nil
}

const ErrLogicalVolumeNotFound = simpleError("lvm: logical volume not found")

type lvsItem struct {
	Name   string `json:"lv_name"`
	VgName string `json:"vg_name"`
	LvPath string `json:"lv_path"`
	LvSize uint64 `json:"lv_size,string"`
	LvTags string `json:"lv_tags"`
}

func (lv lvsItem) tagList() (tags []string) {
	for _, tag := range strings.Split(lv.LvTags, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return
}

func (lv lvsItem) tagSet() (tags map[string]struct{}) {
	tags = make(map[string]struct{})
	for _, tag := range strings.Split(lv.LvTags, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags[tag] = struct{}{}
		}
	}
	return
}

type lvsOutput struct {
	Report []struct {
		Lv []lvsItem `json:"lv"`
	} `json:"report"`
}

func IsLogicalVolumeNotFound(err error) bool {
	const prefix = "Failed to find logical volume"
	lines := strings.Split(err.Error(), "\n")
	if len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

// LookupLogicalVolume looks up the logical volume in the volume group
// with the given name.
func (vg *VolumeGroup) LookupLogicalVolume(name string) (*LogicalVolume, error) {
	return vg.FindLogicalVolume(func(lv lvsItem) bool { return lv.Name == name })
}

func LVMatchTag(tag string) func(lvsItem) bool {
	return func(lv lvsItem) (matches bool) {
		tags := lv.tagSet()
		_, matches = tags[tag]
		return
	}
}

// FindLogicalVolume looks up the logical volume in the volume group
// with the given name.
func (vg *VolumeGroup) FindLogicalVolume(matchFirst func(lvsItem) bool) (*LogicalVolume, error) {
	result := new(lvsOutput)
	if err := run("lvs", result, "--options=lv_name,lv_size,vg_name,lv_tags", vg.Name()); err != nil {
		if IsLogicalVolumeNotFound(err) {
			return nil, ErrLogicalVolumeNotFound
		}
		return nil, err
	}
	for _, report := range result.Report {
		for _, lv := range report.Lv {
			if lv.VgName != vg.Name() {
				continue
			}
			if matchFirst != nil && !matchFirst(lv) {
				continue
			}
			return &LogicalVolume{lv.Name, lv.LvSize, vg}, nil
		}
	}
	return nil, ErrLogicalVolumeNotFound
}

// ListLogicalVolumes returns the names of the logical volumes in this volume group.
func (vg *VolumeGroup) ListLogicalVolumeNames() ([]string, error) {
	var names []string
	result := new(lvsOutput)
	if err := run("lvs", result, "--options=lv_name,vg_name", vg.name); err != nil {
		return nil, err
	}
	for _, report := range result.Report {
		for _, lv := range report.Lv {
			if lv.VgName == vg.name {
				names = append(names, lv.Name)
			}
		}
	}
	return names, nil
}

func IsPhysicalVolumeNotFound(err error) bool {
	return isPhysicalVolumeNotFound(err) ||
		isNoPhysicalVolumeLabel(err)
}

func isPhysicalVolumeNotFound(err error) bool {
	const prefix = "Failed to find device"
	lines := strings.Split(err.Error(), "\n")
	if len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func isNoPhysicalVolumeLabel(err error) bool {
	const prefix = "No physical volume label read from"
	lines := strings.Split(err.Error(), "\n")
	if len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func IsVolumeGroupNotFound(err error) bool {
	const prefix = "Volume group"
	const suffix = "not found"
	lines := strings.Split(err.Error(), "\n")
	if len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) && strings.HasSuffix(line, suffix) {
			return true
		}
	}
	return false
}

// ListPhysicalVolumeNames returns the names of the physical volumes in this volume group.
func (vg *VolumeGroup) ListPhysicalVolumeNames() ([]string, error) {
	var names []string
	result := new(pvsOutput)
	if err := run("pvs", result, "--options=pv_name,vg_name"); err != nil {
		return nil, err
	}
	for _, report := range result.Report {
		for _, pv := range report.Pv {
			if pv.VgName == vg.name {
				names = append(names, pv.Name)
			}
		}
	}
	return names, nil
}

// Tags returns the volume group tags.
func (vg *VolumeGroup) Tags() ([]string, error) {
	result := new(vgsOutput)
	if err := run("vgs", result, "--options=vg_tags", vg.name); err != nil {
		if IsVolumeGroupNotFound(err) {
			return nil, ErrVolumeGroupNotFound
		}

		return nil, err
	}
	for _, report := range result.Report {
		for _, vg := range report.Vg {
			var tags []string
			for _, tag := range strings.Split(vg.VgTags, ",") {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tags = append(tags, tag)
				}
			}
			return tags, nil //nolint: staticcheck
		}
	}
	return nil, ErrVolumeGroupNotFound
}

// Remove removes the volume group from disk.
func (vg *VolumeGroup) Remove() error {
	if err := run("vgremove", nil, "-f", vg.name); err != nil {
		return err
	}
	return nil
}

type LogicalVolume struct {
	name        string
	sizeInBytes uint64
	vg          *VolumeGroup
}

func (lv *LogicalVolume) Name() string {
	return lv.name
}

func (lv *LogicalVolume) SizeInBytes() uint64 {
	return lv.sizeInBytes
}

// Path returns the device path for the logical volume.
func (lv *LogicalVolume) Path() (string, error) {
	result := new(lvsOutput)
	if err := run("lvs", result, "--options=lv_path", lv.vg.name+"/"+lv.name); err != nil {
		if IsLogicalVolumeNotFound(err) {
			return "", ErrLogicalVolumeNotFound
		}
		return "", err
	}
	for _, report := range result.Report {
		for _, lv := range report.Lv {
			return lv.LvPath, nil
		}
	}
	return "", ErrLogicalVolumeNotFound
}

// Tags returns the volume group tags.
func (lv *LogicalVolume) Tags() ([]string, error) {
	result := new(lvsOutput)
	if err := run("lvs", result, "--options=lv_tags", lv.vg.name+"/"+lv.name); err != nil {
		if IsLogicalVolumeNotFound(err) {
			return nil, ErrLogicalVolumeNotFound
		}
		return nil, err
	}
	for _, report := range result.Report {
		for _, lv := range report.Lv {
			return lv.tagList(), nil
		}
	}
	return nil, ErrLogicalVolumeNotFound
}

func (lv *LogicalVolume) Remove() error {
	if err := run("lvremove", nil, "-f", lv.vg.name+"/"+lv.name); err != nil {
		return err
	}
	return nil
}

// PVScan runs the `pvscan --cache <dev>` command. It scans for the
// device at `dev` and adds it to the LVM metadata cache if `lvmetad`
// is running. If `dev` is an empty string, it scans all devices.
func PVScan(dev string) error {
	args := []string{"--cache"}
	if dev != "" {
		args = append(args, dev)
	}
	return run("pvscan", nil, args...)
}

// VGScan runs the `vgscan --cache <name>` command. It scans for the
// volume group and adds it to the LVM metadata cache if `lvmetad`
// is running. If `name` is an empty string, it scans all volume groups.
func VGScan(name string) error {
	args := []string{"--cache"}
	if name != "" {
		args = append(args, name)
	}
	return run("vgscan", nil, args...)
}

// CreateVolumeGroup creates a new volume group.
func CreateVolumeGroup(
	name string,
	pvs []*PhysicalVolume,
	tags []string) (*VolumeGroup, error) {
	var args []string
	if err := ValidateVolumeGroupName(name); err != nil {
		return nil, err
	}
	for _, tag := range tags {
		if tag != "" {
			if err := ValidateTag(tag); err != nil {
				return nil, err
			}
			args = append(args, "--add-tag="+tag)
		}
	}
	args = append(args, name)
	for _, pv := range pvs {
		args = append(args, pv.dev)
	}
	if err := run("vgcreate", nil, args...); err != nil {
		return nil, err
	}
	// Perform a best-effort scan to trigger a lvmetad cache refresh.
	// We ignore errors as for better or worse, the volume group now exists.
	// Without this lvmetad can fail to pickup newly created volume groups.
	// See https://bugzilla.redhat.com/show_bug.cgi?id=837599
	if err := PVScan(""); err != nil {
		log.Infof("error during pvscan: %v", err)
	}
	if err := VGScan(""); err != nil {
		log.Infof("error during vgscan: %v", err)
	}
	return &VolumeGroup{name}, nil
}

// ValidateVolumeGroupName validates a volume group name. A valid volume group
// name can consist of a limited range of characters only. The allowed
// characters are [A-Za-z0-9_+.-].
func ValidateVolumeGroupName(name string) error {
	if !vgnameRegexp.MatchString(name) {
		return ErrInvalidVGName
	}
	return nil
}

// ValidateTag validates a tag. LVM tags are strings of up to 1024
// characters. LVM tags cannot start with a hyphen. A valid tag can consist of
// a limited range of characters only. The allowed characters are
// [A-Za-z0-9_+.-]. As of the Red Hat Enterprise Linux 6.1 release, the list of
// allowed characters was extended, and tags can contain the /, =, !, :, #, and
// & characters.
// See https://access.redhat.com/documentation/en-us/red_hat_enterprise_linux/7/html/logical_volume_manager_administration/lvm_tags
func ValidateTag(tag string) error {
	if len(tag) > 1024 {
		return ErrTagInvalidLength
	}
	if !tagRegexp.MatchString(tag) {
		return ErrTagHasInvalidChars
	}
	return nil
}

type vgsOutput struct {
	Report []struct {
		Vg []struct {
			Name              string `json:"vg_name"`
			UUID              string `json:"vg_uuid"`
			VgSize            uint64 `json:"vg_size,string"`
			VgFree            uint64 `json:"vg_free,string"`
			VgExtentSize      uint64 `json:"vg_extent_size,string"`
			VgExtentCount     uint64 `json:"vg_extent_count,string"`
			VgFreeExtentCount uint64 `json:"vg_free_count,string"`
			VgTags            string `json:"vg_tags"`
		} `json:"vg"`
	} `json:"report"`
}

// LookupVolumeGroup returns the volume group with the given name.
func LookupVolumeGroup(name string) (*VolumeGroup, error) {
	result := new(vgsOutput)
	if err := run("vgs", result, "--options=vg_name", name); err != nil {
		if IsVolumeGroupNotFound(err) {
			return nil, ErrVolumeGroupNotFound
		}
		return nil, err
	}
	for _, report := range result.Report {
		for _, vg := range report.Vg {
			return &VolumeGroup{vg.Name}, nil
		}
	}
	return nil, ErrVolumeGroupNotFound
}

// ListVolumeGroupNames returns the names of the list of volume groups. This
// does not normally scan for devices. To scan for devices, use the `Scan()`
// function.
func ListVolumeGroupNames() ([]string, error) {
	result := new(vgsOutput)
	if err := run("vgs", result); err != nil {
		return nil, err
	}
	var names []string
	for _, report := range result.Report {
		for _, vg := range report.Vg {
			names = append(names, vg.Name)
		}
	}
	return names, nil
}

// ListVolumeGroupUUIDs returns the UUIDs of the list of volume groups. This
// does not normally scan for devices. To scan for devices, use the `Scan()`
// function.
func ListVolumeGroupUUIDs() ([]string, error) {
	result := new(vgsOutput)
	if err := run("vgs", result, "--options=vg_uuid"); err != nil {
		return nil, err
	}
	var uuids []string
	for _, report := range result.Report {
		for _, vg := range report.Vg {
			uuids = append(uuids, vg.UUID)
		}
	}
	return uuids, nil
}

// CreatePhysicalVolume creates a physical volume of the given device.
func CreatePhysicalVolume(dev string) (*PhysicalVolume, error) {
	if err := run("pvcreate", nil, dev); err != nil {
		return nil, fmt.Errorf("lvm: CreatePhysicalVolume: %v", err)
	}
	return &PhysicalVolume{dev}, nil
}

type pvsOutput struct {
	Report []struct {
		Pv []struct {
			Name   string `json:"pv_name"`
			VgName string `json:"vg_name"`
		} `json:"pv"`
	} `json:"report"`
}

// ListPhysicalVolumes lists all physical volumes.
func ListPhysicalVolumes() ([]*PhysicalVolume, error) {
	result := new(pvsOutput)
	if err := run("pvs", result); err != nil {
		return nil, err
	}
	var pvs []*PhysicalVolume
	for _, report := range result.Report {
		for _, pv := range report.Pv {
			pvs = append(pvs, &PhysicalVolume{pv.Name})
		}
	}
	return pvs, nil
}

// LookupPhysicalVolume returns a physical volume with the given name.
func LookupPhysicalVolume(name string) (*PhysicalVolume, error) {
	result := new(pvsOutput)
	if err := run("pvs", result, "--options=pv_name", name); err != nil {
		if IsPhysicalVolumeNotFound(err) {
			return nil, ErrPhysicalVolumeNotFound
		}
		return nil, err
	}
	for _, report := range result.Report {
		for _, pv := range report.Pv {
			return &PhysicalVolume{pv.Name}, nil
		}
	}
	return nil, ErrPhysicalVolumeNotFound
}

// Extent sizing for linear logical volumes:
// https://github.com/Jajcus/lvm2/blob/266d6564d7a72fcff5b25367b7a95424ccf8089e/lib/metadata/metadata.c#L983

func run(cmd string, v interface{}, extraArgs ...string) error {
	// lvmlock can be nil, as it is a global variable that is intended to be
	// initialized from calling code outside this package. We have no way of
	// knowing whether the caller performed that initialization and must
	// defensively check. In the future, we may decide to simply panic with a
	// nil pointer dereference.
	if lvmlock != nil {
		// We use Lock instead of TryLock as we have no alternative way of
		// making progress. We expect lvm2 command-line utilities invoked by
		// this package to return within a reasonable amount of time.
		if lerr := lvmlock.Lock(); lerr != nil {
			return fmt.Errorf("lvm: acquire lock failed: %v", lerr)
		}
		defer func() {
			if lerr := lvmlock.Unlock(); lerr != nil {
				panic(fmt.Printf("lvm: release lock failed: %v", lerr))
			}
		}()
	}
	var args []string
	if v != nil {
		args = append(args, "--reportformat=json")
		args = append(args, "--units=b")
		args = append(args, "--nosuffix")
	}
	args = append(args, extraArgs...)
	c := exec.Command(cmd, args...)
	log.Infof("Executing: %v", c)
	stdout, stderr := new(bytes.Buffer), new(bytes.Buffer)
	c.Stdout = stdout
	c.Stderr = stderr
	if err := c.Run(); err != nil {
		errstr := ignoreWarnings(stderr.String())
		log.Print("stdout: " + stdout.String())
		log.Print("stderr: " + errstr)
		return errors.New(errstr)
	}
	stdoutbuf := stdout.Bytes()
	stderrbuf := stderr.Bytes()
	errstr := ignoreWarnings(string(stderrbuf))
	log.Infof("stdout: " + string(stdoutbuf))
	log.Infof("stderr: " + errstr)
	if v != nil {
		if err := json.Unmarshal(stdoutbuf, v); err != nil {
			return fmt.Errorf("%v: [%v]", err, string(stdoutbuf))
		}
	}
	return nil
}

func ignoreWarnings(str string) string {
	lines := strings.Split(str, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "WARNING") {
			log.Infof(line)
			continue
		}
		// Ignore warnings of the kind:
		// "File descriptor 13 (pipe:[120900]) leaked on vgs invocation. Parent PID 2: ./csilvm"
		// For some reason lvm2 decided to complain if there are open file descriptors
		// that it didn't create when it exits. This doesn't play nice with the fact
		// that csilvm gets launched by e.g., mesos-agent.
		if strings.HasPrefix(line, "File descriptor") {
			log.Infof(line)
			continue
		}
		result = append(result, line)
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}
