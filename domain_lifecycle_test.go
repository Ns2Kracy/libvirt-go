package libvirt

import (
	"errors"
	"runtime"
	"testing"
	"unsafe"
)

func TestDomainGetIDUnsignedSentinel(t *testing.T) {
	for _, test := range []struct {
		name   string
		id     uint32
		failed bool
	}{
		{name: "zero is a valid native ID", id: 0},
		{name: "active domain", id: 42},
		{name: "unsigned failure sentinel", id: ^uint32(0), failed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			message := append([]byte("domain ID is unavailable"), 0)
			record := cError{code: int32(ERR_OPERATION_INVALID), domain: 10, message: unsafe.Pointer(&message[0]), level: 2}
			api := &nativeAPI{generatedNativeAPI: generatedNativeAPI{
				virResetLastError: func() {},
				virGetLastError:   func() unsafe.Pointer { return unsafe.Pointer(&record) },
				virDomainGetID:    func(unsafe.Pointer) uint32 { return test.id },
			}}
			domain := newDomain(api, unsafe.Pointer(new(byte(1))))
			id, err := domain.GetID()
			runtime.KeepAlive(message)
			if id != uint(test.id) {
				t.Fatalf("GetID = %d, want %d", id, test.id)
			}
			if test.failed {
				nativeError, ok := errors.AsType[Error](err)
				if !ok || nativeError.Code != ERR_OPERATION_INVALID || nativeError.Operation != "virDomainGetID" {
					t.Fatalf("GetID failure = %#v", err)
				}
			} else if err != nil {
				t.Fatalf("GetID: %v", err)
			}
		})
	}
}
