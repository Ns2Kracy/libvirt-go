package libvirt

import (
	"runtime"
	"unsafe"
)

// DomainSnapshot is a reference-counted domain snapshot handle.
type DomainSnapshot struct {
	object nativeObject
}

func domainSnapshotObject(snapshot *DomainSnapshot) *nativeObject {
	if snapshot == nil {
		return nil
	}
	return &snapshot.object
}

func newDomainSnapshot(api *nativeAPI, ptr unsafe.Pointer) *DomainSnapshot {
	return &DomainSnapshot{object: newNativeObject(api, ptr, "domain snapshot")}
}

// ListAllSnapshots returns snapshots matching flags. Each handle must be freed.
func (domain *Domain) ListAllSnapshots(flags uint32) ([]*DomainSnapshot, error) {
	handles, err := domainListObjects(domain, "virDomainListAllSnapshots", flags, func(api *nativeAPI, ptr unsafe.Pointer, list *unsafe.Pointer, flags uint32) int32 {
		return api.virDomainListAllSnapshots(ptr, list, flags)
	})
	if err != nil {
		return nil, err
	}
	snapshots := make([]*DomainSnapshot, len(handles))
	for i, handle := range handles {
		snapshots[i] = newDomainSnapshot(domain.api, handle)
	}
	return snapshots, nil
}

// CreateSnapshotXML creates a domain snapshot.
func (domain *Domain) CreateSnapshotXML(xml string, flags uint32) (*DomainSnapshot, error) {
	buffer, xmlPtr, err := makeCString("snapshot XML", xml, false)
	if err != nil {
		return nil, err
	}
	ptr, err := domainCall(domain, "virDomainSnapshotCreateXML", func(api *nativeAPI, domainPtr unsafe.Pointer) (unsafe.Pointer, bool) {
		result := api.virDomainSnapshotCreateXML(domainPtr, xmlPtr, flags)
		return result, result == nil
	})
	runtime.KeepAlive(buffer)
	if err != nil {
		return nil, err
	}
	return newDomainSnapshot(domain.api, ptr), nil
}

// LookupSnapshotByName returns a referenced domain snapshot.
func (domain *Domain) LookupSnapshotByName(name string, flags uint32) (*DomainSnapshot, error) {
	buffer, namePtr, err := makeCString("snapshot name", name, false)
	if err != nil {
		return nil, err
	}
	ptr, err := domainCall(domain, "virDomainSnapshotLookupByName", func(api *nativeAPI, domainPtr unsafe.Pointer) (unsafe.Pointer, bool) {
		result := api.virDomainSnapshotLookupByName(domainPtr, namePtr, flags)
		return result, result == nil
	})
	runtime.KeepAlive(buffer)
	if err != nil {
		return nil, err
	}
	return newDomainSnapshot(domain.api, ptr), nil
}

// CurrentSnapshot returns the current snapshot.
func (domain *Domain) CurrentSnapshot(flags uint32) (*DomainSnapshot, error) {
	ptr, err := domainCall(domain, "virDomainSnapshotCurrent", func(api *nativeAPI, domainPtr unsafe.Pointer) (unsafe.Pointer, bool) {
		result := api.virDomainSnapshotCurrent(domainPtr, flags)
		return result, result == nil
	})
	if err != nil {
		return nil, err
	}
	return newDomainSnapshot(domain.api, ptr), nil
}

// Free releases this wrapper's snapshot reference.
func (snapshot *DomainSnapshot) Free() error {
	return objectFree(domainSnapshotObject(snapshot), "virDomainSnapshotFree", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainSnapshotFree(ptr)
	})
}

// GetName returns the snapshot name.
func (snapshot *DomainSnapshot) GetName() (string, error) {
	return objectBorrowedString(domainSnapshotObject(snapshot), "virDomainSnapshotGetName", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virDomainSnapshotGetName(ptr)
	})
}

// GetXMLDesc returns the snapshot XML.
func (snapshot *DomainSnapshot) GetXMLDesc(flags uint32) (string, error) {
	return objectOwnedString(domainSnapshotObject(snapshot), "virDomainSnapshotGetXMLDesc", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virDomainSnapshotGetXMLDesc(ptr, flags)
	})
}

// Delete removes snapshot metadata and optionally its data according to flags.
func (snapshot *DomainSnapshot) Delete(flags uint32) error {
	return objectStatus(domainSnapshotObject(snapshot), "virDomainSnapshotDelete", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainSnapshotDelete(ptr, flags)
	})
}

// Revert reverts the domain to this snapshot.
func (snapshot *DomainSnapshot) Revert(flags uint32) error {
	return objectStatus(domainSnapshotObject(snapshot), "virDomainRevertToSnapshot", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainRevertToSnapshot(ptr, flags)
	})
}

// DomainCheckpoint is a reference-counted domain checkpoint handle.
type DomainCheckpoint struct {
	object nativeObject
}

func domainCheckpointObject(checkpoint *DomainCheckpoint) *nativeObject {
	if checkpoint == nil {
		return nil
	}
	return &checkpoint.object
}

func newDomainCheckpoint(api *nativeAPI, ptr unsafe.Pointer) *DomainCheckpoint {
	return &DomainCheckpoint{object: newNativeObject(api, ptr, "domain checkpoint")}
}

// ListAllCheckpoints returns checkpoints matching flags. Each handle must be freed.
func (domain *Domain) ListAllCheckpoints(flags uint32) ([]*DomainCheckpoint, error) {
	handles, err := domainListObjects(domain, "virDomainListAllCheckpoints", flags, func(api *nativeAPI, ptr unsafe.Pointer, list *unsafe.Pointer, flags uint32) int32 {
		return api.virDomainListAllCheckpoints(ptr, list, flags)
	})
	if err != nil {
		return nil, err
	}
	checkpoints := make([]*DomainCheckpoint, len(handles))
	for i, handle := range handles {
		checkpoints[i] = newDomainCheckpoint(domain.api, handle)
	}
	return checkpoints, nil
}

// CreateCheckpointXML creates a domain checkpoint.
func (domain *Domain) CreateCheckpointXML(xml string, flags uint32) (*DomainCheckpoint, error) {
	buffer, xmlPtr, err := makeCString("checkpoint XML", xml, false)
	if err != nil {
		return nil, err
	}
	ptr, err := domainCall(domain, "virDomainCheckpointCreateXML", func(api *nativeAPI, domainPtr unsafe.Pointer) (unsafe.Pointer, bool) {
		result := api.virDomainCheckpointCreateXML(domainPtr, xmlPtr, flags)
		return result, result == nil
	})
	runtime.KeepAlive(buffer)
	if err != nil {
		return nil, err
	}
	return newDomainCheckpoint(domain.api, ptr), nil
}

// LookupCheckpointByName returns a referenced checkpoint.
func (domain *Domain) LookupCheckpointByName(name string, flags uint32) (*DomainCheckpoint, error) {
	buffer, namePtr, err := makeCString("checkpoint name", name, false)
	if err != nil {
		return nil, err
	}
	ptr, err := domainCall(domain, "virDomainCheckpointLookupByName", func(api *nativeAPI, domainPtr unsafe.Pointer) (unsafe.Pointer, bool) {
		result := api.virDomainCheckpointLookupByName(domainPtr, namePtr, flags)
		return result, result == nil
	})
	runtime.KeepAlive(buffer)
	if err != nil {
		return nil, err
	}
	return newDomainCheckpoint(domain.api, ptr), nil
}

// Free releases this wrapper's checkpoint reference.
func (checkpoint *DomainCheckpoint) Free() error {
	return objectFree(domainCheckpointObject(checkpoint), "virDomainCheckpointFree", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainCheckpointFree(ptr)
	})
}

// GetName returns the checkpoint name.
func (checkpoint *DomainCheckpoint) GetName() (string, error) {
	return objectBorrowedString(domainCheckpointObject(checkpoint), "virDomainCheckpointGetName", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virDomainCheckpointGetName(ptr)
	})
}

// GetXMLDesc returns the checkpoint XML.
func (checkpoint *DomainCheckpoint) GetXMLDesc(flags uint32) (string, error) {
	return objectOwnedString(domainCheckpointObject(checkpoint), "virDomainCheckpointGetXMLDesc", func(api *nativeAPI, ptr unsafe.Pointer) unsafe.Pointer {
		return api.virDomainCheckpointGetXMLDesc(ptr, flags)
	})
}

// ListAllChildren returns child checkpoints. Each handle must be freed.
func (checkpoint *DomainCheckpoint) ListAllChildren(flags uint32) ([]*DomainCheckpoint, error) {
	handles, err := objectListObjects(domainCheckpointObject(checkpoint), "virDomainCheckpointListAllChildren", flags, func(api *nativeAPI, ptr unsafe.Pointer, list *unsafe.Pointer, flags uint32) int32 {
		return api.virDomainCheckpointListAllChildren(ptr, list, flags)
	})
	if err != nil {
		return nil, err
	}
	children := make([]*DomainCheckpoint, len(handles))
	for i, handle := range handles {
		children[i] = newDomainCheckpoint(checkpoint.object.api, handle)
	}
	return children, nil
}

// Delete removes checkpoint metadata according to flags.
func (checkpoint *DomainCheckpoint) Delete(flags uint32) error {
	return objectStatus(domainCheckpointObject(checkpoint), "virDomainCheckpointDelete", func(api *nativeAPI, ptr unsafe.Pointer) int32 {
		return api.virDomainCheckpointDelete(ptr, flags)
	})
}
