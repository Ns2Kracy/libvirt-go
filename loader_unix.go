//go:build linux || darwin || freebsd || netbsd

package libvirt

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/ebitengine/purego"
)

func loadNativeAPI() (*nativeAPI, error) {
	candidates := libvirtLibraryCandidates()
	attempts := make([]error, 0, len(candidates))
	for _, path := range candidates {
		api, err := loadNativeAPIFrom(path)
		if err == nil {
			return api, nil
		}
		attempts = append(attempts, fmt.Errorf("%s: %w", path, err))
	}
	return nil, fmt.Errorf("%w (tried %s): %w", ErrLibraryNotFound, strings.Join(candidates, ", "), errors.Join(attempts...))
}

func libvirtLibraryCandidates() []string {
	if path := os.Getenv("LIBVIRT_GO_LIBRARY"); path != "" {
		return []string{path}
	}
	if runtime.GOOS == "darwin" {
		return []string{"libvirt.0.dylib", "libvirt.dylib"}
	}
	return []string{"libvirt.so.0", "libvirt.so"}
}

func loadExtensionLibraries() map[string]uintptr {
	handles := make(map[string]uintptr, 3)
	for _, library := range []string{"admin", "lxc", "qemu"} {
		for _, path := range extensionLibraryCandidates(library) {
			handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
			if err == nil {
				handles[library] = handle
				break
			}
		}
	}
	return handles
}

func extensionLibraryCandidates(library string) []string {
	environment := "LIBVIRT_" + strings.ToUpper(library) + "_LIBRARY"
	if path := os.Getenv(environment); path != "" {
		return []string{path}
	}
	if runtime.GOOS == "darwin" {
		return []string{"libvirt-" + library + ".0.dylib", "libvirt-" + library + ".dylib"}
	}
	return []string{"libvirt-" + library + ".so.0", "libvirt-" + library + ".so"}
}

func loadNativeAPIFrom(path string) (*nativeAPI, error) {
	handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}

	api := &nativeAPI{
		handle:  handle,
		path:    path,
		missing: make(map[string]string),
	}
	for _, allocator := range []nativeSymbolBinding{
		{name: "malloc", target: &api.malloc},
		{name: "calloc", target: &api.calloc},
		{name: "free", target: &api.free},
	} {
		if err := registerSymbol(purego.RTLD_DEFAULT, allocator.name, allocator.target); err != nil {
			_ = purego.Dlclose(handle)
			return nil, err
		}
	}
	// Initialize the main library before dlopen of admin, QEMU, or LXC. Older
	// libvirt releases can corrupt allocator state if extensions load first.
	bindLibvirtSymbols(api, func(binding nativeSymbolBinding) error {
		if binding.library != "main" {
			return fmt.Errorf("%s extension library is not loaded yet", binding.library)
		}
		return registerSymbol(handle, binding.name, binding.target)
	})

	if err := initializeNativeAPI(api); err != nil {
		_ = purego.Dlclose(handle)
		return nil, err
	}

	api.extensionHandles = loadExtensionLibraries()
	bindLibvirtSymbols(api, func(binding nativeSymbolBinding) error {
		if binding.library == "main" {
			return nil
		}
		libraryHandle := api.extensionHandles[binding.library]
		if libraryHandle == 0 {
			return fmt.Errorf("%s extension library is unavailable", binding.library)
		}
		return registerSymbol(libraryHandle, binding.name, binding.target)
	})

	// Registered function values remain valid only while the library is loaded,
	// so the successful handle intentionally lives for the process lifetime.
	return api, nil
}

func initializeNativeAPI(api *nativeAPI) error {
	if err := api.requireSymbol("virInitialize"); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if result := api.virInitialize(); result < 0 {
		return api.lastError("virInitialize")
	}
	return nil
}

func registerSymbol(handle uintptr, name string, target any) error {
	symbol, err := purego.Dlsym(handle, name)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", name, err)
	}
	purego.RegisterFunc(target, symbol)
	return nil
}
