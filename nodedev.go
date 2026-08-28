package libvirt

import (
	"runtime"
	"unsafe"
)

// NodeDevice is a reference-counted host node-device handle.
type NodeDevice struct {
	object nativeObject
}

func nodeDeviceObject(device *NodeDevice) *nativeObject {
	if device == nil {
		return nil
	}
	return &device.object
}

func newNodeDevice(api *nativeAPI, ptr unsafe.Pointer) *NodeDevice {
	return &NodeDevice{object: newNativeObject(api, ptr, "node device")}
}

// ListAllNodeDevices returns node devices matching flags. Each handle must be freed.
func (c *Connect) ListAllNodeDevices(flags uint32) ([]*NodeDevice, error) {
	handles, err := connectListObjects(c, "virConnectListAllNodeDevices", flags, func(api *nativeAPI, conn unsafe.Pointer, list *unsafe.Pointer, flags uint32) int32 {
		return api.virConnectListAllNodeDevices(conn, list, flags)
	})
	if err != nil {
		return nil, err
	}
	devices := make([]*NodeDevice, len(handles))
	for i, handle := range handles {
		devices[i] = newNodeDevice(c.api, handle)
	}
	return devices, nil
}

// LookupNodeDeviceByName returns a referenced node device.
func (c *Connect) LookupNodeDeviceByName(name string) (*NodeDevice, error) {
	ptr, err := connectObjectFromString(c, "node device name", name, "virNodeDeviceLookupByName", func(api *nativeAPI, conn unsafe.Pointer, name *byte) unsafe.Pointer {
		return api.virNodeDeviceLookupByName(conn, name)
	})
	if err != nil {
		return nil, err
	}
	return newNodeDevice(c.api, ptr), nil
}

// CreateNodeDeviceXML creates a node device from XML.
func (c *Connect) CreateNodeDeviceXML(xml string, flags uint32) (*NodeDevice, error) {
	ptr, err := connectObjectFromXML(c, xml, "virNodeDeviceCreateXML", flags, func(api *nativeAPI, conn unsafe.Pointer, xml *byte, flags uint32) unsafe.Pointer {
		return api.virNodeDeviceCreateXML(conn, xml, flags)
	})
	if err != nil {
		return nil, err
	}
	return newNodeDevice(c.api, ptr), nil
}

// Free releases this wrapper's node-device reference.
func (device *NodeDevice) Free() error {
	return objectFree(nodeDeviceObject(device), "virNodeDeviceFree", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNodeDeviceFree(ptr)
	})
}

// GetName returns the node-device name.
func (device *NodeDevice) GetName() (string, error) {
	return objectBorrowedString(nodeDeviceObject(device), "virNodeDeviceGetName", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virNodeDeviceGetName(ptr)
	})
}

// GetXMLDesc returns the node-device XML.
func (device *NodeDevice) GetXMLDesc(flags uint32) (string, error) {
	return objectOwnedString(nodeDeviceObject(device), "virNodeDeviceGetXMLDesc", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virNodeDeviceGetXMLDesc(ptr, flags)
	})
}

// Destroy destroys a transient node device.
func (device *NodeDevice) Destroy() error {
	return objectStatus(nodeDeviceObject(device), "virNodeDeviceDestroy", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNodeDeviceDestroy(ptr)
	})
}

// Detach detaches the device from the host driver.
func (device *NodeDevice) Detach(driverName string, flags uint32) error {
	buffer, driverPtr, err := makeCString("node device driver", driverName, true)
	if err != nil {
		return err
	}
	_, err = objectCall(nodeDeviceObject(device), "virNodeDeviceDetachFlags", func(api *nativeAPI, ptr unsafe.Pointer) (int32, bool) {
		result := api.virNodeDeviceDetachFlags(ptr, driverPtr, flags)
		return result, result < 0
	})
	runtime.KeepAlive(buffer)
	return err
}

// Reattach reattaches the device to the host driver.
func (device *NodeDevice) Reattach() error {
	return objectStatus(nodeDeviceObject(device), "virNodeDeviceReAttach", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNodeDeviceReAttach(ptr)
	})
}

// Reset resets the node device.
func (device *NodeDevice) Reset() error {
	return objectStatus(nodeDeviceObject(device), "virNodeDeviceReset", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virNodeDeviceReset(ptr)
	})
}
