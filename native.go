package libvirt

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"unsafe"
)

// nativeAPI combines process-local loader state with generated libvirt ABI fields.
type nativeAPI struct {
	handle uintptr
	path   string
	free   func(unsafe.Pointer)

	generatedNativeAPI
}

type nativeSymbolBinding struct {
	name   string
	target any
}

var (
	apiOnce sync.Once
	api     *nativeAPI
	apiErr  error
)

func getNativeAPI() (*nativeAPI, error) {
	apiOnce.Do(func() {
		api, apiErr = loadNativeAPI()
	})
	return api, apiErr
}

// nativeCall keeps the failed API call and virGetLastError on the same OS
// thread because libvirt stores errors in thread-local storage.
func nativeCall[T any](api *nativeAPI, operation string, call func() (T, bool)) (T, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if api.virResetLastError != nil {
		api.virResetLastError()
	}
	value, failed := call()
	if !failed {
		return value, nil
	}
	return value, api.lastError(operation)
}

func (api *nativeAPI) lastError(operation string) error {
	if api.virGetLastError == nil {
		return &Error{Operation: operation}
	}
	ptr := api.virGetLastError()
	if ptr == nil {
		return &Error{Operation: operation}
	}

	record := (*cError)(ptr)
	return &Error{
		Operation: operation,
		Code:      record.code,
		Domain:    record.domain,
		Level:     record.level,
		Message:   copyCString(record.message),
	}
}

// cError is the stable prefix of struct _virError from virterror.h.
type cError struct {
	code    int32
	domain  int32
	message unsafe.Pointer
	level   int32
}

func makeCString(field, value string, optional bool) ([]byte, *byte, error) {
	if strings.IndexByte(value, 0) >= 0 {
		return nil, nil, fmt.Errorf("libvirt: %s: %w", field, ErrEmbeddedNUL)
	}
	if optional && value == "" {
		return nil, nil, nil
	}
	buf := append([]byte(value), 0)
	return buf, &buf[0], nil
}

func copyCString(ptr unsafe.Pointer) string {
	if ptr == nil {
		return ""
	}
	length := 0
	for *(*byte)(unsafe.Add(ptr, length)) != 0 {
		length++
	}
	return string(unsafe.Slice((*byte)(ptr), length))
}
