package libvirt

import "testing"

func TestVersionRoundTrip(t *testing.T) {
	raw := uint64(11_006_002)
	version := DecodeVersion(raw)
	if version.Major != 11 || version.Minor != 6 || version.Release != 2 {
		t.Fatalf("DecodeVersion(%d) = %#v", raw, version)
	}
	if got := version.Number(); got != raw {
		t.Fatalf("Version.Number() = %d, want %d", got, raw)
	}
	if got := version.String(); got != "11.6.2" {
		t.Fatalf("Version.String() = %q, want %q", got, "11.6.2")
	}
}
