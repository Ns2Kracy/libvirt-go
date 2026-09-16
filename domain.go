package libvirt

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

const uuidStringBufferLength = 37

// Domain is a reference-counted libvirt domain handle.
type Domain struct {
	mu  *sync.RWMutex
	api *nativeAPI
	ptr unsafe.Pointer
}

func newDomain(api *nativeAPI, ptr unsafe.Pointer) *Domain {
	return &Domain{mu: new(sync.RWMutex), api: api, ptr: ptr}
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

// Reset immediately resets a running domain.
func (d *Domain) Reset(flags uint32) error {
	return d.callStatus("virDomainReset", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainReset(ptr, flags)
	})
}

// Reboot requests a guest reboot.
func (d *Domain) Reboot(flags uint32) error {
	return d.callStatus("virDomainReboot", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainReboot(ptr, flags)
	})
}

// Resume resumes a suspended domain.
func (d *Domain) Resume() error {
	return d.callStatus("virDomainResume", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainResume(ptr)
	})
}

// Suspend pauses a running domain.
func (d *Domain) Suspend() error {
	return d.callStatus("virDomainSuspend", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainSuspend(ptr)
	})
}

// DestroyFlags immediately stops a running domain with the requested behavior.
func (d *Domain) DestroyFlags(flags uint32) error {
	return d.callStatus("virDomainDestroyFlags", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainDestroyFlags(ptr, flags)
	})
}

// AttachDeviceFlags attaches a device described by XML.
func (d *Domain) AttachDeviceFlags(xml string, flags uint32) error {
	return d.modifyDevice("virDomainAttachDeviceFlags", xml, flags, func(api *nativeAPI, ptr unsafe.Pointer, xml *byte, flags uint32) int32 {
		return api.virDomainAttachDeviceFlags(ptr, xml, flags)
	})
}

// DetachDeviceFlags detaches a device described by XML.
func (d *Domain) DetachDeviceFlags(xml string, flags uint32) error {
	return d.modifyDevice("virDomainDetachDeviceFlags", xml, flags, func(api *nativeAPI, ptr unsafe.Pointer, xml *byte, flags uint32) int32 {
		return api.virDomainDetachDeviceFlags(ptr, xml, flags)
	})
}

func (d *Domain) modifyDevice(operation, xml string, flags uint32, call func(*nativeAPI, unsafe.Pointer, *byte, uint32) int32) error {
	buffer, xmlPtr, err := makeCString("device XML", xml, false)
	if err != nil {
		return err
	}
	_, err = domainCall(d, operation, func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := call(api, ptr, xmlPtr, flags)
		return result, result < 0
	})
	runtime.KeepAlive(buffer)
	return err
}

// SendKey sends keycodes to a domain using the selected keycode set.
func (d *Domain) SendKey(codeSet, holdTime uint, codes []uint, flags uint32) error {
	const maxKeys = 16
	if len(codes) > maxKeys {
		return fmt.Errorf("libvirt: send key accepts at most %d keycodes", maxKeys)
	}
	if uint64(codeSet) > uint64(^uint32(0)) || uint64(holdTime) > uint64(^uint32(0)) {
		return fmt.Errorf("libvirt: send key arguments exceed uint32")
	}
	var nativeCodes [maxKeys]uint32
	for i, code := range codes {
		if uint64(code) > uint64(^uint32(0)) {
			return fmt.Errorf("libvirt: keycode %d exceeds uint32", code)
		}
		nativeCodes[i] = uint32(code)
	}
	var codesPtr *uint32
	if len(codes) != 0 {
		codesPtr = &nativeCodes[0]
	}
	_, err := domainCall(d, "virDomainSendKey", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := api.virDomainSendKey(ptr, uint32(codeSet), uint32(holdTime), codesPtr, int32(len(codes)), flags)
		return result, result < 0
	})
	runtime.KeepAlive(nativeCodes)
	return err
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

// GetAutostart reports whether the domain starts with the host.
func (d *Domain) GetAutostart() (bool, error) {
	value, err := domainCall(d, "virDomainGetAutostart", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		var autostart int32
		result := api.virDomainGetAutostart(ptr, &autostart)
		return autostart, result < 0
	})
	return value != 0, err
}

// SetAutostart changes whether the domain starts with the host.
func (d *Domain) SetAutostart(autostart bool) error {
	value := int32(0)
	if autostart {
		value = 1
	}
	_, err := domainCall(d, "virDomainSetAutostart", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := api.virDomainSetAutostart(ptr, value)
		return result, result < 0
	})
	return err
}

// Undefine removes a persistent domain definition.
func (d *Domain) Undefine() error {
	return d.callStatus("virDomainUndefine", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainUndefine(ptr)
	})
}

// UndefineFlags removes a persistent domain definition with flags.
func (d *Domain) UndefineFlags(flags uint32) error {
	return d.callStatus("virDomainUndefineFlags", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainUndefineFlags(ptr, flags)
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
