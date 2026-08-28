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
	handle           uintptr
	path             string
	malloc           func(uintptr) unsafe.Pointer
	calloc           func(uintptr, uintptr) unsafe.Pointer
	free             func(unsafe.Pointer)
	missing          map[string]string
	extensionHandles map[string]uintptr

	generatedNativeAPI
}

type nativeSymbolBinding struct {
	name    string
	since   string
	library string
	target  any
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

// HasSymbol reports whether the loaded libvirt exports a generated API symbol.
func HasSymbol(name string) (bool, error) {
	api, err := getNativeAPI()
	if err != nil {
		return false, err
	}
	return api.hasSymbol(name), nil
}

// SymbolVersion returns the libvirt version that introduced a generated symbol.
func SymbolVersion(name string) (string, bool) {
	version, ok := generatedLibvirtSymbolVersions[name]
	return version, ok
}

func (api *nativeAPI) hasSymbol(name string) bool {
	if _, known := generatedLibvirtSymbolVersions[name]; !known {
		return false
	}
	_, missing := api.missing[name]
	return !missing
}

func (api *nativeAPI) requireSymbol(name string) error {
	if version, missing := api.missing[name]; missing {
		return &SymbolUnavailableError{Symbol: name, Since: version}
	}
	return nil
}

func bindLibvirtSymbols(api *nativeAPI, bind func(nativeSymbolBinding) error) {
	if api.missing == nil {
		api.missing = make(map[string]string)
	}
	for _, binding := range libvirtSymbolBindings(api) {
		if err := bind(binding); err != nil {
			api.missing[binding.name] = binding.since
		} else {
			delete(api.missing, binding.name)
		}
	}
}

// nativeCall keeps the failed API call and virGetLastError on the same OS
// thread because libvirt stores errors in thread-local storage.
func nativeCall[T any](api *nativeAPI, operation string, call func() (T, bool)) (T, error) {
	if err := api.requireSymbol(operation); err != nil {
		var zero T
		return zero, err
	}

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
