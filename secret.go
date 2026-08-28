package libvirt

import (
	"runtime"
	"unsafe"
)

// Secret is a reference-counted libvirt secret handle.
type Secret struct {
	object nativeObject
}

func secretObject(secret *Secret) *nativeObject {
	if secret == nil {
		return nil
	}
	return &secret.object
}

func newSecret(api *nativeAPI, ptr unsafe.Pointer) *Secret {
	return &Secret{object: newNativeObject(api, ptr, "secret")}
}

// ListAllSecrets returns secrets matching flags. Each handle must be freed.
func (c *Connect) ListAllSecrets(flags uint32) ([]*Secret, error) {
	handles, err := connectListObjects(c, "virConnectListAllSecrets", flags, func(api *nativeAPI, conn unsafe.Pointer, list *unsafe.Pointer, flags uint32) int32 {
		return api.virConnectListAllSecrets(conn, list, flags)
	})
	if err != nil {
		return nil, err
	}
	secrets := make([]*Secret, len(handles))
	for i, handle := range handles {
		secrets[i] = newSecret(c.api, handle)
	}
	return secrets, nil
}

// LookupSecretByUUIDString returns a referenced secret.
func (c *Connect) LookupSecretByUUIDString(uuid string) (*Secret, error) {
	ptr, err := connectObjectFromString(c, "secret UUID", uuid, "virSecretLookupByUUIDString", func(api *nativeAPI, conn unsafe.Pointer, uuid *byte) unsafe.Pointer {
		return api.virSecretLookupByUUIDString(conn, uuid)
	})
	if err != nil {
		return nil, err
	}
	return newSecret(c.api, ptr), nil
}

// LookupSecretByUsage returns a referenced secret for a usage type and ID.
func (c *Connect) LookupSecretByUsage(usageType int32, usageID string) (*Secret, error) {
	buffer, usagePtr, err := makeCString("secret usage ID", usageID, false)
	if err != nil {
		return nil, err
	}
	ptr, err := connectCall(c, "virSecretLookupByUsage", func(api *nativeAPI, conn unsafe.Pointer) (unsafe.Pointer, bool) {
		result := api.virSecretLookupByUsage(conn, usageType, usagePtr)
		return result, result == nil
	})
	runtime.KeepAlive(buffer)
	if err != nil {
		return nil, err
	}
	return newSecret(c.api, ptr), nil
}

// DefineSecretXML defines or updates a secret.
func (c *Connect) DefineSecretXML(xml string, flags uint32) (*Secret, error) {
	ptr, err := connectObjectFromXML(c, xml, "virSecretDefineXML", flags, func(api *nativeAPI, conn unsafe.Pointer, xml *byte, flags uint32) unsafe.Pointer {
		return api.virSecretDefineXML(conn, xml, flags)
	})
	if err != nil {
		return nil, err
	}
	return newSecret(c.api, ptr), nil
}

// Free releases this wrapper's secret reference.
func (secret *Secret) Free() error {
	return objectFree(secretObject(secret), "virSecretFree", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virSecretFree(ptr)
	})
}

// GetUUIDString returns the canonical secret UUID.
func (secret *Secret) GetUUIDString() (string, error) {
	return objectUUIDString(secretObject(secret), "virSecretGetUUIDString", func(api *nativeAPI, ptr unsafe.Pointer, buffer *byte) int32 {
		return api.virSecretGetUUIDString(ptr, buffer)
	})
}

// GetXMLDesc returns secret metadata XML. It does not return the secret value.
func (secret *Secret) GetXMLDesc(flags uint32) (string, error) {
	return objectOwnedString(secretObject(secret), "virSecretGetXMLDesc", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virSecretGetXMLDesc(ptr, flags)
	})
}

// GetValue copies the secret value into Go memory.
func (secret *Secret) GetValue(flags uint32) ([]byte, error) {
	return objectCall(secretObject(secret), "virSecretGetValue", func(api *nativeAPI, ptr unsafe.Pointer) ([]byte, bool) {
		var size uintptr
		value := api.virSecretGetValue(ptr, &size, flags)
		if value == nil {
			return nil, true
		}
		defer api.free(value)
		if size > uintptr(int(^uint(0)>>1)) {
			return nil, true
		}
		return append([]byte(nil), unsafe.Slice((*byte)(value), int(size))...), false
	})
}

// SetValue replaces the secret value.
func (secret *Secret) SetValue(value []byte, flags uint32) error {
	var valuePtr *byte
	if len(value) != 0 {
		valuePtr = &value[0]
	}
	_, err := objectCall(secretObject(secret), "virSecretSetValue", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := api.virSecretSetValue(ptr, valuePtr, uintptr(len(value)), flags)
		return result, result < 0
	})
	runtime.KeepAlive(value)
	return err
}

// Undefine removes the secret metadata and value.
func (secret *Secret) Undefine() error {
	return objectStatus(secretObject(secret), "virSecretUndefine", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virSecretUndefine(ptr)
	})
}
