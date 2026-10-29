//go:build linux || darwin || freebsd || netbsd

package libvirt

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"
)

func TestLoadedLibraryContracts(t *testing.T) {
	compiler, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("C compiler required for the shared-library fixture")
	}
	library := filepath.Join(t.TempDir(), "libvirt-missing.so")
	args := []string{"-shared", "-fPIC"}
	if runtime.GOOS == "darwin" {
		args = []string{"-dynamiclib"}
	}
	args = append(args, "testdata/missing_symbols.c", "-o", library)
	if output, err := exec.Command(compiler, args...).CombinedOutput(); err != nil {
		t.Fatalf("compile shared library: %v\n%s", err, output)
	}
	t.Setenv("LIBVIRT_ADMIN_LIBRARY", filepath.Join(t.TempDir(), "missing-admin"))
	t.Setenv("LIBVIRT_QEMU_LIBRARY", filepath.Join(t.TempDir(), "missing-qemu"))
	t.Setenv("LIBVIRT_LXC_LIBRARY", filepath.Join(t.TempDir(), "missing-lxc"))
	api, err := loadNativeAPIFrom(library)
	if err != nil {
		t.Fatal(err)
	}
	raw := &RawAPI{api: api}
	if !raw.HasSymbol("virInitialize") {
		t.Fatal("fixture initialization symbol is unavailable")
	}
	if raw.HasSymbol("virConnectOpen") {
		t.Fatal("missing main-library symbol reported as available")
	}
	if _, err := raw.VirConnectOpen(nil); !errors.Is(err, ErrSymbolUnavailable) {
		t.Fatalf("missing main-library call = %v, want ErrSymbolUnavailable", err)
	}
	handle := byte(1)
	domain := newDomain(api, unsafe.Pointer(&handle))
	params, err := domain.GetSchedulerParameters(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 1 || params[0].Field != "cpu_shares" || params[0].Value != uint64(1024) {
		t.Fatalf("scheduler parameters = %#v, want cpu_shares=1024", params)
	}
}

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
