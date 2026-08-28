package libvirt

import "unsafe"

// StoragePool is a reference-counted libvirt storage pool handle.
type StoragePool struct {
	object nativeObject
}

func storagePoolObject(pool *StoragePool) *nativeObject {
	if pool == nil {
		return nil
	}
	return &pool.object
}

func newStoragePool(api *nativeAPI, ptr unsafe.Pointer) *StoragePool {
	return &StoragePool{object: newNativeObject(api, ptr, "storage pool")}
}

// ListAllStoragePools returns storage pools matching flags. Each handle must be freed.
func (c *Connect) ListAllStoragePools(flags uint32) ([]*StoragePool, error) {
	handles, err := connectListObjects(c, "virConnectListAllStoragePools", flags, func(api *nativeAPI, conn unsafe.Pointer, list *unsafe.Pointer, flags uint32) int32 {
		return api.virConnectListAllStoragePools(conn, list, flags)
	})
	if err != nil {
		return nil, err
	}
	pools := make([]*StoragePool, len(handles))
	for i, handle := range handles {
		pools[i] = newStoragePool(c.api, handle)
	}
	return pools, nil
}

// LookupStoragePoolByName returns a referenced storage pool.
func (c *Connect) LookupStoragePoolByName(name string) (*StoragePool, error) {
	ptr, err := connectObjectFromString(c, "storage pool name", name, "virStoragePoolLookupByName", func(api *nativeAPI, conn unsafe.Pointer, name *byte) unsafe.Pointer {
		return api.virStoragePoolLookupByName(conn, name)
	})
	if err != nil {
		return nil, err
	}
	return newStoragePool(c.api, ptr), nil
}

// LookupStoragePoolByUUIDString returns a referenced storage pool.
func (c *Connect) LookupStoragePoolByUUIDString(uuid string) (*StoragePool, error) {
	ptr, err := connectObjectFromString(c, "storage pool UUID", uuid, "virStoragePoolLookupByUUIDString", func(api *nativeAPI, conn unsafe.Pointer, uuid *byte) unsafe.Pointer {
		return api.virStoragePoolLookupByUUIDString(conn, uuid)
	})
	if err != nil {
		return nil, err
	}
	return newStoragePool(c.api, ptr), nil
}

// DefineStoragePoolXML defines a persistent storage pool.
func (c *Connect) DefineStoragePoolXML(xml string, flags uint32) (*StoragePool, error) {
	ptr, err := connectObjectFromXML(c, xml, "virStoragePoolDefineXML", flags, func(api *nativeAPI, conn unsafe.Pointer, xml *byte, flags uint32) unsafe.Pointer {
		return api.virStoragePoolDefineXML(conn, xml, flags)
	})
	if err != nil {
		return nil, err
	}
	return newStoragePool(c.api, ptr), nil
}

// CreateStoragePoolXML creates a transient storage pool.
func (c *Connect) CreateStoragePoolXML(xml string, flags uint32) (*StoragePool, error) {
	ptr, err := connectObjectFromXML(c, xml, "virStoragePoolCreateXML", flags, func(api *nativeAPI, conn unsafe.Pointer, xml *byte, flags uint32) unsafe.Pointer {
		return api.virStoragePoolCreateXML(conn, xml, flags)
	})
	if err != nil {
		return nil, err
	}
	return newStoragePool(c.api, ptr), nil
}

// Free releases this wrapper's storage-pool reference.
func (pool *StoragePool) Free() error {
	return objectFree(storagePoolObject(pool), "virStoragePoolFree", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStoragePoolFree(ptr)
	})
}

// GetName returns the pool name.
func (pool *StoragePool) GetName() (string, error) {
	return objectBorrowedString(storagePoolObject(pool), "virStoragePoolGetName", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virStoragePoolGetName(ptr)
	})
}

// GetUUIDString returns the canonical pool UUID.
func (pool *StoragePool) GetUUIDString() (string, error) {
	return objectUUIDString(storagePoolObject(pool), "virStoragePoolGetUUIDString", func(api *nativeAPI, ptr unsafe.Pointer, buffer *byte) int32 {
		return api.virStoragePoolGetUUIDString(ptr, buffer)
	})
}

// GetXMLDesc returns the pool XML.
func (pool *StoragePool) GetXMLDesc(flags uint32) (string, error) {
	return objectOwnedString(storagePoolObject(pool), "virStoragePoolGetXMLDesc", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virStoragePoolGetXMLDesc(ptr, flags)
	})
}

// IsActive reports whether the storage pool is active.
func (pool *StoragePool) IsActive() (bool, error) {
	return objectBool(storagePoolObject(pool), "virStoragePoolIsActive", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStoragePoolIsActive(ptr)
	})
}

// IsPersistent reports whether the pool has persistent configuration.
func (pool *StoragePool) IsPersistent() (bool, error) {
	return objectBool(storagePoolObject(pool), "virStoragePoolIsPersistent", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStoragePoolIsPersistent(ptr)
	})
}

// GetAutostart reports whether the storage pool starts automatically.
func (pool *StoragePool) GetAutostart() (bool, error) {
	value, err := objectCall(storagePoolObject(pool), "virStoragePoolGetAutostart", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		var autostart int32
		result := api.virStoragePoolGetAutostart(ptr, &autostart)
		return autostart, result < 0
	})
	return value == 1, err
}

// SetAutostart changes whether the storage pool starts automatically.
func (pool *StoragePool) SetAutostart(autostart bool) error {
	value := int32(0)
	if autostart {
		value = 1
	}
	_, err := objectCall(storagePoolObject(pool), "virStoragePoolSetAutostart", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := api.virStoragePoolSetAutostart(ptr, value)
		return result, result < 0
	})
	return err
}

// Create starts an inactive storage pool.
func (pool *StoragePool) Create(flags uint32) error {
	return objectStatus(storagePoolObject(pool), "virStoragePoolCreate", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStoragePoolCreate(ptr, flags)
	})
}

// Destroy stops an active storage pool.
func (pool *StoragePool) Destroy() error {
	return objectStatus(storagePoolObject(pool), "virStoragePoolDestroy", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStoragePoolDestroy(ptr)
	})
}

// Undefine removes a persistent storage-pool definition.
func (pool *StoragePool) Undefine() error {
	return objectStatus(storagePoolObject(pool), "virStoragePoolUndefine", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStoragePoolUndefine(ptr)
	})
}

// Refresh refreshes the storage pool's volume list.
func (pool *StoragePool) Refresh(flags uint32) error {
	return objectStatus(storagePoolObject(pool), "virStoragePoolRefresh", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStoragePoolRefresh(ptr, flags)
	})
}

// ListAllVolumes returns volumes in this pool. Each handle must be freed.
func (pool *StoragePool) ListAllVolumes(flags uint32) ([]*StorageVol, error) {
	handles, err := objectListObjects(storagePoolObject(pool), "virStoragePoolListAllVolumes", flags, func(api *nativeAPI, ptr unsafe.Pointer, list *unsafe.Pointer, flags uint32) int32 {
		return api.virStoragePoolListAllVolumes(ptr, list, flags)
	})
	if err != nil {
		return nil, err
	}
	volumes := make([]*StorageVol, len(handles))
	for i, handle := range handles {
		volumes[i] = newStorageVol(pool.object.api, handle)
	}
	return volumes, nil
}

// LookupVolumeByName returns a referenced volume in this pool.
func (pool *StoragePool) LookupVolumeByName(name string) (*StorageVol, error) {
	ptr, err := objectFromString(storagePoolObject(pool), "storage volume name", name, "virStorageVolLookupByName", func(api *nativeAPI, pool unsafe.Pointer, name *byte) unsafe.Pointer {
		return api.virStorageVolLookupByName(pool, name)
	})
	if err != nil {
		return nil, err
	}
	return newStorageVol(pool.object.api, ptr), nil
}

// CreateVolumeXML creates a storage volume.
func (pool *StoragePool) CreateVolumeXML(xml string, flags uint32) (*StorageVol, error) {
	ptr, err := objectFromXML(storagePoolObject(pool), xml, "virStorageVolCreateXML", flags, func(api *nativeAPI, pool unsafe.Pointer, xml *byte, flags uint32) unsafe.Pointer {
		return api.virStorageVolCreateXML(pool, xml, flags)
	})
	if err != nil {
		return nil, err
	}
	return newStorageVol(pool.object.api, ptr), nil
}

// StorageVol is a reference-counted libvirt storage volume handle.
type StorageVol struct {
	object nativeObject
}

func storageVolObject(volume *StorageVol) *nativeObject {
	if volume == nil {
		return nil
	}
	return &volume.object
}

func newStorageVol(api *nativeAPI, ptr unsafe.Pointer) *StorageVol {
	return &StorageVol{object: newNativeObject(api, ptr, "storage volume")}
}

// LookupStorageVolByKey returns a referenced volume by globally unique key.
func (c *Connect) LookupStorageVolByKey(key string) (*StorageVol, error) {
	ptr, err := connectObjectFromString(c, "storage volume key", key, "virStorageVolLookupByKey", func(api *nativeAPI, conn unsafe.Pointer, key *byte) unsafe.Pointer {
		return api.virStorageVolLookupByKey(conn, key)
	})
	if err != nil {
		return nil, err
	}
	return newStorageVol(c.api, ptr), nil
}

// LookupStorageVolByPath returns a referenced volume by local path.
func (c *Connect) LookupStorageVolByPath(path string) (*StorageVol, error) {
	ptr, err := connectObjectFromString(c, "storage volume path", path, "virStorageVolLookupByPath", func(api *nativeAPI, conn unsafe.Pointer, path *byte) unsafe.Pointer {
		return api.virStorageVolLookupByPath(conn, path)
	})
	if err != nil {
		return nil, err
	}
	return newStorageVol(c.api, ptr), nil
}

// Free releases this wrapper's storage-volume reference.
func (volume *StorageVol) Free() error {
	return objectFree(storageVolObject(volume), "virStorageVolFree", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStorageVolFree(ptr)
	})
}

// GetName returns the volume name.
func (volume *StorageVol) GetName() (string, error) {
	return objectBorrowedString(storageVolObject(volume), "virStorageVolGetName", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virStorageVolGetName(ptr)
	})
}

// GetKey returns the volume key.
func (volume *StorageVol) GetKey() (string, error) {
	return objectBorrowedString(storageVolObject(volume), "virStorageVolGetKey", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virStorageVolGetKey(ptr)
	})
}

// GetPath returns the volume path.
func (volume *StorageVol) GetPath() (string, error) {
	return objectOwnedString(storageVolObject(volume), "virStorageVolGetPath", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virStorageVolGetPath(ptr)
	})
}

// GetXMLDesc returns the volume XML.
func (volume *StorageVol) GetXMLDesc(flags uint32) (string, error) {
	return objectOwnedString(storageVolObject(volume), "virStorageVolGetXMLDesc", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virStorageVolGetXMLDesc(ptr, flags)
	})
}

// Delete removes the storage volume.
func (volume *StorageVol) Delete(flags uint32) error {
	return objectStatus(storageVolObject(volume), "virStorageVolDelete", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStorageVolDelete(ptr, flags)
	})
}

// Wipe securely erases the storage volume according to flags.
func (volume *StorageVol) Wipe(flags uint32) error {
	return objectStatus(storageVolObject(volume), "virStorageVolWipe", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStorageVolWipe(ptr, flags)
	})
}

// Resize changes the storage volume capacity.
func (volume *StorageVol) Resize(capacity uint64, flags uint32) error {
	return objectStatus(storageVolObject(volume), "virStorageVolResize", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virStorageVolResize(ptr, capacity, flags)
	})
}
