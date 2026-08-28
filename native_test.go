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

func TestMissingGeneratedSymbolIsCompatible(t *testing.T) {
	api := &nativeAPI{}
	bindLibvirtSymbols(api, func(binding nativeSymbolBinding) error {
		if binding.name == "virConnectListAllDomains" {
			return errors.New("not exported by old libvirt")
		}
		return nil
	})

	called := false
	_, err := nativeCall(api, "virConnectListAllDomains", func() (int32, bool) {
		called = true
		return 0, false
	})
	if called {
		t.Fatal("nativeCall invoked a function missing from the loaded libvirt")
	}
	if !errors.Is(err, ErrSymbolUnavailable) {
		t.Fatalf("nativeCall error = %v, want ErrSymbolUnavailable", err)
	}
	var unavailable *SymbolUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("nativeCall error = %T, want *SymbolUnavailableError", err)
	}
	if unavailable.Symbol != "virConnectListAllDomains" || unavailable.Since != "0.9.13" {
		t.Fatalf("SymbolUnavailableError = %#v", unavailable)
	}
	if api.hasSymbol("virConnectListAllDomains") {
		t.Fatal("hasSymbol reported a missing old-libvirt function")
	}
	if !api.hasSymbol("virConnectOpen") {
		t.Fatal("hasSymbol rejected an available baseline function")
	}
}

func TestGeneratedAPICatalogComplete(t *testing.T) {
	bindings := libvirtSymbolBindings(&nativeAPI{})
	if len(bindings) < 560 {
		t.Fatalf("generated only %d libvirt functions, want main/admin/LXC/QEMU APIs", len(bindings))
	}
	if len(bindings) != len(generatedLibvirtSymbolVersions) {
		t.Fatalf("binding/version metadata counts differ: %d != %d", len(bindings), len(generatedLibvirtSymbolVersions))
	}
	seen := make(map[string]struct{}, len(bindings))
	libraries := make(map[string]int)
	for _, binding := range bindings {
		if binding.name == "" || binding.since == "" || binding.library == "" || binding.target == nil {
			t.Fatalf("incomplete generated binding: %#v", binding)
		}
		if _, duplicate := seen[binding.name]; duplicate {
			t.Fatalf("duplicate generated binding %s", binding.name)
		}
		seen[binding.name] = struct{}{}
		libraries[binding.library]++
	}
	for _, library := range []string{"main", "admin", "lxc", "qemu"} {
		if libraries[library] == 0 {
			t.Errorf("generated no functions for %s API", library)
		}
	}
}

func TestRawAPIMethodGuardsMissingSymbol(t *testing.T) {
	api := &nativeAPI{missing: map[string]string{"virConnectListAllDomains": "0.9.13"}}
	raw := &RawAPI{api: api}
	if _, err := raw.VirConnectListAllDomains(nil, nil, 0); !errors.Is(err, ErrSymbolUnavailable) {
		t.Fatalf("VirConnectListAllDomains error = %v, want ErrSymbolUnavailable", err)
	}
	if raw.HasSymbol("virConnectListAllDomains") {
		t.Fatal("RawAPI.HasSymbol reported a missing function")
	}
}
