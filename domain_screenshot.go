package libvirt

import (
	"fmt"
	"unsafe"
)

// Screenshot starts a screenshot transfer on a stream from the same connection.
// The caller owns the stream and must finish or abort it, then free it.
func (d *Domain) Screenshot(stream *Stream, screen, flags uint32) (string, error) {
	object := streamObject(stream)
	if d == nil || object == nil {
		return "", fmt.Errorf("%w: domain or stream", ErrClosed)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	object.mu.RLock()
	defer object.mu.RUnlock()
	if d.ptr == nil || object.ptr == nil {
		return "", fmt.Errorf("%w: domain or stream", ErrClosed)
	}
	if d.api != object.api {
		return "", fmt.Errorf("domain and stream use different libvirt APIs")
	}

	ptr, err := nativeCall(d.api, "virDomainScreenshot", func() (unsafe.Pointer, bool) {
		value := d.api.virDomainScreenshot(d.ptr, object.ptr, screen, flags)
		return value, value == nil
	})
	if err != nil {
		return "", err
	}
	defer d.api.free(ptr)
	return copyCString(ptr), nil
}
