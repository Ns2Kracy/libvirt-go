package libvirt

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// Connect is a reference-counted connection to a libvirt hypervisor driver.
// Connect must not be copied after first use.
type Connect struct {
	mu  sync.RWMutex
	api *nativeAPI
	ptr unsafe.Pointer
}

// NewConnect opens a read-write libvirt connection. An empty URI asks libvirt
// to select its configured default.
func NewConnect(uri string) (*Connect, error) {
	return newConnect(uri, false)
}

// NewConnectReadOnly opens a read-only libvirt connection. An empty URI asks
// libvirt to select its configured default.
func NewConnectReadOnly(uri string) (*Connect, error) {
	return newConnect(uri, true)
}

func newConnect(uri string, readOnly bool) (*Connect, error) {
	api, err := getNativeAPI()
	if err != nil {
		return nil, err
	}
	buf, uriPtr, err := makeCString("URI", uri, true)
	if err != nil {
		return nil, err
	}

	operation := "virConnectOpen"
	open := api.virConnectOpen
	if readOnly {
		operation = "virConnectOpenReadOnly"
		open = api.virConnectOpenReadOnly
	}
	ptr, err := nativeCall(api, operation, func() (unsafe.Pointer, bool) {
		result := open(uriPtr)
		return result, result == nil
	})
	runtime.KeepAlive(buf)
	if err != nil {
		return nil, err
	}
	return &Connect{api: api, ptr: ptr}, nil
}

// Close releases this wrapper's connection reference. The returned value is
// libvirt's indication of whether other references remain.
func (c *Connect) Close() (int, error) {
	if c == nil {
		return 0, fmt.Errorf("%w: connection", ErrClosed)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ptr == nil {
		return 0, fmt.Errorf("%w: connection", ErrClosed)
	}

	result, err := nativeCall(c.api, "virConnectClose", func() (int32, bool) {
		value := c.api.virConnectClose(c.ptr)
		return value, value < 0
	})
	if err != nil {
		return int(result), err
	}
	c.ptr = nil
	return int(result), nil
}

// GetURI returns the canonical URI for the connection.
func (c *Connect) GetURI() (string, error) {
	if c == nil {
		return "", fmt.Errorf("%w: connection", ErrClosed)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.ptr == nil {
		return "", fmt.Errorf("%w: connection", ErrClosed)
	}

	ptr, err := nativeCall(c.api, "virConnectGetURI", func() (unsafe.Pointer, bool) {
		value := c.api.virConnectGetURI(c.ptr)
		return value, value == nil
	})
	if err != nil {
		return "", err
	}
	defer c.api.free(ptr)
	return copyCString(ptr), nil
}

// GetLibVersion returns the version of the libvirt library used by this connection.
func (c *Connect) GetLibVersion() (uint64, error) {
	return c.getVersion("virConnectGetLibVersion", func(api *nativeAPI, ptr unsafe.Pointer, version *uintptr) int32 {
		return api.virConnectGetLibVersion(ptr, version)
	})
}

// GetVersion returns the version of the connected hypervisor.
func (c *Connect) GetVersion() (uint64, error) {
	return c.getVersion("virConnectGetVersion", func(api *nativeAPI, ptr unsafe.Pointer, version *uintptr) int32 {
		return api.virConnectGetVersion(ptr, version)
	})
}

func (c *Connect) getVersion(operation string, get func(*nativeAPI, unsafe.Pointer, *uintptr) int32) (uint64, error) {
	if c == nil {
		return 0, fmt.Errorf("%w: connection", ErrClosed)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.ptr == nil {
		return 0, fmt.Errorf("%w: connection", ErrClosed)
	}

	var version uintptr
	_, err := nativeCall(c.api, operation, func() (int32, bool) {
		result := get(c.api, c.ptr, &version)
		return result, result < 0
	})
	return uint64(version), err
}

// IsAlive reports whether the connection is still alive.
func (c *Connect) IsAlive() (bool, error) {
	if c == nil {
		return false, fmt.Errorf("%w: connection", ErrClosed)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.ptr == nil {
		return false, fmt.Errorf("%w: connection", ErrClosed)
	}

	result, err := nativeCall(c.api, "virConnectIsAlive", func() (int32, bool) {
		value := c.api.virConnectIsAlive(c.ptr)
		return value, value < 0
	})
	return result == 1, err
}

// ListAllDomains returns domains matching flags. Every returned Domain owns a
// reference that the caller must release with Free.
func (c *Connect) ListAllDomains(flags ConnectListAllDomainsFlags) ([]*Domain, error) {
	if c == nil {
		return nil, fmt.Errorf("%w: connection", ErrClosed)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.ptr == nil {
		return nil, fmt.Errorf("%w: connection", ErrClosed)
	}

	var list unsafe.Pointer
	count, err := nativeCall(c.api, "virConnectListAllDomains", func() (int32, bool) {
		value := c.api.virConnectListAllDomains(c.ptr, &list, uint32(flags))
		return value, value < 0
	})
	if err != nil {
		return nil, err
	}
	if list != nil {
		defer c.api.free(list)
	}
	if count == 0 {
		return []*Domain{}, nil
	}
	if list == nil {
		return nil, fmt.Errorf("libvirt: virConnectListAllDomains returned %d domains with a nil array", count)
	}

	handles := unsafe.Slice((*unsafe.Pointer)(list), int(count))
	domains := make([]*Domain, len(handles))
	for i, handle := range handles {
		// Each generated array entry is an opaque virDomainPtr reference.
		domains[i] = &Domain{api: c.api, ptr: handle}
	}
	return domains, nil
}

// LookupDomainByName returns a referenced domain handle. The caller must call Free.
func (c *Connect) LookupDomainByName(name string) (*Domain, error) {
	return c.domainFromString("domain name", name, "virDomainLookupByName", c.apiDomainLookupByName)
}

// DefineDomainXML defines a persistent domain and returns a referenced handle.
// The caller must call Free.
func (c *Connect) DefineDomainXML(xml string) (*Domain, error) {
	return c.domainFromString("domain XML", xml, "virDomainDefineXML", c.apiDomainDefineXML)
}

func (c *Connect) domainFromString(field, value, operation string, call func(*nativeAPI, unsafe.Pointer, *byte) unsafe.Pointer) (*Domain, error) {
	if c == nil {
		return nil, fmt.Errorf("%w: connection", ErrClosed)
	}
	buf, valuePtr, err := makeCString(field, value, false)
	if err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.ptr == nil {
		return nil, fmt.Errorf("%w: connection", ErrClosed)
	}
	ptr, err := nativeCall(c.api, operation, func() (unsafe.Pointer, bool) {
		result := call(c.api, c.ptr, valuePtr)
		return result, result == nil
	})
	runtime.KeepAlive(buf)
	if err != nil {
		return nil, err
	}
	return &Domain{api: c.api, ptr: ptr}, nil
}

func (c *Connect) apiDomainLookupByName(api *nativeAPI, ptr unsafe.Pointer, value *byte) unsafe.Pointer {
	return api.virDomainLookupByName(ptr, value)
}

func (c *Connect) apiDomainDefineXML(api *nativeAPI, ptr unsafe.Pointer, value *byte) unsafe.Pointer {
	return api.virDomainDefineXML(ptr, value)
}
