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

func loadNativeAPIFrom(path string) (*nativeAPI, error) {
	handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}

	api := &nativeAPI{handle: handle, path: path}
	if err := registerSymbol(purego.RTLD_DEFAULT, "free", &api.free); err != nil {
		_ = purego.Dlclose(handle)
		return nil, err
	}
	// The generated table contains exactly the vir* selectors used by wrappers.
	for _, binding := range libvirtSymbolBindings(api) {
		if err := registerSymbol(handle, binding.name, binding.target); err != nil {
			_ = purego.Dlclose(handle)
			return nil, err
		}
	}

	if _, err := nativeCall(api, "virInitialize", func() (int32, bool) {
		result := api.virInitialize()
		return result, result < 0
	}); err != nil {
		_ = purego.Dlclose(handle)
		return nil, err
	}

	// Registered function values remain valid only while the library is loaded,
	// so the successful handle intentionally lives for the process lifetime.
	return api, nil
}

func registerSymbol(handle uintptr, name string, target any) error {
	symbol, err := purego.Dlsym(handle, name)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", name, err)
	}
	purego.RegisterFunc(target, symbol)
	return nil
}
