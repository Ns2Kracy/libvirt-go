//go:build linux || darwin || freebsd || netbsd

package libvirt

import "testing"

func TestInitializeNativeAPIDoesNotResetErrorsFirst(t *testing.T) {
	resetCalls := 0
	initializeCalls := 0
	api := &nativeAPI{
		generatedNativeAPI: generatedNativeAPI{
			virInitialize: func() int32 {
				initializeCalls++
				return 0
			},
			virResetLastError: func() {
				resetCalls++
			},
		},
	}
	if err := initializeNativeAPI(api); err != nil {
		t.Fatal(err)
	}
	if initializeCalls != 1 {
		t.Fatalf("virInitialize called %d times, want 1", initializeCalls)
	}
	if resetCalls != 0 {
		t.Fatalf("virResetLastError called %d times before initialization", resetCalls)
	}
}
