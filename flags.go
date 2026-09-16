package libvirt

// ConnectListAllDomainsFlags filters ListAllDomains results. Flags in the same
// group are ORed; a group with no selected bits does not filter the results.
// Values are generated from virConnectListAllDomainsFlags in libvirt-api.xml.
type ConnectListAllDomainsFlags uint32

// DomainState describes a domain's current execution state. Values are
// generated from virDomainState in libvirt-api.xml.
type DomainState int32

// DomainXMLFlags controls which form of domain XML GetXMLDesc returns. Values
// are generated from virDomainXMLFlags in libvirt-api.xml.
type DomainXMLFlags uint32

// DomainDeviceModifyFlags controls whether device changes affect the live or
// persistent domain configuration.
type DomainDeviceModifyFlags = uint32

// DomainSnapshotRevertFlags controls domain state after snapshot reversion.
type DomainSnapshotRevertFlags = uint32

// DomainDestroyFlags controls domain destruction behavior.
type DomainDestroyFlags = uint32

// DomainUndefineFlagsValues controls which domain metadata is removed.
type DomainUndefineFlagsValues = uint32

// DomainGetJobStatsFlags controls retrieval of completed job statistics.
type DomainGetJobStatsFlags = uint32

// DomainSnapshotCreateFlags controls snapshot creation behavior.
type DomainSnapshotCreateFlags = uint32
