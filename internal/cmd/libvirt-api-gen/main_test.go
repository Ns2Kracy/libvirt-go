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
	callbackTypes := map[string]struct{}{"virFreeCallback": {}}
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
		{"char ** const", false, "*unsafe.Pointer"},
		{"double *", false, "*float64"},
		{"virFreeCallback", false, "uintptr"},
		{"void", true, ""},
	}
	for _, test := range tests {
		t.Run(test.cType, func(t *testing.T) {
			got, err := goABIType(test.cType, test.result, callbackTypes)
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

func TestLoadAPIDocuments(t *testing.T) {
	dir := t.TempDir()
	sources := map[string]string{
		"libvirt-api.xml":       "virMain",
		"libvirt-admin-api.xml": "virAdmTest",
		"libvirt-lxc-api.xml":   "virLxcTest",
		"libvirt-qemu-api.xml":  "virQemuTest",
	}
	for filename, function := range sources {
		xml := `<api><symbols><function name="` + function + `" version="1.0.0"><return type="int"/></function></symbols></api>`
		if err := os.WriteFile(filepath.Join(dir, filename), []byte(xml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	document, hash, err := loadAPIDocuments(filepath.Join(dir, "libvirt-api.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Functions) != 4 || len(hash) != 64 {
		t.Fatalf("loadAPIDocuments = %d functions, hash %q", len(document.Functions), hash)
	}
	libraries := make(map[string]string)
	for _, function := range document.Functions {
		libraries[function.Name] = function.Library
	}
	for function, library := range map[string]string{
		"virMain": "main", "virAdmTest": "admin", "virLxcTest": "lxc", "virQemuTest": "qemu",
	} {
		if libraries[function] != library {
			t.Errorf("function %s library = %q, want %q", function, libraries[function], library)
		}
	}
}

func TestSelectNativeSymbolsAll(t *testing.T) {
	document := &apiDocument{Functions: []apiFunction{
		{Name: "virZed"},
		{Name: "virAlpha"},
	}}
	symbols, err := selectNativeSymbols(document, "all", t.TempDir(), defaultOutput)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"virAlpha", "virZed"}
	if !reflect.DeepEqual(symbols, want) {
		t.Fatalf("selectNativeSymbols(all) = %v, want %v", symbols, want)
	}
	if _, err := selectNativeSymbols(document, "invalid", t.TempDir(), defaultOutput); err == nil {
		t.Fatal("selectNativeSymbols accepted an invalid mode")
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
		"func (raw *RawAPI) VirConnectOpen(name *byte) (unsafe.Pointer, error)",
		`{name: "virConnectOpen", since: "0.0.3", library: "main", target: &api.virConnectOpen}`,
		`"virConnectOpen": "0.0.3"`,
		"VIR_DOMAIN_NOSTATE = 0",
		"DomainNoState DomainState = DomainState(VIR_DOMAIN_NOSTATE)",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("generated source does not contain %q", expected)
		}
	}
}
