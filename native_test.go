package libvirt

import (
	"errors"
	"runtime"
	"testing"
	"unsafe"
)

func TestCStringConversion(t *testing.T) {
	buf, ptr, err := makeCString("name", "guest", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := copyCString(unsafe.Pointer(ptr)); got != "guest" {
		t.Fatalf("copyCString() = %q, want %q", got, "guest")
	}
	runtime.KeepAlive(buf)

	buf, ptr, err = makeCString("URI", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if buf != nil || ptr != nil {
		t.Fatalf("optional empty C string = (%v, %p), want (nil, nil)", buf, ptr)
	}

	_, _, err = makeCString("name", "bad\x00name", false)
	if !errors.Is(err, ErrEmbeddedNUL) {
		t.Fatalf("embedded NUL error = %v, want ErrEmbeddedNUL", err)
	}
}

func TestNativeCallCopiesError(t *testing.T) {
	message := append([]byte("lookup failed"), 0)
	record := cError{
		code:    42,
		domain:  10,
		message: unsafe.Pointer(&message[0]),
		level:   2,
	}
	api := &nativeAPI{
		generatedNativeAPI: generatedNativeAPI{
			virResetLastError: func() {},
			virGetLastError: func() unsafe.Pointer {
				return unsafe.Pointer(&record)
			},
		},
	}

	_, err := nativeCall(api, "virDomainLookupByName", func() (int32, bool) {
		return -1, true
	})
	runtime.KeepAlive(message)
	var libvirtErr *Error
	if !errors.As(err, &libvirtErr) {
		t.Fatalf("nativeCall() error = %T, want *Error", err)
	}
	if libvirtErr.Operation != "virDomainLookupByName" || libvirtErr.Code != 42 || libvirtErr.Domain != 10 || libvirtErr.Level != 2 || libvirtErr.Message != "lookup failed" {
		t.Fatalf("nativeCall() copied error = %#v", libvirtErr)
	}
}

func TestConnectCloseConsumesReferenceOnce(t *testing.T) {
	calls := 0
	handleValue := byte(1)
	handle := unsafe.Pointer(&handleValue)
	api := &nativeAPI{
		generatedNativeAPI: generatedNativeAPI{
			virResetLastError: func() {},
			virConnectClose: func(ptr unsafe.Pointer) int32 {
				calls++
				if ptr != handle {
					t.Fatalf("virConnectClose pointer = %p, want %p", ptr, handle)
				}
				return 0
			},
		},
	}
	conn := &Connect{api: api, ptr: handle}

	remaining, err := conn.Close()
	if err != nil {
		t.Fatal(err)
	}
	if remaining != 0 || calls != 1 {
		t.Fatalf("Close() = (%d, %v), calls = %d", remaining, err, calls)
	}
	if _, err := conn.Close(); !errors.Is(err, ErrClosed) {
		t.Fatalf("second Close() error = %v, want ErrClosed", err)
	}
	if calls != 1 {
		t.Fatalf("virConnectClose called %d times, want 1", calls)
	}
}
