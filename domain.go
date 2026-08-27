package libvirt

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

const uuidStringBufferLength = 37

// Domain is a reference-counted libvirt domain handle. Domain must not be
// copied after first use.
type Domain struct {
	mu  sync.RWMutex
	api *nativeAPI
	ptr unsafe.Pointer
}

// Free releases this wrapper's domain reference.
func (d *Domain) Free() error {
	if d == nil {
		return fmt.Errorf("%w: domain", ErrClosed)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ptr == nil {
		return fmt.Errorf("%w: domain", ErrClosed)
	}

	_, err := nativeCall(d.api, "virDomainFree", func() (int32, bool) {
		result := d.api.virDomainFree(d.ptr)
		return result, result < 0
	})
	if err == nil {
		d.ptr = nil
	}
	return err
}

// GetName returns the domain's public name.
func (d *Domain) GetName() (string, error) {
	if d == nil {
		return "", fmt.Errorf("%w: domain", ErrClosed)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.ptr == nil {
		return "", fmt.Errorf("%w: domain", ErrClosed)
	}

	ptr, err := nativeCall(d.api, "virDomainGetName", func() (unsafe.Pointer, bool) {
		value := d.api.virDomainGetName(d.ptr)
		return value, value == nil
	})
	if err != nil {
		return "", err
	}
	return copyCString(ptr), nil
}

// GetUUIDString returns the domain UUID in canonical RFC 4122 form.
func (d *Domain) GetUUIDString() (string, error) {
	if d == nil {
		return "", fmt.Errorf("%w: domain", ErrClosed)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.ptr == nil {
		return "", fmt.Errorf("%w: domain", ErrClosed)
	}

	var buf [uuidStringBufferLength]byte
	_, err := nativeCall(d.api, "virDomainGetUUIDString", func() (int32, bool) {
		result := d.api.virDomainGetUUIDString(d.ptr, &buf[0])
		return result, result < 0
	})
	runtime.KeepAlive(&buf)
	if err != nil {
		return "", err
	}
	return string(buf[:uuidStringBufferLength-1]), nil
}

// GetState returns the current domain state and the state-specific reason code.
func (d *Domain) GetState() (DomainState, int32, error) {
	if d == nil {
		return DomainNoState, 0, fmt.Errorf("%w: domain", ErrClosed)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.ptr == nil {
		return DomainNoState, 0, fmt.Errorf("%w: domain", ErrClosed)
	}

	var state, reason int32
	_, err := nativeCall(d.api, "virDomainGetState", func() (int32, bool) {
		result := d.api.virDomainGetState(d.ptr, &state, &reason, 0)
		return result, result < 0
	})
	return DomainState(state), reason, err
}

// IsActive reports whether the domain is currently running.
func (d *Domain) IsActive() (bool, error) {
	if d == nil {
		return false, fmt.Errorf("%w: domain", ErrClosed)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.ptr == nil {
		return false, fmt.Errorf("%w: domain", ErrClosed)
	}

	result, err := nativeCall(d.api, "virDomainIsActive", func() (int32, bool) {
		value := d.api.virDomainIsActive(d.ptr)
		return value, value < 0
	})
	return result == 1, err
}

// GetXMLDesc returns the domain XML. The native allocation is copied into Go
// memory and released before this method returns.
func (d *Domain) GetXMLDesc(flags DomainXMLFlags) (string, error) {
	if d == nil {
		return "", fmt.Errorf("%w: domain", ErrClosed)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.ptr == nil {
		return "", fmt.Errorf("%w: domain", ErrClosed)
	}

	ptr, err := nativeCall(d.api, "virDomainGetXMLDesc", func() (unsafe.Pointer, bool) {
		value := d.api.virDomainGetXMLDesc(d.ptr, uint32(flags))
		return value, value == nil
	})
	if err != nil {
		return "", err
	}
	defer d.api.free(ptr)
	return copyCString(ptr), nil
}

// Create starts an inactive domain.
func (d *Domain) Create() error {
	return d.callStatus("virDomainCreate", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainCreate(ptr)
	})
}

// Shutdown requests a graceful guest shutdown.
func (d *Domain) Shutdown() error {
	return d.callStatus("virDomainShutdown", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainShutdown(ptr)
	})
}

// Destroy immediately stops a running domain.
func (d *Domain) Destroy() error {
	return d.callStatus("virDomainDestroy", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainDestroy(ptr)
	})
}

func (d *Domain) callStatus(operation string, call func(*nativeAPI, unsafe.Pointer) int32) error {
	if d == nil {
		return fmt.Errorf("%w: domain", ErrClosed)
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.ptr == nil {
		return fmt.Errorf("%w: domain", ErrClosed)
	}

	_, err := nativeCall(d.api, operation, func() (int32, bool) {
		result := call(d.api, d.ptr)
		return result, result < 0
	})
	return err
}
