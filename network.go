package libvirt

import "unsafe"

// Network is a reference-counted libvirt virtual network handle.
type Network struct {
	object nativeObject
}

func networkObject(network *Network) *nativeObject {
	if network == nil {
		return nil
	}
	return &network.object
}

func newNetwork(api *nativeAPI, ptr unsafe.Pointer) *Network {
	return &Network{object: newNativeObject(api, ptr, "network")}
}

// ListAllNetworks returns virtual networks matching flags. Each handle must be freed.
func (c *Connect) ListAllNetworks(flags uint32) ([]*Network, error) {
	handles, err := connectListObjects(c, "virConnectListAllNetworks", flags, func(api *nativeAPI, conn unsafe.Pointer, list *unsafe.Pointer, flags uint32) int32 {
		return api.virConnectListAllNetworks(conn, list, flags)
	})
	if err != nil {
		return nil, err
	}
	networks := make([]*Network, len(handles))
	for i, handle := range handles {
		networks[i] = newNetwork(c.api, handle)
	}
	return networks, nil
}

// LookupNetworkByName returns a referenced network handle.
func (c *Connect) LookupNetworkByName(name string) (*Network, error) {
	ptr, err := connectObjectFromString(c, "network name", name, "virNetworkLookupByName", func(api *nativeAPI, conn unsafe.Pointer, name *byte) unsafe.Pointer {
		return api.virNetworkLookupByName(conn, name)
	})
	if err != nil {
		return nil, err
	}
	return newNetwork(c.api, ptr), nil
}

// LookupNetworkByUUIDString returns a referenced network handle.
func (c *Connect) LookupNetworkByUUIDString(uuid string) (*Network, error) {
	ptr, err := connectObjectFromString(c, "network UUID", uuid, "virNetworkLookupByUUIDString", func(api *nativeAPI, conn unsafe.Pointer, uuid *byte) unsafe.Pointer {
		return api.virNetworkLookupByUUIDString(conn, uuid)
	})
	if err != nil {
		return nil, err
	}
	return newNetwork(c.api, ptr), nil
}

// DefineNetworkXML defines a persistent virtual network.
func (c *Connect) DefineNetworkXML(xml string) (*Network, error) {
	ptr, err := connectObjectFromXML(c, xml, "virNetworkDefineXML", 0, func(api *nativeAPI, conn unsafe.Pointer, xml *byte, _ uint32) unsafe.Pointer {
		return api.virNetworkDefineXML(conn, xml)
	})
	if err != nil {
		return nil, err
	}
	return newNetwork(c.api, ptr), nil
}

// DefineNetworkXMLFlags defines a persistent virtual network with flags.
func (c *Connect) DefineNetworkXMLFlags(xml string, flags uint32) (*Network, error) {
	ptr, err := connectObjectFromXML(c, xml, "virNetworkDefineXMLFlags", flags, func(api *nativeAPI, conn unsafe.Pointer, xml *byte, flags uint32) unsafe.Pointer {
		return api.virNetworkDefineXMLFlags(conn, xml, flags)
	})
	if err != nil {
		return nil, err
	}
	return newNetwork(c.api, ptr), nil
}

// CreateNetworkXML creates a transient virtual network.
func (c *Connect) CreateNetworkXML(xml string) (*Network, error) {
	ptr, err := connectObjectFromXML(c, xml, "virNetworkCreateXML", 0, func(api *nativeAPI, conn unsafe.Pointer, xml *byte, _ uint32) unsafe.Pointer {
		return api.virNetworkCreateXML(conn, xml)
	})
	if err != nil {
		return nil, err
	}
	return newNetwork(c.api, ptr), nil
}

// Free releases this wrapper's network reference.
func (network *Network) Free() error {
	return objectFree(networkObject(network), "virNetworkFree", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNetworkFree(ptr)
	})
}

// GetName returns the network name.
func (network *Network) GetName() (string, error) {
	return objectBorrowedString(networkObject(network), "virNetworkGetName", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virNetworkGetName(ptr)
	})
}

// GetUUIDString returns the canonical network UUID.
func (network *Network) GetUUIDString() (string, error) {
	return objectUUIDString(networkObject(network), "virNetworkGetUUIDString", func(api *nativeAPI, ptr unsafe.Pointer, buffer *byte) int32 {
		return api.virNetworkGetUUIDString(ptr, buffer)
	})
}

// GetXMLDesc returns the network XML.
func (network *Network) GetXMLDesc(flags uint32) (string, error) {
	return objectOwnedString(networkObject(network), "virNetworkGetXMLDesc", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virNetworkGetXMLDesc(ptr, flags)
	})
}

// IsActive reports whether the network is active.
func (network *Network) IsActive() (bool, error) {
	return objectBool(networkObject(network), "virNetworkIsActive", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNetworkIsActive(ptr)
	})
}

// IsPersistent reports whether the network has persistent configuration.
func (network *Network) IsPersistent() (bool, error) {
	return objectBool(networkObject(network), "virNetworkIsPersistent", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNetworkIsPersistent(ptr)
	})
}

// GetAutostart reports whether the network starts automatically.
func (network *Network) GetAutostart() (bool, error) {
	value, err := objectCall(networkObject(network), "virNetworkGetAutostart", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		var autostart int32
		result := api.virNetworkGetAutostart(ptr, &autostart)
		return autostart, result < 0
	})
	return value == 1, err
}

// SetAutostart changes whether the network starts automatically.
func (network *Network) SetAutostart(autostart bool) error {
	value := int32(0)
	if autostart {
		value = 1
	}
	_, err := objectCall(networkObject(network), "virNetworkSetAutostart", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := api.virNetworkSetAutostart(ptr, value)
		return result, result < 0
	})
	return err
}

// Create starts an inactive persistent network.
func (network *Network) Create() error {
	return objectStatus(networkObject(network), "virNetworkCreate", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNetworkCreate(ptr)
	})
}

// Destroy stops an active network.
func (network *Network) Destroy() error {
	return objectStatus(networkObject(network), "virNetworkDestroy", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNetworkDestroy(ptr)
	})
}

// Undefine removes a persistent network definition.
func (network *Network) Undefine() error {
	return objectStatus(networkObject(network), "virNetworkUndefine", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNetworkUndefine(ptr)
	})
}

// ListAllPorts returns ports associated with this network. Each handle must be freed.
func (network *Network) ListAllPorts(flags uint32) ([]*NetworkPort, error) {
	handles, err := objectListObjects(networkObject(network), "virNetworkListAllPorts", flags, func(api *nativeAPI, ptr unsafe.Pointer, list *unsafe.Pointer, flags uint32) int32 {
		return api.virNetworkListAllPorts(ptr, list, flags)
	})
	if err != nil {
		return nil, err
	}
	ports := make([]*NetworkPort, len(handles))
	for i, handle := range handles {
		ports[i] = newNetworkPort(network.object.api, handle)
	}
	return ports, nil
}

// LookupPortByUUIDString returns a referenced network port.
func (network *Network) LookupPortByUUIDString(uuid string) (*NetworkPort, error) {
	ptr, err := objectFromString(networkObject(network), "network port UUID", uuid, "virNetworkPortLookupByUUIDString", func(api *nativeAPI, network unsafe.Pointer, uuid *byte) unsafe.Pointer {
		return api.virNetworkPortLookupByUUIDString(network, uuid)
	})
	if err != nil {
		return nil, err
	}
	return newNetworkPort(network.object.api, ptr), nil
}

// CreatePortXML creates a network port from XML.
func (network *Network) CreatePortXML(xml string, flags uint32) (*NetworkPort, error) {
	ptr, err := objectFromXML(networkObject(network), xml, "virNetworkPortCreateXML", flags, func(api *nativeAPI, network unsafe.Pointer, xml *byte, flags uint32) unsafe.Pointer {
		return api.virNetworkPortCreateXML(network, xml, flags)
	})
	if err != nil {
		return nil, err
	}
	return newNetworkPort(network.object.api, ptr), nil
}

// NetworkPort is a reference-counted virtual network port handle.
type NetworkPort struct {
	object nativeObject
}

func networkPortObject(port *NetworkPort) *nativeObject {
	if port == nil {
		return nil
	}
	return &port.object
}

func newNetworkPort(api *nativeAPI, ptr unsafe.Pointer) *NetworkPort {
	return &NetworkPort{object: newNativeObject(api, ptr, "network port")}
}

// Free releases this wrapper's network-port reference.
func (port *NetworkPort) Free() error {
	return objectFree(networkPortObject(port), "virNetworkPortFree", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNetworkPortFree(ptr)
	})
}

// GetUUIDString returns the canonical port UUID.
func (port *NetworkPort) GetUUIDString() (string, error) {
	return objectUUIDString(networkPortObject(port), "virNetworkPortGetUUIDString", func(api *nativeAPI, ptr unsafe.Pointer, buffer *byte) int32 {
		return api.virNetworkPortGetUUIDString(ptr, buffer)
	})
}

// GetXMLDesc returns the port XML.
func (port *NetworkPort) GetXMLDesc(flags uint32) (string, error) {
	return objectOwnedString(networkPortObject(port), "virNetworkPortGetXMLDesc", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virNetworkPortGetXMLDesc(ptr, flags)
	})
}

// Delete removes the network port.
func (port *NetworkPort) Delete(flags uint32) error {
	return objectStatus(networkPortObject(port), "virNetworkPortDelete", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNetworkPortDelete(ptr, flags)
	})
}
