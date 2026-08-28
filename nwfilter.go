package libvirt

import "unsafe"

// NWFilter is a reference-counted libvirt network-filter handle.
type NWFilter struct {
	object nativeObject
}

func nwFilterObject(filter *NWFilter) *nativeObject {
	if filter == nil {
		return nil
	}
	return &filter.object
}

func newNWFilter(api *nativeAPI, ptr unsafe.Pointer) *NWFilter {
	return &NWFilter{object: newNativeObject(api, ptr, "network filter")}
}

// ListAllNWFilters returns network filters matching flags. Each handle must be freed.
func (c *Connect) ListAllNWFilters(flags uint32) ([]*NWFilter, error) {
	handles, err := connectListObjects(c, "virConnectListAllNWFilters", flags, func(api *nativeAPI, conn unsafe.Pointer, list *unsafe.Pointer, flags uint32) int32 {
		return api.virConnectListAllNWFilters(conn, list, flags)
	})
	if err != nil {
		return nil, err
	}
	filters := make([]*NWFilter, len(handles))
	for i, handle := range handles {
		filters[i] = newNWFilter(c.api, handle)
	}
	return filters, nil
}

// LookupNWFilterByName returns a referenced network filter.
func (c *Connect) LookupNWFilterByName(name string) (*NWFilter, error) {
	ptr, err := connectObjectFromString(c, "network filter name", name, "virNWFilterLookupByName", func(api *nativeAPI, conn unsafe.Pointer, name *byte) unsafe.Pointer {
		return api.virNWFilterLookupByName(conn, name)
	})
	if err != nil {
		return nil, err
	}
	return newNWFilter(c.api, ptr), nil
}

// LookupNWFilterByUUIDString returns a referenced network filter.
func (c *Connect) LookupNWFilterByUUIDString(uuid string) (*NWFilter, error) {
	ptr, err := connectObjectFromString(c, "network filter UUID", uuid, "virNWFilterLookupByUUIDString", func(api *nativeAPI, conn unsafe.Pointer, uuid *byte) unsafe.Pointer {
		return api.virNWFilterLookupByUUIDString(conn, uuid)
	})
	if err != nil {
		return nil, err
	}
	return newNWFilter(c.api, ptr), nil
}

// DefineNWFilterXML defines a network filter.
func (c *Connect) DefineNWFilterXML(xml string) (*NWFilter, error) {
	ptr, err := connectObjectFromXML(c, xml, "virNWFilterDefineXML", 0, func(api *nativeAPI, conn unsafe.Pointer, xml *byte, _ uint32) unsafe.Pointer {
		return api.virNWFilterDefineXML(conn, xml)
	})
	if err != nil {
		return nil, err
	}
	return newNWFilter(c.api, ptr), nil
}

// Free releases this wrapper's network-filter reference.
func (filter *NWFilter) Free() error {
	return objectFree(nwFilterObject(filter), "virNWFilterFree", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNWFilterFree(ptr)
	})
}

// GetName returns the network-filter name.
func (filter *NWFilter) GetName() (string, error) {
	return objectBorrowedString(nwFilterObject(filter), "virNWFilterGetName", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virNWFilterGetName(ptr)
	})
}

// GetUUIDString returns the canonical filter UUID.
func (filter *NWFilter) GetUUIDString() (string, error) {
	return objectUUIDString(nwFilterObject(filter), "virNWFilterGetUUIDString", func(api *nativeAPI, ptr unsafe.Pointer, buffer *byte) int32 {
		return api.virNWFilterGetUUIDString(ptr, buffer)
	})
}

// GetXMLDesc returns the network-filter XML.
func (filter *NWFilter) GetXMLDesc(flags uint32) (string, error) {
	return objectOwnedString(nwFilterObject(filter), "virNWFilterGetXMLDesc", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virNWFilterGetXMLDesc(ptr, flags)
	})
}

// Undefine removes the network-filter definition.
func (filter *NWFilter) Undefine() error {
	return objectStatus(nwFilterObject(filter), "virNWFilterUndefine", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNWFilterUndefine(ptr)
	})
}
