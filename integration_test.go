package libvirt

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestIntegrationTestDriver(t *testing.T) {
	if os.Getenv("LIBVIRT_INTEGRATION") != "1" {
		t.Skip("set LIBVIRT_INTEGRATION=1 to test against the local libvirt runtime")
	}

	rawVersion, err := GetVersion()
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if DecodeVersion(rawVersion).Major == 0 {
		t.Fatalf("invalid library version %d", rawVersion)
	}

	raw, err := NewRawAPI()
	if err != nil {
		t.Fatalf("NewRawAPI: %v", err)
	}
	var rawAPIVersion uintptr
	status, err := raw.VirGetVersion(&rawAPIVersion, nil, nil)
	if err != nil || status != 0 || uint64(rawAPIVersion) != rawVersion {
		t.Fatalf("RawAPI.VirGetVersion = (%d, %d, %v), want (0, %d, nil)", status, rawAPIVersion, err, rawVersion)
	}

	for _, symbol := range []string{"virGetVersion", "virConnectOpenReadOnly", "virDomainGetName", "virAdmConnectOpen", "virDomainLxcOpenNamespace", "virDomainQemuAgentCommand"} {
		available, availableErr := HasSymbol(symbol)
		if availableErr != nil || !available {
			t.Fatalf("HasSymbol(%q) = (%t, %v), want (true, nil)", symbol, available, availableErr)
		}
		if since, known := SymbolVersion(symbol); !known || since == "" {
			t.Fatalf("SymbolVersion(%q) = (%q, %t)", symbol, since, known)
		}
	}

	conn, err := NewConnectReadOnly("test:///default")
	if err != nil {
		t.Fatalf("NewConnectReadOnly: %v", err)
	}
	t.Cleanup(func() {
		if _, closeErr := conn.Close(); closeErr != nil && !errors.Is(closeErr, ErrClosed) {
			t.Errorf("Close: %v", closeErr)
		}
	})

	uri, err := conn.GetURI()
	if err != nil {
		t.Fatalf("GetURI: %v", err)
	}
	if uri != "test:///default" {
		t.Errorf("GetURI = %q, want test:///default", uri)
	}
	if alive, aliveErr := conn.IsAlive(); aliveErr != nil || !alive {
		t.Fatalf("IsAlive = (%t, %v), want (true, nil)", alive, aliveErr)
	}
	if version, versionErr := conn.GetLibVersion(); versionErr != nil || version == 0 {
		t.Fatalf("GetLibVersion = (%d, %v)", version, versionErr)
	}
	if version, versionErr := conn.GetVersion(); versionErr != nil || version == 0 {
		t.Fatalf("GetVersion = (%d, %v)", version, versionErr)
	}

	domains, err := conn.ListAllDomains(0)
	if err != nil {
		t.Fatalf("ListAllDomains: %v", err)
	}
	if len(domains) == 0 {
		t.Fatal("ListAllDomains returned no test domains")
	}
	for _, domain := range domains {
		defer func(domain *Domain) {
			if freeErr := domain.Free(); freeErr != nil && !errors.Is(freeErr, ErrClosed) {
				t.Errorf("Free: %v", freeErr)
			}
		}(domain)
	}

	domain := domains[0]
	name, err := domain.GetName()
	if err != nil || name == "" {
		t.Fatalf("GetName = (%q, %v)", name, err)
	}
	uuid, err := domain.GetUUIDString()
	if err != nil || len(uuid) != 36 {
		t.Fatalf("GetUUIDString = (%q, %v)", uuid, err)
	}
	state, _, err := domain.GetState()
	if err != nil || state < DomainNoState || state > DomainPMSuspended {
		t.Fatalf("GetState = (%d, %v)", state, err)
	}
	if _, err := domain.IsActive(); err != nil {
		t.Fatalf("IsActive: %v", err)
	}
	xml, err := domain.GetXMLDesc(0)
	if err != nil || !strings.Contains(xml, "<domain") {
		t.Fatalf("GetXMLDesc returned valid domain XML = %t, error = %v", strings.Contains(xml, "<domain"), err)
	}

	exerciseNextAreaAPIs(t, conn, domain)

	lookedUp, err := conn.LookupDomainByName(name)
	if err != nil {
		t.Fatalf("LookupDomainByName(%q): %v", name, err)
	}
	if err := lookedUp.Free(); err != nil {
		t.Fatalf("looked-up Domain.Free: %v", err)
	}

	_, err = conn.LookupDomainByName("__purego_binding_missing_domain__")
	var libvirtErr *Error
	if !errors.As(err, &libvirtErr) || libvirtErr.Message == "" {
		t.Fatalf("missing domain error = %#v, want populated *Error", err)
	}
}

type freeableResource interface {
	Free() error
}

type namedXMLResource interface {
	freeableResource
	GetName() (string, error)
	GetXMLDesc(uint32) (string, error)
}

func cleanupResources[T freeableResource](t *testing.T, resources []T) {
	t.Helper()
	t.Cleanup(func() {
		for _, resource := range resources {
			if err := resource.Free(); err != nil && !errors.Is(err, ErrClosed) {
				t.Errorf("resource Free: %v", err)
			}
		}
	})
}

func inspectNamedXMLResources[T namedXMLResource](t *testing.T, label string, resources []T) {
	t.Helper()
	cleanupResources(t, resources)
	if len(resources) == 0 {
		return
	}
	name, err := resources[0].GetName()
	if err != nil || name == "" {
		t.Errorf("%s GetName = (%q, %v)", label, name, err)
	}
	xml, err := resources[0].GetXMLDesc(0)
	if err != nil || xml == "" {
		t.Errorf("%s GetXMLDesc returned %d bytes, error = %v", label, len(xml), err)
	}
}

func unsupportedByDriver(err error) bool {
	var libvirtErr *Error
	return errors.As(err, &libvirtErr) && libvirtErr.Code == int32(VIR_ERR_NO_SUPPORT)
}

func allowUnsupported(t *testing.T, label string, err error) bool {
	t.Helper()
	if err == nil {
		return true
	}
	if unsupportedByDriver(err) {
		t.Logf("%s is not supported by test:///default: %v", label, err)
		return false
	}
	t.Fatalf("%s: %v", label, err)
	return false
}

func exerciseNextAreaAPIs(t *testing.T, conn *Connect, domain *Domain) {
	t.Helper()

	networks, err := conn.ListAllNetworks(0)
	if allowUnsupported(t, "ListAllNetworks", err) {
		inspectNamedXMLResources(t, "network", networks)
		if len(networks) != 0 {
			ports, portErr := networks[0].ListAllPorts(0)
			if allowUnsupported(t, "Network.ListAllPorts", portErr) {
				cleanupResources(t, ports)
			}
		}
	}

	pools, err := conn.ListAllStoragePools(0)
	if allowUnsupported(t, "ListAllStoragePools", err) {
		inspectNamedXMLResources(t, "storage pool", pools)
		if len(pools) != 0 {
			volumes, volumeErr := pools[0].ListAllVolumes(0)
			if allowUnsupported(t, "StoragePool.ListAllVolumes", volumeErr) {
				cleanupResources(t, volumes)
				if len(volumes) != 0 {
					if name, nameErr := volumes[0].GetName(); nameErr != nil || name == "" {
						t.Errorf("StorageVol.GetName = (%q, %v)", name, nameErr)
					}
				}
			}
		}
	}

	devices, err := conn.ListAllNodeDevices(0)
	if allowUnsupported(t, "ListAllNodeDevices", err) {
		inspectNamedXMLResources(t, "node device", devices)
	}
	interfaces, err := conn.ListAllInterfaces(0)
	if allowUnsupported(t, "ListAllInterfaces", err) {
		inspectNamedXMLResources(t, "interface", interfaces)
	}
	filters, err := conn.ListAllNWFilters(0)
	if allowUnsupported(t, "ListAllNWFilters", err) {
		inspectNamedXMLResources(t, "network filter", filters)
	}
	secrets, err := conn.ListAllSecrets(0)
	if allowUnsupported(t, "ListAllSecrets", err) {
		cleanupResources(t, secrets)
	}

	snapshots, err := domain.ListAllSnapshots(0)
	if allowUnsupported(t, "Domain.ListAllSnapshots", err) {
		inspectNamedXMLResources(t, "domain snapshot", snapshots)
	}
	checkpoints, err := domain.ListAllCheckpoints(0)
	if allowUnsupported(t, "Domain.ListAllCheckpoints", err) {
		inspectNamedXMLResources(t, "domain checkpoint", checkpoints)
	}
	if params, paramsErr := domain.GetMemoryParameters(0); allowUnsupported(t, "Domain.GetMemoryParameters", paramsErr) && len(params) == 0 {
		t.Log("test driver returned no memory typed parameters")
	}

	if err := RegisterDefaultEventImpl(); allowUnsupported(t, "RegisterDefaultEventImpl", err) {
		closeCallback, callbackErr := conn.RegisterCloseCallback(func(int32) {})
		if allowUnsupported(t, "RegisterCloseCallback", callbackErr) {
			if err := closeCallback.Close(); err != nil {
				t.Errorf("CloseCallback.Close: %v", err)
			}
		}
		lifecycle, lifecycleErr := conn.RegisterDomainLifecycleCallback(domain, func(DomainLifecycleEvent) {})
		if allowUnsupported(t, "RegisterDomainLifecycleCallback", lifecycleErr) {
			if err := lifecycle.Close(); err != nil {
				t.Errorf("DomainEventCallback.Close: %v", err)
			}
		}
	}
}
