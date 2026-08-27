package libvirt

// RawAPI exposes the generated libvirt C ABI with purego-compatible types.
//
// Raw methods preserve libvirt's C return values and ownership rules; their
// error result only reports that the loaded library does not export the
// function. Callers remain responsible for checking C failure sentinels,
// releasing native allocations and references, and locking the goroutine to an
// OS thread when a failing call must be paired with VirGetLastError.
type RawAPI struct {
	api *nativeAPI
}

// NewRawAPI returns the process-wide generated low-level API.
func NewRawAPI() (*RawAPI, error) {
	api, err := getNativeAPI()
	if err != nil {
		return nil, err
	}
	return &RawAPI{api: api}, nil
}

// HasSymbol reports whether this loaded libvirt exports a generated function.
func (raw *RawAPI) HasSymbol(name string) bool {
	return raw != nil && raw.api != nil && raw.api.hasSymbol(name)
}

func (raw *RawAPI) requireSymbol(name string) error {
	if raw == nil || raw.api == nil {
		return ErrClosed
	}
	return raw.api.requireSymbol(name)
}
