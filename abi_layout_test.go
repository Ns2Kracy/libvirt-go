package libvirt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Compare generated Go layouts to the compiler's view of the public C headers.
func TestNativeLayoutsMatchLibvirtHeaders(t *testing.T) {
	compiler, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("C compiler is unavailable")
	}
	args := []string{}
	if headers := os.Getenv("LIBVIRT_HEADER_DIR"); headers != "" {
		args = append(args, "-I"+headers)
	} else {
		flags, err := exec.Command("pkg-config", "--cflags", "libvirt").Output()
		if err != nil {
			t.Skip("libvirt headers are unavailable; set LIBVIRT_HEADER_DIR")
		}
		args = append(args, strings.Fields(string(flags))...)
	}
	binary := filepath.Join(t.TempDir(), "abi-layout")
	args = append(args, "-x", "c", "testdata/abi_layout.c.in", "-o", binary)
	if output, err := exec.Command(compiler, args...).CombinedOutput(); err != nil {
		t.Fatalf("compile ABI probe: %v\n%s", err, output)
	}
	output, err := exec.Command(binary).Output()
	if err != nil {
		t.Fatal(err)
	}
	layouts := map[string]reflect.Type{
		"virTypedParameter":    reflect.TypeFor[cTypedParameter](),
		"virDomainStatsRecord": reflect.TypeFor[cDomainStatsRecord](),
		"virStoragePoolInfo":   reflect.TypeFor[cStoragePoolInfo](),
		"virStorageVolInfo":    reflect.TypeFor[cStorageVolInfo](),
		"virError":             reflect.TypeFor[cError](),
	}
	for line := range strings.Lines(string(output)) {
		var name string
		var want uintptr
		if _, err := fmt.Sscan(line, &name, &want); err != nil {
			t.Fatal(err)
		}
		structure, field, hasField := strings.Cut(name, ".")
		layout, ok := layouts[structure]
		if !ok {
			t.Fatalf("unknown C layout %s", structure)
		}
		got := layout.Size()
		if hasField {
			if field == "type" {
				field = "type_"
			}
			if structure == "virDomainStatsRecord" {
				if field == "dom" {
					field = "domain"
				}
				if field == "nparams" {
					field = "count"
				}
			}
			member, ok := layout.FieldByName(field)
			if !ok {
				t.Fatalf("missing Go field %s", name)
			}
			got = member.Offset
		}
		if got != want {
			t.Errorf("%s: Go=%d C=%d", name, got, want)
		}
	}
}
