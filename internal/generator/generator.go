// Package generator implements libvirt API XML to Go source generation.
package generator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
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

const DefaultOutput = "libvirt_api.gen.go"

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
	Library string    `xml:"-"`
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

// Config controls libvirt API generation.
type Config struct {
	APIPath      string
	FunctionMode string
	PackageDir   string
	Output       string
}

// Run generates the low-level binding using config.
func Run(config Config) error {
	if config.APIPath == "" {
		config.APIPath = "auto"
	}
	if config.FunctionMode == "" {
		config.FunctionMode = "all"
	}
	if config.PackageDir == "" {
		config.PackageDir = "."
	}
	if config.Output == "" {
		config.Output = DefaultOutput
	}
	return run(config.APIPath, config.FunctionMode, config.PackageDir, config.Output)
}

func run(apiPath, functionMode, packageDir, output string) error {
	resolvedAPI, err := resolveAPIPath(apiPath, packageDir)
	if err != nil {
		return err
	}
	document, sourceHash, err := loadAPIDocuments(resolvedAPI)
	if err != nil {
		return err
	}

	outputPath := output
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(packageDir, outputPath)
	}
	symbols, err := selectNativeSymbols(&document, functionMode, packageDir, filepath.Clean(outputPath))
	if err != nil {
		return err
	}

	generated, err := renderGenerated(&document, symbols, sourceHash)
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

func loadAPIDocuments(mainPath string) (apiDocument, string, error) {
	sources := []struct {
		library string
		path    string
	}{
		{library: "main", path: mainPath},
		{library: "admin", path: filepath.Join(filepath.Dir(mainPath), "libvirt-admin-api.xml")},
		{library: "lxc", path: filepath.Join(filepath.Dir(mainPath), "libvirt-lxc-api.xml")},
		{library: "qemu", path: filepath.Join(filepath.Dir(mainPath), "libvirt-qemu-api.xml")},
	}
	var combined apiDocument
	digest := sha256.New()
	functions := make(map[string]string)
	enums := make(map[string]string)
	for _, source := range sources {
		contents, err := os.ReadFile(source.path)
		if err != nil {
			return apiDocument{}, "", fmt.Errorf("read %s API XML %q: %w", source.library, source.path, err)
		}
		var document apiDocument
		if err := xml.Unmarshal(contents, &document); err != nil {
			return apiDocument{}, "", fmt.Errorf("parse %s API XML: %w", source.library, err)
		}
		if len(document.Functions) == 0 {
			return apiDocument{}, "", fmt.Errorf("%s API XML contains no functions", source.library)
		}
		for i := range document.Functions {
			function := &document.Functions[i]
			if previous, duplicate := functions[function.Name]; duplicate {
				return apiDocument{}, "", fmt.Errorf("function %s appears in both %s and %s API XML", function.Name, previous, source.library)
			}
			functions[function.Name] = source.library
			function.Library = source.library
		}
		for _, enum := range document.Enums {
			if previous, duplicate := enums[enum.Name]; duplicate {
				return apiDocument{}, "", fmt.Errorf("enum %s appears in both %s and %s API XML", enum.Name, previous, source.library)
			}
			enums[enum.Name] = source.library
		}
		combined.Functions = append(combined.Functions, document.Functions...)
		combined.FunctionTypes = append(combined.FunctionTypes, document.FunctionTypes...)
		combined.Enums = append(combined.Enums, document.Enums...)
		digest.Write([]byte(source.library))
		digest.Write([]byte{0})
		digest.Write(contents)
	}
	return combined, hex.EncodeToString(digest.Sum(nil)), nil
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

	data := generatedFileData{SourceHash: sourceHash}
	for _, symbol := range symbols {
		function, ok := functions[symbol]
		if !ok {
			return nil, fmt.Errorf("native symbol %s is not present in libvirt-api.xml", symbol)
		}
		signature, err := goFunctionSignature(function, callbackTypes)
		if err != nil {
			return nil, fmt.Errorf("map %s: %w", symbol, err)
		}
		arguments := make([]generatedArgumentData, len(function.Args))
		for i, argument := range function.Args {
			mapped, err := goABIType(argument.Type, false, callbackTypes)
			if err != nil {
				return nil, fmt.Errorf("map %s argument %s: %w", symbol, argument.Name, err)
			}
			arguments[i] = generatedArgumentData{Name: goArgumentName(argument.Name), Type: mapped}
		}
		result, err := goABIType(function.Return.Type, true, callbackTypes)
		if err != nil {
			return nil, fmt.Errorf("map %s return: %w", symbol, err)
		}
		library := function.Library
		if library == "" {
			library = "main"
		}
		data.Functions = append(data.Functions, generatedFunctionData{
			Name:      symbol,
			Since:     function.Version,
			Library:   library,
			Signature: signature,
			Method:    rawMethodName(symbol),
			Arguments: arguments,
			Result:    result,
			HasResult: result != "",
		})
	}

	data.Enums = append([]apiEnum(nil), document.Enums...)
	sort.Slice(data.Enums, func(i, j int) bool { return data.Enums[i].Name < data.Enums[j].Name })
	for _, enum := range data.Enums {
		if !validEnumValue(enum.Value) {
			return nil, fmt.Errorf("enum %s has unsupported value %q", enum.Name, enum.Value)
		}
	}
	for _, spec := range enumAliases {
		selected := make([]apiEnum, 0)
		for _, enum := range data.Enums {
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
		group := generatedAliasGroupData{Values: make([]generatedAliasData, len(selected))}
		for i, enum := range selected {
			group.Values[i] = generatedAliasData{
				Name:    goEnumAlias(enum.Name, spec),
				GoType:  spec.GoType,
				RawName: enum.Name,
			}
		}
		data.AliasGroups = append(data.AliasGroups, group)
	}

	var output bytes.Buffer
	if err := generatedSourceTemplate.Execute(&output, data); err != nil {
		return nil, fmt.Errorf("render generated source: %w", err)
	}
	formatted, err := format.Source(output.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated source: %w", err)
	}
	return formatted, nil
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
