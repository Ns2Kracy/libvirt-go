package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const defaultOutput = "libvirt_api.gen.go"

type apiDocument struct {
	Functions     []apiFunction     `xml:"symbols>function"`
	FunctionTypes []apiFunctionType `xml:"symbols>functype"`
	Enums         []apiEnum         `xml:"symbols>enum"`
}

type apiFunctionType struct {
	Name string `xml:"name,attr"`
}

type apiFunction struct {
	Name    string    `xml:"name,attr"`
	Version string    `xml:"version,attr"`
	Return  apiReturn `xml:"return"`
	Args    []apiArg  `xml:"arg"`
}

type apiReturn struct {
	Type string `xml:"type,attr"`
}

type apiArg struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

type apiEnum struct {
	Name    string `xml:"name,attr"`
	Type    string `xml:"type,attr"`
	Value   string `xml:"value,attr"`
	Version string `xml:"version,attr"`
}

type enumAliasSpec struct {
	XMLType  string
	GoType   string
	CPrefix  string
	GoPrefix string
}

var enumAliases = []enumAliasSpec{
	{
		XMLType:  "virConnectListAllDomainsFlags",
		GoType:   "ConnectListAllDomainsFlags",
		CPrefix:  "VIR_CONNECT_LIST_DOMAINS_",
		GoPrefix: "ConnectListDomains",
	},
	{
		XMLType:  "virDomainState",
		GoType:   "DomainState",
		CPrefix:  "VIR_DOMAIN_",
		GoPrefix: "Domain",
	},
	{
		XMLType:  "virDomainXMLFlags",
		GoType:   "DomainXMLFlags",
		CPrefix:  "VIR_DOMAIN_XML_",
		GoPrefix: "DomainXML",
	},
}

func main() {
	var apiPath string
	var functionMode string
	var packageDir string
	var output string
	flag.StringVar(&apiPath, "api", "auto", "path to libvirt-api.xml, or auto")
	flag.StringVar(&functionMode, "functions", "all", "function set to generate: all or used")
	flag.StringVar(&packageDir, "package", ".", "directory containing the libvirt Go package")
	flag.StringVar(&output, "out", defaultOutput, "generated Go output path")
	flag.Parse()

	if err := run(apiPath, functionMode, packageDir, output); err != nil {
		fmt.Fprintln(os.Stderr, "libvirt-api-gen:", err)
		os.Exit(1)
	}
}

func run(apiPath, functionMode, packageDir, output string) error {
	resolvedAPI, err := resolveAPIPath(apiPath, packageDir)
	if err != nil {
		return err
	}
	contents, err := os.ReadFile(resolvedAPI)
	if err != nil {
		return fmt.Errorf("read API XML: %w", err)
	}
	var document apiDocument
	if err := xml.Unmarshal(contents, &document); err != nil {
		return fmt.Errorf("parse API XML: %w", err)
	}
	if len(document.Functions) == 0 || len(document.Enums) == 0 {
		return errors.New("API XML contains no functions or enums")
	}

	outputPath := output
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(packageDir, outputPath)
	}
	symbols, err := selectNativeSymbols(&document, functionMode, packageDir, filepath.Clean(outputPath))
	if err != nil {
		return err
	}

	digest := sha256.Sum256(contents)
	generated, err := renderGenerated(&document, symbols, hex.EncodeToString(digest[:]))
	if err != nil {
		return err
	}
	if previous, readErr := os.ReadFile(outputPath); readErr == nil && bytes.Equal(previous, generated) {
		fmt.Printf("%s is up to date (%d symbols, %d enums)\n", outputPath, len(symbols), len(document.Enums))
		return nil
	}
	if err := os.WriteFile(outputPath, generated, 0o644); err != nil {
		return fmt.Errorf("write generated output: %w", err)
	}
	fmt.Printf("generated %s (%d symbols, %d enums)\n", outputPath, len(symbols), len(document.Enums))
	return nil
}

func resolveAPIPath(requested, packageDir string) (string, error) {
	if requested != "" && requested != "auto" {
		return existingPath(requested, packageDir)
	}
	if envPath := os.Getenv("LIBVIRT_API_XML"); envPath != "" {
		return existingPath(envPath, packageDir)
	}
	candidates := []string{
		filepath.Join(packageDir, "libvirt-api.xml"),
		filepath.Join(packageDir, "api", "libvirt-api.xml"),
		"/usr/share/libvirt/api/libvirt-api.xml",
		"/usr/local/share/libvirt/api/libvirt-api.xml",
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("cannot find libvirt-api.xml; install libvirt development metadata or set LIBVIRT_API_XML")
}

func existingPath(path, packageDir string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(packageDir, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("locate API XML %q: %w", path, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("API XML path %q is a directory", path)
	}
	return path, nil
}

func selectNativeSymbols(document *apiDocument, mode, packageDir, outputPath string) ([]string, error) {
	var symbols []string
	switch mode {
	case "all":
		symbols = make([]string, 0, len(document.Functions))
		for _, function := range document.Functions {
			symbols = append(symbols, function.Name)
		}
		sort.Strings(symbols)
	case "used":
		var err error
		symbols, err = discoverNativeSymbols(packageDir, outputPath)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid -functions value %q: use all or used", mode)
	}
	if len(symbols) == 0 {
		return nil, errors.New("no libvirt functions selected for generation")
	}
	return symbols, nil
}

func discoverNativeSymbols(packageDir, outputPath string) ([]string, error) {
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		return nil, fmt.Errorf("read package directory: %w", err)
	}
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		return nil, err
	}

	symbols := make(map[string]struct{})
	files := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(packageDir, name)
		absolute, absErr := filepath.Abs(path)
		if absErr != nil {
			return nil, absErr
		}
		if absolute == outputPath {
			continue
		}
		parsed, parseErr := parser.ParseFile(files, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, fmt.Errorf("parse %s: %w", path, parseErr)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			name := selector.Sel.Name
			if len(name) > 3 && strings.HasPrefix(name, "vir") && unicode.IsUpper(rune(name[3])) {
				symbols[name] = struct{}{}
			}
			return true
		})
	}

	result := make([]string, 0, len(symbols))
	for symbol := range symbols {
		result = append(result, symbol)
	}
	sort.Strings(result)
	return result, nil
}

func renderGenerated(document *apiDocument, symbols []string, sourceHash string) ([]byte, error) {
	functions := make(map[string]apiFunction, len(document.Functions))
	for _, function := range document.Functions {
		functions[function.Name] = function
	}
	callbackTypes := make(map[string]struct{}, len(document.FunctionTypes))
	for _, functionType := range document.FunctionTypes {
		callbackTypes[functionType.Name] = struct{}{}
	}

	var output bytes.Buffer
	fmt.Fprintln(&output, "// Code generated by internal/cmd/libvirt-api-gen; DO NOT EDIT.")
	fmt.Fprintf(&output, "// Source libvirt-api.xml SHA-256: %s.\n\n", sourceHash)
	fmt.Fprintln(&output, "package libvirt")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, `import "unsafe"`)
	fmt.Fprintln(&output)
	fmt.Fprintf(&output, "const generatedLibvirtAPIHash = %q\n\n", sourceHash)
	fmt.Fprintln(&output, "type generatedNativeAPI struct {")
	for _, symbol := range symbols {
		function, ok := functions[symbol]
		if !ok {
			return nil, fmt.Errorf("native symbol %s is not present in libvirt-api.xml", symbol)
		}
		signature, err := goFunctionSignature(function, callbackTypes)
		if err != nil {
			return nil, fmt.Errorf("map %s: %w", symbol, err)
		}
		fmt.Fprintf(&output, "\t%s %s\n", symbol, signature)
	}
	fmt.Fprintln(&output, "}")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "func libvirtSymbolBindings(api *nativeAPI) []nativeSymbolBinding {")
	fmt.Fprintln(&output, "\treturn []nativeSymbolBinding{")
	for _, symbol := range symbols {
		function := functions[symbol]
		fmt.Fprintf(&output, "\t\t{name: %q, since: %q, target: &api.%s},\n", symbol, function.Version, symbol)
	}
	fmt.Fprintln(&output, "\t}")
	fmt.Fprintln(&output, "}")
	fmt.Fprintln(&output)
	fmt.Fprintln(&output, "var generatedLibvirtSymbolVersions = map[string]string{")
	for _, symbol := range symbols {
		fmt.Fprintf(&output, "\t%q: %q,\n", symbol, functions[symbol].Version)
	}
	fmt.Fprintln(&output, "}")
	fmt.Fprintln(&output)
	if err := renderRawMethods(&output, functions, symbols, callbackTypes); err != nil {
		return nil, err
	}

	enums := append([]apiEnum(nil), document.Enums...)
	sort.Slice(enums, func(i, j int) bool { return enums[i].Name < enums[j].Name })
	fmt.Fprintln(&output, "// Raw constants retain libvirt's C names so the full enum catalog updates automatically.")
	fmt.Fprintln(&output, "const (")
	for _, enum := range enums {
		if !validEnumValue(enum.Value) {
			return nil, fmt.Errorf("enum %s has unsupported value %q", enum.Name, enum.Value)
		}
		fmt.Fprintf(&output, "\t%s = %s\n", enum.Name, enum.Value)
	}
	fmt.Fprintln(&output, ")")

	for _, spec := range enumAliases {
		selected := make([]apiEnum, 0)
		for _, enum := range enums {
			if enum.Type == spec.XMLType && strings.HasPrefix(enum.Name, spec.CPrefix) {
				selected = append(selected, enum)
			}
		}
		sort.Slice(selected, func(i, j int) bool {
			left, leftErr := strconv.ParseInt(selected[i].Value, 10, 64)
			right, rightErr := strconv.ParseInt(selected[j].Value, 10, 64)
			if leftErr == nil && rightErr == nil && left != right {
				return left < right
			}
			return selected[i].Name < selected[j].Name
		})
		if len(selected) == 0 {
			return nil, fmt.Errorf("enum type %s is not present in libvirt-api.xml", spec.XMLType)
		}
		fmt.Fprintln(&output)
		fmt.Fprintln(&output, "const (")
		for _, enum := range selected {
			alias := goEnumAlias(enum.Name, spec)
			fmt.Fprintf(&output, "\t%s %s = %s(%s)\n", alias, spec.GoType, spec.GoType, enum.Name)
		}
		fmt.Fprintln(&output, ")")
	}

	formatted, err := format.Source(output.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated source: %w", err)
	}
	return formatted, nil
}

func renderRawMethods(output *bytes.Buffer, functions map[string]apiFunction, symbols []string, callbackTypes map[string]struct{}) error {
	for _, symbol := range symbols {
		function := functions[symbol]
		declarations := make([]string, len(function.Args))
		arguments := make([]string, len(function.Args))
		for i, argument := range function.Args {
			mapped, err := goABIType(argument.Type, false, callbackTypes)
			if err != nil {
				return fmt.Errorf("map %s argument %s: %w", symbol, argument.Name, err)
			}
			name := goArgumentName(argument.Name)
			declarations[i] = name + " " + mapped
			arguments[i] = name
		}
		result, err := goABIType(function.Return.Type, true, callbackTypes)
		if err != nil {
			return fmt.Errorf("map %s return: %w", symbol, err)
		}

		method := rawMethodName(symbol)
		fmt.Fprintf(output, "// %s calls %s using its raw C ABI contract.\n", method, symbol)
		if result == "" {
			fmt.Fprintf(output, "func (raw *RawAPI) %s(%s) error {\n", method, strings.Join(declarations, ", "))
			fmt.Fprintf(output, "\tif err := raw.requireSymbol(%q); err != nil {\n\t\treturn err\n\t}\n", symbol)
			fmt.Fprintf(output, "\traw.api.%s(%s)\n\treturn nil\n}\n\n", symbol, strings.Join(arguments, ", "))
			continue
		}
		fmt.Fprintf(output, "func (raw *RawAPI) %s(%s) (%s, error) {\n", method, strings.Join(declarations, ", "), result)
		fmt.Fprintf(output, "\tif err := raw.requireSymbol(%q); err != nil {\n\t\tvar zero %s\n\t\treturn zero, err\n\t}\n", symbol, result)
		fmt.Fprintf(output, "\treturn raw.api.%s(%s), nil\n}\n\n", symbol, strings.Join(arguments, ", "))
	}
	return nil
}

func rawMethodName(symbol string) string {
	return strings.ToUpper(symbol[:1]) + symbol[1:]
}

func goArgumentName(name string) string {
	switch name {
	case "break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range", "raw", "return", "select", "struct", "switch", "type", "var":
		return name + "_"
	default:
		return name
	}
}

func goFunctionSignature(function apiFunction, callbackTypes map[string]struct{}) (string, error) {
	args := make([]string, len(function.Args))
	for i, argument := range function.Args {
		mapped, err := goABIType(argument.Type, false, callbackTypes)
		if err != nil {
			return "", fmt.Errorf("argument %s: %w", argument.Name, err)
		}
		args[i] = mapped
	}
	result, err := goABIType(function.Return.Type, true, callbackTypes)
	if err != nil {
		return "", fmt.Errorf("return: %w", err)
	}
	signature := "func(" + strings.Join(args, ", ") + ")"
	if result != "" {
		signature += " " + result
	}
	return signature, nil
}

func goABIType(cType string, result bool, callbackTypes map[string]struct{}) (string, error) {
	fields := strings.Fields(cType)
	filtered := fields[:0]
	for _, field := range fields {
		if field != "const" {
			filtered = append(filtered, field)
		}
	}
	typeName := strings.ReplaceAll(strings.Join(filtered, " "), " *", "*")

	pointerDepth := 0
	for strings.HasSuffix(typeName, "*") {
		pointerDepth++
		typeName = strings.TrimSpace(strings.TrimSuffix(typeName, "*"))
	}
	if _, callback := callbackTypes[typeName]; callback {
		if pointerDepth == 0 {
			return "uintptr", nil
		}
		return "*uintptr", nil
	}
	if strings.HasSuffix(typeName, "Ptr") {
		if pointerDepth == 0 || result {
			return "unsafe.Pointer", nil
		}
		// Libvirt pointer typedefs plus C indirection are represented by the
		// address of one pointer-sized slot at the Go call boundary.
		return "*unsafe.Pointer", nil
	}
	if pointerDepth > 1 {
		if result {
			return "unsafe.Pointer", nil
		}
		return "*unsafe.Pointer", nil
	}
	if pointerDepth == 1 {
		switch typeName {
		case "char", "unsigned char":
			if result {
				return "unsafe.Pointer", nil
			}
			return "*byte", nil
		case "void":
			return "unsafe.Pointer", nil
		case "int":
			return "*int32", nil
		case "unsigned int":
			return "*uint32", nil
		case "long":
			return "*int", nil
		case "unsigned long":
			return "*uintptr", nil
		case "long long":
			return "*int64", nil
		case "unsigned long long":
			return "*uint64", nil
		case "size_t":
			return "*uintptr", nil
		case "double":
			return "*float64", nil
		}
		return "", fmt.Errorf("unsupported pointer type %q", cType)
	}

	switch typeName {
	case "void":
		if result {
			return "", nil
		}
	case "int":
		return "int32", nil
	case "unsigned int":
		return "uint32", nil
	case "long":
		return "int", nil
	case "unsigned long", "size_t":
		return "uintptr", nil
	case "long long":
		return "int64", nil
	case "unsigned long long":
		return "uint64", nil
	case "char":
		return "int8", nil
	case "unsigned char":
		return "uint8", nil
	case "double":
		return "float64", nil
	}
	return "", fmt.Errorf("unsupported C type %q", cType)
}

func validEnumValue(value string) bool {
	if value == "" {
		return false
	}
	if value[0] == '-' {
		value = value[1:]
		if value == "" {
			return false
		}
	}
	if value[0] >= '0' && value[0] <= '9' {
		for _, char := range value {
			if char < '0' || char > '9' {
				return false
			}
		}
		return true
	}
	if !strings.HasPrefix(value, "VIR_") {
		return false
	}
	for _, char := range value {
		if char != '_' && (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}

func goEnumAlias(cName string, spec enumAliasSpec) string {
	suffix := strings.TrimPrefix(cName, spec.CPrefix)
	parts := strings.Split(suffix, "_")
	var name strings.Builder
	name.WriteString(spec.GoPrefix)
	for _, part := range parts {
		name.WriteString(goEnumWord(part))
	}
	return name.String()
}

func goEnumWord(word string) string {
	special := map[string]string{
		"CPU":         "CPU",
		"ID":          "ID",
		"MANAGEDSAVE": "ManagedSave",
		"NOSTATE":     "NoState",
		"PMSUSPENDED": "PMSuspended",
		"SHUTOFF":     "Shutoff",
		"UUID":        "UUID",
		"XML":         "XML",
	}
	if mapped, ok := special[word]; ok {
		return mapped
	}
	lower := strings.ToLower(word)
	if lower == "" {
		return ""
	}
	return strings.ToUpper(lower[:1]) + lower[1:]
}
