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
