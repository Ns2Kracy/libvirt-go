package libvirt

import "unsafe"

// Interface is a reference-counted host network-interface handle.
type Interface struct {
	object nativeObject
}

func interfaceObject(iface *Interface) *nativeObject {
	if iface == nil {
		return nil
	}
	return &iface.object
}

func newInterface(api *nativeAPI, ptr unsafe.Pointer) *Interface {
	return &Interface{object: newNativeObject(api, ptr, "interface")}
}

// ListAllInterfaces returns host interfaces matching flags. Each handle must be freed.
func (c *Connect) ListAllInterfaces(flags uint32) ([]*Interface, error) {
	handles, err := connectListObjects(c, "virConnectListAllInterfaces", flags, func(api *nativeAPI, conn unsafe.Pointer, list *unsafe.Pointer, flags uint32) int32 {
		return api.virConnectListAllInterfaces(conn, list, flags)
	})
	if err != nil {
		return nil, err
	}
	interfaces := make([]*Interface, len(handles))
	for i, handle := range handles {
		interfaces[i] = newInterface(c.api, handle)
	}
	return interfaces, nil
}

// LookupInterfaceByName returns a referenced host interface.
func (c *Connect) LookupInterfaceByName(name string) (*Interface, error) {
	ptr, err := connectObjectFromString(c, "interface name", name, "virInterfaceLookupByName", func(api *nativeAPI, conn unsafe.Pointer, name *byte) unsafe.Pointer {
		return api.virInterfaceLookupByName(conn, name)
	})
	if err != nil {
		return nil, err
	}
	return newInterface(c.api, ptr), nil
}

// LookupInterfaceByMACString returns a referenced host interface.
func (c *Connect) LookupInterfaceByMACString(mac string) (*Interface, error) {
	ptr, err := connectObjectFromString(c, "interface MAC", mac, "virInterfaceLookupByMACString", func(api *nativeAPI, conn unsafe.Pointer, mac *byte) unsafe.Pointer {
		return api.virInterfaceLookupByMACString(conn, mac)
	})
	if err != nil {
		return nil, err
	}
	return newInterface(c.api, ptr), nil
}

// DefineInterfaceXML defines a persistent host interface.
func (c *Connect) DefineInterfaceXML(xml string, flags uint32) (*Interface, error) {
	ptr, err := connectObjectFromXML(c, xml, "virInterfaceDefineXML", flags, func(api *nativeAPI, conn unsafe.Pointer, xml *byte, flags uint32) unsafe.Pointer {
		return api.virInterfaceDefineXML(conn, xml, flags)
	})
	if err != nil {
		return nil, err
	}
	return newInterface(c.api, ptr), nil
}

// Free releases this wrapper's interface reference.
func (iface *Interface) Free() error {
	return objectFree(interfaceObject(iface), "virInterfaceFree", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virInterfaceFree(ptr)
	})
}

// GetName returns the interface name.
func (iface *Interface) GetName() (string, error) {
	return objectBorrowedString(interfaceObject(iface), "virInterfaceGetName", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virInterfaceGetName(ptr)
	})
}

// GetMACString returns the interface MAC address.
func (iface *Interface) GetMACString() (string, error) {
	return objectBorrowedString(interfaceObject(iface), "virInterfaceGetMACString", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virInterfaceGetMACString(ptr)
	})
}

// GetXMLDesc returns the interface XML.
func (iface *Interface) GetXMLDesc(flags uint32) (string, error) {
	return objectOwnedString(interfaceObject(iface), "virInterfaceGetXMLDesc", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virInterfaceGetXMLDesc(ptr, flags)
	})
}

// IsActive reports whether the host interface is active.
func (iface *Interface) IsActive() (bool, error) {
	return objectBool(interfaceObject(iface), "virInterfaceIsActive", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virInterfaceIsActive(ptr)
	})
}

// Create activates the host interface.
func (iface *Interface) Create(flags uint32) error {
	return objectStatus(interfaceObject(iface), "virInterfaceCreate", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virInterfaceCreate(ptr, flags)
	})
}

// Destroy deactivates the host interface.
func (iface *Interface) Destroy(flags uint32) error {
	return objectStatus(interfaceObject(iface), "virInterfaceDestroy", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virInterfaceDestroy(ptr, flags)
	})
}

// Undefine removes the persistent interface definition.
func (iface *Interface) Undefine() error {
	return objectStatus(interfaceObject(iface), "virInterfaceUndefine", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virInterfaceUndefine(ptr)
	})
}
