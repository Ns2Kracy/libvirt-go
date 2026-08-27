//go:build !linux && !darwin && !freebsd && !netbsd

package libvirt

func loadNativeAPI() (*nativeAPI, error) {
	return nil, ErrUnsupportedPlatform
}
