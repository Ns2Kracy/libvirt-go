package libvirt

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"text/template"
	"time"
)

type realFixtureData struct {
	Name   string
	UUID   string
	Bridge string
	Path   string
}

func TestRealFixtureTemplates(t *testing.T) {
	data := realFixtureData{
		Name:   "libvirt-go-fixture",
		UUID:   "9b6f76f1-a87f-4e32-9ae8-35fd948c7a11",
		Bridge: "lvgo-fixture",
		Path:   "/tmp/libvirt-go-fixture",
	}
	fixtures, err := filepath.Glob("testdata/real/*.xml.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("no real integration XML fixtures found")
	}
	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			rendered := renderRealFixture(t, fixture, data)
			decoder := xml.NewDecoder(strings.NewReader(rendered))
			for {
				if _, err := decoder.Token(); err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					t.Fatalf("invalid fixture XML: %v", err)
				}
			}
		})
	}
}

func TestRealFixtureUUID(t *testing.T) {
	uuid := realFixtureUUID(t)
	if len(uuid) != 36 || uuid[8] != '-' || uuid[13] != '-' || uuid[18] != '-' || uuid[23] != '-' {
		t.Fatalf("realFixtureUUID() = %q", uuid)
	}
	if uuid[14] != '4' || !strings.ContainsRune("89ab", rune(uuid[19])) {
		t.Fatalf("realFixtureUUID() is not RFC 4122 version 4: %q", uuid)
	}
}

func TestRealIntegrationFixtures(t *testing.T) {
	if os.Getenv("LIBVIRT_REAL_INTEGRATION") != "1" {
		t.Skip("set LIBVIRT_REAL_INTEGRATION=1 to run real libvirt fixtures")
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("real integration fixtures are supported only on Linux amd64")
	}
	if os.Getenv("LIBVIRT_REAL_ALLOW_MUTATION") != "1" {
		t.Fatal("set LIBVIRT_REAL_ALLOW_MUTATION=1 to acknowledge resource creation")
	}
	uri := os.Getenv("LIBVIRT_REAL_URI")
	if uri == "" || strings.HasPrefix(uri, "test:") {
		t.Fatal("LIBVIRT_REAL_URI must select a non-test libvirt connection")
	}

	conn, err := NewConnect(uri)
	if err != nil {
		t.Fatalf("NewConnect(%q): %v", uri, err)
	}
	t.Cleanup(func() {
		if _, err := conn.Close(); err != nil && !errors.Is(err, ErrClosed) {
			t.Errorf("connection cleanup: %v", err)
		}
	})

	suffix := realFixtureSuffix(t)
	t.Run("domain", func(t *testing.T) {
		exerciseRealDomain(t, conn, suffix)
	})
	t.Run("network", func(t *testing.T) {
		exerciseRealNetwork(t, conn, suffix)
	})
	t.Run("storage", func(t *testing.T) {
		exerciseRealStorage(t, conn, suffix)
	})
	t.Run("secret", func(t *testing.T) {
		exerciseRealSecret(t, conn, suffix)
	})
	t.Run("nwfilter", func(t *testing.T) {
		exerciseRealNWFilter(t, conn, suffix)
	})
	t.Run("host-inventory", func(t *testing.T) {
		exerciseRealHostInventory(t, conn)
	})
}

func exerciseRealDomain(t *testing.T, conn *Connect, suffix string) {
	data := realFixtureData{Name: "libvirt-go-domain-" + suffix, UUID: realFixtureUUID(t)}
	domain, err := conn.DefineDomainXML(renderRealFixture(t, "testdata/real/domain.xml.tmpl", data))
	requireRealFeature(t, "DefineDomainXML", err)
	t.Cleanup(func() { cleanupRealDomain(t, domain) })

	assertRealNamedObject(t, data.Name, domain.GetName)
	assertRealUUID(t, data.UUID, domain.GetUUIDString)
	if value, err := domain.GetXMLDesc(0); err != nil || !strings.Contains(value, data.Name) {
		t.Fatalf("Domain.GetXMLDesc contains fixture name = %t, error = %v", strings.Contains(value, data.Name), err)
	}

	snapshot, err := domain.CreateSnapshotXML(renderRealFixture(t, "testdata/real/snapshot.xml.tmpl", realFixtureData{Name: "libvirt-go-snapshot-" + suffix}), 0)
	if optionalRealFeature(t, "CreateSnapshotXML", err) {
		t.Cleanup(func() {
			if err := snapshot.Delete(0); err != nil && !realOptionalError(err) {
				t.Errorf("snapshot delete: %v", err)
			}
			if err := snapshot.Free(); err != nil && !errors.Is(err, ErrClosed) {
				t.Errorf("snapshot free: %v", err)
			}
		})
		assertRealNamedObject(t, "libvirt-go-snapshot-"+suffix, snapshot.GetName)
	}

	checkpoint, err := domain.CreateCheckpointXML(renderRealFixture(t, "testdata/real/checkpoint.xml.tmpl", realFixtureData{Name: "libvirt-go-checkpoint-" + suffix}), 0)
	if optionalRealFeature(t, "CreateCheckpointXML", err) {
		t.Cleanup(func() {
			if err := checkpoint.Delete(0); err != nil && !realOptionalError(err) {
				t.Errorf("checkpoint delete: %v", err)
			}
			if err := checkpoint.Free(); err != nil && !errors.Is(err, ErrClosed) {
				t.Errorf("checkpoint free: %v", err)
			}
		})
		assertRealNamedObject(t, "libvirt-go-checkpoint-"+suffix, checkpoint.GetName)
	}

	if os.Getenv("LIBVIRT_REAL_START_GUEST") == "1" {
		exerciseRealGuestLifecycle(t, conn, domain)
	}
}

func exerciseRealGuestLifecycle(t *testing.T, conn *Connect, domain *Domain) {
	requireRealFeature(t, "RegisterDefaultEventImpl", RegisterDefaultEventImpl())
	events := make(chan DomainLifecycleEvent, 8)
	callback, err := conn.RegisterDomainLifecycleCallback(domain, func(event DomainLifecycleEvent) {
		events <- event
	})
	requireRealFeature(t, "RegisterDomainLifecycleCallback", err)
	t.Cleanup(func() {
		if err := callback.Close(); err != nil {
			t.Errorf("lifecycle callback cleanup: %v", err)
		}
	})
	requireRealFeature(t, "Domain.Create", domain.Create())

	eventLoop := make(chan error, 1)
	go func() { eventLoop <- RunDefaultEventImpl() }()
	select {
	case event := <-events:
		if event.DomainName == "" {
			t.Error("lifecycle callback returned an empty domain name")
		}
	case err := <-eventLoop:
		if err != nil {
			t.Fatalf("RunDefaultEventImpl: %v", err)
		}
		select {
		case <-events:
		case <-time.After(5 * time.Second):
			t.Fatal("no domain lifecycle event received")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for domain lifecycle event")
	}
}

func exerciseRealNetwork(t *testing.T, conn *Connect, suffix string) {
	data := realFixtureData{
		Name:   "libvirt-go-network-" + suffix,
		UUID:   realFixtureUUID(t),
		Bridge: "lvgo" + suffix,
	}
	network, err := conn.DefineNetworkXML(renderRealFixture(t, "testdata/real/network.xml.tmpl", data))
	requireRealFeature(t, "DefineNetworkXML", err)
	t.Cleanup(func() { cleanupRealNetwork(t, network) })
	requireRealFeature(t, "Network.Create", network.Create())
	assertRealNamedObject(t, data.Name, network.GetName)
	assertRealUUID(t, data.UUID, network.GetUUIDString)
	if active, err := network.IsActive(); err != nil || !active {
		t.Fatalf("Network.IsActive = (%t, %v), want true", active, err)
	}
}

func exerciseRealStorage(t *testing.T, conn *Connect, suffix string) {
	poolPath := t.TempDir()
	data := realFixtureData{Name: "libvirt-go-pool-" + suffix, UUID: realFixtureUUID(t), Path: poolPath}
	pool, err := conn.DefineStoragePoolXML(renderRealFixture(t, "testdata/real/storage-pool.xml.tmpl", data), 0)
	requireRealFeature(t, "DefineStoragePoolXML", err)
	t.Cleanup(func() { cleanupRealStoragePool(t, pool) })
	requireRealFeature(t, "StoragePool.Create", pool.Create(0))
	assertRealNamedObject(t, data.Name, pool.GetName)
	assertRealUUID(t, data.UUID, pool.GetUUIDString)

	volumeData := realFixtureData{Name: "libvirt-go-volume-" + suffix}
	volume, err := pool.CreateVolumeXML(renderRealFixture(t, "testdata/real/storage-volume.xml.tmpl", volumeData), 0)
	requireRealFeature(t, "CreateVolumeXML", err)
	t.Cleanup(func() {
		if err := volume.Delete(0); err != nil && !realOptionalError(err) {
			t.Errorf("volume delete: %v", err)
		}
		if err := volume.Free(); err != nil && !errors.Is(err, ErrClosed) {
			t.Errorf("volume free: %v", err)
		}
	})
	assertRealNamedObject(t, volumeData.Name, volume.GetName)
	if path, err := volume.GetPath(); err != nil || !strings.HasPrefix(path, poolPath) {
		t.Fatalf("StorageVol.GetPath = (%q, %v), want path under %q", path, err, poolPath)
	}
}

func exerciseRealSecret(t *testing.T, conn *Connect, suffix string) {
	data := realFixtureData{UUID: realFixtureUUID(t)}
	secret, err := conn.DefineSecretXML(renderRealFixture(t, "testdata/real/secret.xml.tmpl", data), 0)
	requireRealFeature(t, "DefineSecretXML", err)
	t.Cleanup(func() {
		if err := secret.Undefine(); err != nil && !realOptionalError(err) {
			t.Errorf("secret undefine: %v", err)
		}
		if err := secret.Free(); err != nil && !errors.Is(err, ErrClosed) {
			t.Errorf("secret free: %v", err)
		}
	})
	value := []byte("libvirt-go-real-" + suffix)
	requireRealFeature(t, "Secret.SetValue", secret.SetValue(value, 0))
	got, err := secret.GetValue(0)
	if err != nil || !bytes.Equal(got, value) {
		t.Fatalf("Secret.GetValue = (%q, %v), want %q", got, err, value)
	}
	assertRealUUID(t, data.UUID, secret.GetUUIDString)
}

func exerciseRealNWFilter(t *testing.T, conn *Connect, suffix string) {
	data := realFixtureData{Name: "libvirt-go-filter-" + suffix, UUID: realFixtureUUID(t)}
	filter, err := conn.DefineNWFilterXML(renderRealFixture(t, "testdata/real/nwfilter.xml.tmpl", data))
	requireRealFeature(t, "DefineNWFilterXML", err)
	t.Cleanup(func() {
		if err := filter.Undefine(); err != nil && !realOptionalError(err) {
			t.Errorf("NWFilter undefine: %v", err)
		}
		if err := filter.Free(); err != nil && !errors.Is(err, ErrClosed) {
			t.Errorf("NWFilter free: %v", err)
		}
	})
	assertRealNamedObject(t, data.Name, filter.GetName)
	assertRealUUID(t, data.UUID, filter.GetUUIDString)
}

func exerciseRealHostInventory(t *testing.T, conn *Connect) {
	devices, err := conn.ListAllNodeDevices(0)
	if optionalRealFeature(t, "ListAllNodeDevices", err) {
		cleanupResources(t, devices)
		if len(devices) != 0 {
			if name, err := devices[0].GetName(); err != nil || name == "" {
				t.Errorf("NodeDevice.GetName = (%q, %v)", name, err)
			}
		}
	}
	interfaces, err := conn.ListAllInterfaces(0)
	if optionalRealFeature(t, "ListAllInterfaces", err) {
		cleanupResources(t, interfaces)
		if len(interfaces) != 0 {
			if name, err := interfaces[0].GetName(); err != nil || name == "" {
				t.Errorf("Interface.GetName = (%q, %v)", name, err)
			}
		}
	}
}

func cleanupRealDomain(t *testing.T, domain *Domain) {
	t.Helper()
	if active, err := domain.IsActive(); err == nil && active {
		if err := domain.Destroy(); err != nil {
			t.Errorf("domain destroy: %v", err)
		}
	}
	if err := domain.Undefine(); err != nil && !realOptionalError(err) {
		t.Errorf("domain undefine: %v", err)
	}
	if err := domain.Free(); err != nil && !errors.Is(err, ErrClosed) {
		t.Errorf("domain free: %v", err)
	}
}

func cleanupRealNetwork(t *testing.T, network *Network) {
	t.Helper()
	if active, err := network.IsActive(); err == nil && active {
		if err := network.Destroy(); err != nil {
			t.Errorf("network destroy: %v", err)
		}
	}
	if err := network.Undefine(); err != nil && !realOptionalError(err) {
		t.Errorf("network undefine: %v", err)
	}
	if err := network.Free(); err != nil && !errors.Is(err, ErrClosed) {
		t.Errorf("network free: %v", err)
	}
}

func cleanupRealStoragePool(t *testing.T, pool *StoragePool) {
	t.Helper()
	if active, err := pool.IsActive(); err == nil && active {
		if err := pool.Destroy(); err != nil {
			t.Errorf("storage pool destroy: %v", err)
		}
	}
	if err := pool.Undefine(); err != nil && !realOptionalError(err) {
		t.Errorf("storage pool undefine: %v", err)
	}
	if err := pool.Free(); err != nil && !errors.Is(err, ErrClosed) {
		t.Errorf("storage pool free: %v", err)
	}
}

func renderRealFixture(t *testing.T, path string, data realFixtureData) string {
	t.Helper()
	fixture, err := template.ParseFiles(path)
	if err != nil {
		t.Fatal(err)
	}
	var rendered bytes.Buffer
	if err := fixture.Execute(&rendered, data); err != nil {
		t.Fatal(err)
	}
	return rendered.String()
}

func realFixtureSuffix(t *testing.T) string {
	t.Helper()
	var value [4]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value[:])
}

func realFixtureUUID(t *testing.T) string {
	t.Helper()
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatal(err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}

func assertRealNamedObject(t *testing.T, want string, get func() (string, error)) {
	t.Helper()
	got, err := get()
	if err != nil || got != want {
		t.Fatalf("GetName = (%q, %v), want %q", got, err, want)
	}
}

func assertRealUUID(t *testing.T, want string, get func() (string, error)) {
	t.Helper()
	got, err := get()
	if err != nil || got != want {
		t.Fatalf("GetUUIDString = (%q, %v), want %q", got, err, want)
	}
}

func requireRealFeature(t *testing.T, label string, err error) {
	t.Helper()
	if err == nil {
		return
	}
	if realOptionalError(err) {
		t.Skipf("%s is unavailable on this real libvirt connection: %v", label, err)
	}
	t.Fatalf("%s: %v", label, err)
}

func optionalRealFeature(t *testing.T, label string, err error) bool {
	t.Helper()
	if err == nil {
		return true
	}
	if realOptionalError(err) {
		t.Logf("%s is unavailable: %v", label, err)
		return false
	}
	t.Fatalf("%s: %v", label, err)
	return false
}

func realOptionalError(err error) bool {
	if err == nil || errors.Is(err, ErrSymbolUnavailable) || errors.Is(err, ErrClosed) {
		return true
	}
	var libvirtErr *Error
	if !errors.As(err, &libvirtErr) {
		return false
	}
	switch libvirtErr.Code {
	case int32(VIR_ERR_NO_SUPPORT), int32(VIR_ERR_OPERATION_UNSUPPORTED),
		int32(VIR_ERR_CONFIG_UNSUPPORTED), int32(VIR_ERR_OPERATION_INVALID),
		int32(VIR_ERR_NO_DOMAIN), int32(VIR_ERR_NO_NETWORK),
		int32(VIR_ERR_NO_STORAGE_POOL), int32(VIR_ERR_NO_STORAGE_VOL),
		int32(VIR_ERR_NO_SECRET), int32(VIR_ERR_NO_NWFILTER):
		return true
	default:
		return false
	}
}
