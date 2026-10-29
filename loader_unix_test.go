//go:build linux || darwin || freebsd || netbsd

package libvirt

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadedLibraryPreservesMissingMainSymbols(t *testing.T) {
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
