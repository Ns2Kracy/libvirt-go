package libvirt

import "fmt"

// Version is a decoded libvirt version number.
type Version struct {
	Major   uint32
	Minor   uint32
	Release uint32
}

// DecodeVersion decodes libvirt's major*1,000,000 + minor*1,000 + release format.
func DecodeVersion(raw uint64) Version {
	return Version{
		Major:   uint32(raw / 1_000_000),
		Minor:   uint32((raw % 1_000_000) / 1_000),
		Release: uint32(raw % 1_000),
	}
}

// Number returns the version in libvirt's integer representation.
func (v Version) Number() uint64 {
	return uint64(v.Major)*1_000_000 + uint64(v.Minor)*1_000 + uint64(v.Release)
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Release)
}

// GetVersion returns the version of the dynamically loaded libvirt library.
func GetVersion() (uint64, error) {
	api, err := getNativeAPI()
	if err != nil {
		return 0, err
	}
	var version uintptr
	_, err = nativeCall(api, "virGetVersion", func() (int32, bool) {
		result := api.virGetVersion(&version, nil, nil)
		return result, result < 0
	})
	return uint64(version), err
}
