package libvirt

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

type nativeObject struct {
	mu   sync.RWMutex
	api  *nativeAPI
	ptr  unsafe.Pointer
	kind string
}

func newNativeObject(api *nativeAPI, ptr unsafe.Pointer, kind string) nativeObject {
	return nativeObject{api: api, ptr: ptr, kind: kind}
}

func connectCall[T any](conn *Connect, operation string, call func(*nativeAPI, unsafe.Pointer) (T, bool)) (T, error) {
	var zero T
	if conn == nil {
		return zero, fmt.Errorf("%w: connection", ErrClosed)
	}
	conn.mu.RLock()
	defer conn.mu.RUnlock()
	if conn.ptr == nil {
		return zero, fmt.Errorf("%w: connection", ErrClosed)
	}
	return nativeCall(conn.api, operation, func() (T, bool) {
		return call(conn.api, conn.ptr)
	})
}

func domainCall[T any](domain *Domain, operation string, call func(*nativeAPI, unsafe.Pointer) (T, bool)) (T, error) {
	var zero T
	if domain == nil {
		return zero, fmt.Errorf("%w: domain", ErrClosed)
	}
	domain.mu.RLock()
	defer domain.mu.RUnlock()
	if domain.ptr == nil {
		return zero, fmt.Errorf("%w: domain", ErrClosed)
	}
	return nativeCall(domain.api, operation, func() (T, bool) {
		return call(domain.api, domain.ptr)
	})
}

func objectCall[T any](object *nativeObject, operation string, call func(*nativeAPI, unsafe.Pointer) (T, bool)) (T, error) {
	var zero T
	if object == nil {
		return zero, fmt.Errorf("%w: native object", ErrClosed)
	}
	object.mu.RLock()
	defer object.mu.RUnlock()
	if object.ptr == nil {
		return zero, fmt.Errorf("%w: %s", ErrClosed, object.kind)
	}
	return nativeCall(object.api, operation, func() (T, bool) {
		return call(object.api, object.ptr)
	})
}

func objectFree(object *nativeObject, operation string, free func(*nativeAPI, unsafe.Pointer) int32) error {
	if object == nil {
		return fmt.Errorf("%w: native object", ErrClosed)
	}
	object.mu.Lock()
	defer object.mu.Unlock()
	if object.ptr == nil {
		return fmt.Errorf("%w: %s", ErrClosed, object.kind)
	}
	_, err := nativeCall(object.api, operation, func() (int32, bool) {
		result := free(object.api, object.ptr)
		return result, result < 0
	})
	if err == nil {
		object.ptr = nil
	}
	return err
}

func objectStatus(object *nativeObject, operation string, call func(*nativeAPI, unsafe.Pointer) int32) error {
	_, err := objectCall(object, operation, func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := call(api, ptr)
		return result, result < 0
	})
	return err
}

func objectBool(object *nativeObject, operation string, call func(*nativeAPI, unsafe.Pointer) int32) (bool, error) {
	result, err := objectCall(object, operation, func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		value := call(api, ptr)
		return value, value < 0
	})
	return result == 1, err
}

func objectBorrowedString(object *nativeObject, operation string, call func(*nativeAPI, unsafe.Pointer) unsafe.Pointer) (string, error) {
	return objectCall(object, operation, func(api *nativeAPI, ptr unsafe.Pointer) (string, bool) {
		value := call(api, ptr)
		return copyCString(value), value == nil
	})
}

func objectOwnedString(object *nativeObject, operation string, call func(*nativeAPI, unsafe.Pointer) unsafe.Pointer) (string, error) {
	return objectCall(object, operation, func(api *nativeAPI, ptr unsafe.Pointer) (string, bool) {
		value := call(api, ptr)
		if value == nil {
			return "", true
		}
		result := copyCString(value)
		api.free(value)
		return result, false
	})
}

func objectUUIDString(object *nativeObject, operation string, call func(*nativeAPI, unsafe.Pointer, *byte) int32) (string, error) {
	return objectCall(object, operation, func(api *nativeAPI, ptr unsafe.Pointer) (string, bool) {
		var buffer [uuidStringBufferLength]byte
		result := call(api, ptr, &buffer[0])
		runtime.KeepAlive(&buffer)
		return string(buffer[:uuidStringBufferLength-1]), result < 0
	})
}

func connectObjectFromString(conn *Connect, field, value, operation string, call func(*nativeAPI, unsafe.Pointer, *byte) unsafe.Pointer) (unsafe.Pointer, error) {
	buffer, valuePtr, err := makeCString(field, value, false)
	if err != nil {
		return nil, err
	}
	ptr, err := connectCall(conn, operation, func(api *nativeAPI, connPtr unsafe.Pointer) (unsafe.Pointer, bool) {
		result := call(api, connPtr, valuePtr)
		return result, result == nil
	})
	runtime.KeepAlive(buffer)
	return ptr, err
}

func objectFromString(object *nativeObject, field, value, operation string, call func(*nativeAPI, unsafe.Pointer, *byte) unsafe.Pointer) (unsafe.Pointer, error) {
	buffer, valuePtr, err := makeCString(field, value, false)
	if err != nil {
		return nil, err
	}
	ptr, err := objectCall(object, operation, func(api *nativeAPI, objectPtr unsafe.Pointer) (unsafe.Pointer, bool) {
		result := call(api, objectPtr, valuePtr)
		return result, result == nil
	})
	runtime.KeepAlive(buffer)
	return ptr, err
}

func connectObjectFromXML(conn *Connect, xml, operation string, flags uint32, call func(*nativeAPI, unsafe.Pointer, *byte, uint32) unsafe.Pointer) (unsafe.Pointer, error) {
	buffer, xmlPtr, err := makeCString("XML", xml, false)
	if err != nil {
		return nil, err
	}
	ptr, err := connectCall(conn, operation, func(api *nativeAPI, connPtr unsafe.Pointer) (unsafe.Pointer, bool) {
		result := call(api, connPtr, xmlPtr, flags)
		return result, result == nil
	})
	runtime.KeepAlive(buffer)
	return ptr, err
}

func objectFromXML(object *nativeObject, xml, operation string, flags uint32, call func(*nativeAPI, unsafe.Pointer, *byte, uint32) unsafe.Pointer) (unsafe.Pointer, error) {
	buffer, xmlPtr, err := makeCString("XML", xml, false)
	if err != nil {
		return nil, err
	}
	ptr, err := objectCall(object, operation, func(api *nativeAPI, objectPtr unsafe.Pointer) (unsafe.Pointer, bool) {
		result := call(api, objectPtr, xmlPtr, flags)
		return result, result == nil
	})
	runtime.KeepAlive(buffer)
	return ptr, err
}

func connectListObjects(conn *Connect, operation string, flags uint32, call func(*nativeAPI, unsafe.Pointer, *unsafe.Pointer, uint32) int32) ([]unsafe.Pointer, error) {
	return connectCall(conn, operation, func(api *nativeAPI, connPtr unsafe.Pointer) ([]unsafe.Pointer, bool) {
		return nativeObjectList(api, connPtr, flags, call)
	})
}

func objectListObjects(object *nativeObject, operation string, flags uint32, call func(*nativeAPI, unsafe.Pointer, *unsafe.Pointer, uint32) int32) ([]unsafe.Pointer, error) {
	return objectCall(object, operation, func(api *nativeAPI, objectPtr unsafe.Pointer) ([]unsafe.Pointer, bool) {
		return nativeObjectList(api, objectPtr, flags, call)
	})
}

func domainListObjects(domain *Domain, operation string, flags uint32, call func(*nativeAPI, unsafe.Pointer, *unsafe.Pointer, uint32) int32) ([]unsafe.Pointer, error) {
	return domainCall(domain, operation, func(api *nativeAPI, domainPtr unsafe.Pointer) ([]unsafe.Pointer, bool) {
		return nativeObjectList(api, domainPtr, flags, call)
	})
}

func nativeObjectList(api *nativeAPI, owner unsafe.Pointer, flags uint32, call func(*nativeAPI, unsafe.Pointer, *unsafe.Pointer, uint32) int32) ([]unsafe.Pointer, bool) {
	var list unsafe.Pointer
	count := call(api, owner, &list, flags)
	if count < 0 {
		return nil, true
	}
	if list != nil {
		defer api.free(list)
	}
	if count == 0 {
		return []unsafe.Pointer{}, false
	}
	if list == nil {
		return nil, true
	}
	handles := unsafe.Slice((*unsafe.Pointer)(list), int(count))
	return append([]unsafe.Pointer(nil), handles...), false
}
