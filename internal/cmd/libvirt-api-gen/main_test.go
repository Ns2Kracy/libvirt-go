package main

import (
	"encoding/xml"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGoABIType(t *testing.T) {
	tests := []struct {
		cType  string
		result bool
		want   string
	}{
		{"int", true, "int32"},
		{"unsigned int", false, "uint32"},
		{"unsigned long *", false, "*uintptr"},
		{"const char *", false, "*byte"},
		{"char *", true, "unsafe.Pointer"},
		{"virConnectPtr", false, "unsafe.Pointer"},
		{"virDomainPtr **", false, "*unsafe.Pointer"},
		{"void", true, ""},
	}
	for _, test := range tests {
		t.Run(test.cType, func(t *testing.T) {
			got, err := goABIType(test.cType, test.result)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("goABIType(%q, %t) = %q, want %q", test.cType, test.result, got, test.want)
			}
		})
	}
}

func TestGoEnumAlias(t *testing.T) {
	tests := map[string]string{
		"VIR_CONNECT_LIST_DOMAINS_NO_MANAGEDSAVE": "ConnectListDomainsNoManagedSave",
		"VIR_DOMAIN_NOSTATE":                      "DomainNoState",
		"VIR_DOMAIN_PMSUSPENDED":                  "DomainPMSuspended",
		"VIR_DOMAIN_XML_UPDATE_CPU":               "DomainXMLUpdateCPU",
	}
	for cName, want := range tests {
		var spec enumAliasSpec
		for _, candidate := range enumAliases {
			if strings.HasPrefix(cName, candidate.CPrefix) {
				spec = candidate
				break
			}
		}
		if got := goEnumAlias(cName, spec); got != want {
			t.Errorf("goEnumAlias(%q) = %q, want %q", cName, got, want)
		}
	}
}

func TestDiscoverNativeSymbols(t *testing.T) {
	dir := t.TempDir()
	source := `package libvirt
func use(api *nativeAPI, domain *Domain) {
	api.virConnectOpen(nil)
	domain.api.virDomainGetName(domain.ptr)
}
`
	if err := os.WriteFile(filepath.Join(dir, "binding.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "binding_test.go"), []byte("package libvirt\nfunc ignored(){ api.virTestOnly() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	symbols, err := discoverNativeSymbols(dir, filepath.Join(dir, defaultOutput))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"virConnectOpen", "virDomainGetName"}
	if !reflect.DeepEqual(symbols, want) {
		t.Fatalf("discoverNativeSymbols() = %v, want %v", symbols, want)
	}
}

func TestRenderGenerated(t *testing.T) {
	input := `<api><symbols>
<function name="virConnectOpen" version="0.0.3"><return type="virConnectPtr"/><arg name="name" type="const char *"/></function>
<enum name="VIR_CONNECT_LIST_DOMAINS_ACTIVE" type="virConnectListAllDomainsFlags" value="1" version="0.9.13"/>
<enum name="VIR_DOMAIN_NOSTATE" type="virDomainState" value="0" version="0.0.1"/>
<enum name="VIR_DOMAIN_XML_SECURE" type="virDomainXMLFlags" value="1" version="0.3.3"/>
</symbols></api>`
	var document apiDocument
	if err := xml.Unmarshal([]byte(input), &document); err != nil {
		t.Fatal(err)
	}
	generated, err := renderGenerated(&document, []string{"virConnectOpen"}, "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "generated.go", generated, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, generated)
	}
	text := strings.Join(strings.Fields(string(generated)), " ")
	for _, expected := range []string{
		"virConnectOpen func(*byte) unsafe.Pointer",
		`{name: "virConnectOpen", target: &api.virConnectOpen}`,
		"VIR_DOMAIN_NOSTATE = 0",
		"DomainNoState DomainState = DomainState(VIR_DOMAIN_NOSTATE)",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
}
