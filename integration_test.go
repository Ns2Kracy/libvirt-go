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
