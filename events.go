package libvirt

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// DomainLifecycleEvent is delivered for libvirt domain lifecycle changes.
type DomainLifecycleEvent struct {
	DomainName string
	Event      int32
	Detail     int32
}

type callbackRecord struct {
	api       *nativeAPI
	close     func(int32)
	lifecycle func(DomainLifecycleEvent)
}

var callbackRecords sync.Map

var callbackFreePointer = purego.NewCallback(func(opaque unsafe.Pointer) {
	value, ok := callbackRecords.LoadAndDelete(opaque)
	if !ok {
		return
	}
	record := value.(*callbackRecord)
	record.api.free(opaque)
})

var closeCallbackPointer = purego.NewCallback(func(_ unsafe.Pointer, reason int32, opaque unsafe.Pointer) {
	value, ok := callbackRecords.Load(opaque)
	if !ok {
		return
	}
	record := value.(*callbackRecord)
	if record.close != nil {
		callback := record.close
		go invokeCallback(func() { callback(reason) })
	}
})

var domainLifecycleCallbackPointer = purego.NewCallback(func(_ unsafe.Pointer, domain unsafe.Pointer, event int32, detail int32, opaque unsafe.Pointer) int32 {
	value, ok := callbackRecords.Load(opaque)
	if !ok {
		return 0
	}
	record := value.(*callbackRecord)
	if record.lifecycle == nil {
		return 0
	}
	name := ""
	if domain != nil && record.api.virDomainGetName != nil {
		name = copyCString(record.api.virDomainGetName(domain))
	}
	callback := record.lifecycle
	go invokeCallback(func() {
		callback(DomainLifecycleEvent{DomainName: name, Event: event, Detail: detail})
	})
	return 0
})

func invokeCallback(callback func()) {
	defer func() {
		_ = recover()
	}()
	callback()
}

func allocateCallbackRecord(api *nativeAPI, record *callbackRecord) (unsafe.Pointer, error) {
	opaque := api.malloc(1)
	if opaque == nil {
		return nil, fmt.Errorf("libvirt: allocate callback token")
	}
	record.api = api
	callbackRecords.Store(opaque, record)
	return opaque, nil
}

func discardCallbackRecord(api *nativeAPI, opaque unsafe.Pointer) {
	callbackRecords.Delete(opaque)
	api.free(opaque)
}

var (
	defaultEventOnce sync.Once
	defaultEventErr  error
)

// RegisterDefaultEventImpl installs libvirt's poll-based default event loop.
// Repeated calls return the result of the process-wide first registration.
func RegisterDefaultEventImpl() error {
	defaultEventOnce.Do(func() {
		api, err := getNativeAPI()
		if err != nil {
			defaultEventErr = err
			return
		}
		_, defaultEventErr = nativeCall(api, "virEventRegisterDefaultImpl", func() (int32, bool) {
			result := api.virEventRegisterDefaultImpl()
			return result, result < 0
		})
	})
	return defaultEventErr
}

// RunDefaultEventImpl runs one iteration of libvirt's default event loop.
func RunDefaultEventImpl() error {
	api, err := getNativeAPI()
	if err != nil {
		return err
	}
	_, err = nativeCall(api, "virEventRunDefaultImpl", func() (int32, bool) {
		result := api.virEventRunDefaultImpl()
		return result, result < 0
	})
	return err
}

// CloseCallback owns a registered connection-close callback.
type CloseCallback struct {
	mu       sync.Mutex
	conn     *Connect
	opaque   unsafe.Pointer
	callback uintptr
	closed   bool
}

// RegisterCloseCallback registers a callback for connection closure.
func (c *Connect) RegisterCloseCallback(callback func(reason int32)) (*CloseCallback, error) {
	if callback == nil {
		return nil, fmt.Errorf("libvirt: close callback is nil")
	}
	type registration struct {
		opaque unsafe.Pointer
	}
	registered, err := connectCall(c, "virConnectRegisterCloseCallback", func(api *nativeAPI, conn unsafe.Pointer) (registration, bool) {
		opaque, allocErr := allocateCallbackRecord(api, &callbackRecord{close: callback})
		if allocErr != nil {
			return registration{}, true
		}
		result := api.virConnectRegisterCloseCallback(conn, closeCallbackPointer, opaque, callbackFreePointer)
		if result < 0 {
			discardCallbackRecord(api, opaque)
			return registration{}, true
		}
		return registration{opaque: opaque}, false
	})
	if err != nil {
		return nil, err
	}
	return &CloseCallback{conn: c, opaque: registered.opaque, callback: closeCallbackPointer}, nil
}

// Close unregisters the connection-close callback. It is idempotent.
func (callback *CloseCallback) Close() error {
	if callback == nil {
		return nil
	}
	callback.mu.Lock()
	defer callback.mu.Unlock()
	if callback.closed {
		return nil
	}
	if _, active := callbackRecords.Load(callback.opaque); !active {
		callback.closed = true
		return nil
	}
	_, err := connectCall(callback.conn, "virConnectUnregisterCloseCallback", func(api *nativeAPI, conn unsafe.Pointer) (int32, bool) {
		result := api.virConnectUnregisterCloseCallback(conn, callback.callback)
		return result, result < 0
	})
	if err == nil {
		callback.closed = true
	}
	return err
}

// DomainEventCallback owns a registered domain event callback.
type DomainEventCallback struct {
	mu         sync.Mutex
	conn       *Connect
	opaque     unsafe.Pointer
	callbackID int32
	closed     bool
}

// RegisterDomainLifecycleCallback registers lifecycle events. A nil domain
// receives events for all domains on the connection.
func (c *Connect) RegisterDomainLifecycleCallback(domain *Domain, callback func(DomainLifecycleEvent)) (*DomainEventCallback, error) {
	if callback == nil {
		return nil, fmt.Errorf("libvirt: domain lifecycle callback is nil")
	}
	if c == nil {
		return nil, fmt.Errorf("%w: connection", ErrClosed)
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.ptr == nil {
		return nil, fmt.Errorf("%w: connection", ErrClosed)
	}
	var domainPtr unsafe.Pointer
	if domain != nil {
		domain.mu.RLock()
		defer domain.mu.RUnlock()
		if domain.ptr == nil {
			return nil, fmt.Errorf("%w: domain", ErrClosed)
		}
		domainPtr = domain.ptr
	}
	var opaque unsafe.Pointer
	callbackID, err := nativeCall(c.api, "virConnectDomainEventRegisterAny", func() (int32, bool) {
		var allocErr error
		opaque, allocErr = allocateCallbackRecord(c.api, &callbackRecord{lifecycle: callback})
		if allocErr != nil {
			return -1, true
		}
		result := c.api.virConnectDomainEventRegisterAny(c.ptr, domainPtr, int32(VIR_DOMAIN_EVENT_ID_LIFECYCLE), domainLifecycleCallbackPointer, opaque, callbackFreePointer)
		if result < 0 {
			discardCallbackRecord(c.api, opaque)
		}
		return result, result < 0
	})
	if err != nil {
		return nil, err
	}
	return &DomainEventCallback{conn: c, opaque: opaque, callbackID: callbackID}, nil
}

// Close unregisters the domain event callback. It is idempotent.
func (callback *DomainEventCallback) Close() error {
	if callback == nil {
		return nil
	}
	callback.mu.Lock()
	defer callback.mu.Unlock()
	if callback.closed {
		return nil
	}
	if _, active := callbackRecords.Load(callback.opaque); !active {
		callback.closed = true
		return nil
	}
	_, err := connectCall(callback.conn, "virConnectDomainEventDeregisterAny", func(api *nativeAPI, conn unsafe.Pointer) (int32, bool) {
		result := api.virConnectDomainEventDeregisterAny(conn, callback.callbackID)
		return result, result < 0
	})
	if err == nil {
		callback.closed = true
	}
	return err
}
