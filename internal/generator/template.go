package generator

import (
	_ "embed"
	"strconv"
	"strings"
	"text/template"
)

type generatedFileData struct {
	SourceHash  string
	Functions   []generatedFunctionData
	Enums       []apiEnum
	AliasGroups []generatedAliasGroupData
}

type generatedFunctionData struct {
	Name      string
	Since     string
	Library   string
	Signature string
	Method    string
	Arguments []generatedArgumentData
	Result    string
	HasResult bool
}

type generatedArgumentData struct {
	Name string
	Type string
}

type generatedAliasGroupData struct {
	Values []generatedAliasData
}

type generatedAliasData struct {
	Name    string
	GoType  string
	RawName string
}

func (function generatedFunctionData) Declarations() string {
	declarations := make([]string, len(function.Arguments))
	for i, argument := range function.Arguments {
		declarations[i] = argument.Name + " " + argument.Type
	}
	return strings.Join(declarations, ", ")
}

func (function generatedFunctionData) ArgumentNames() string {
	names := make([]string, len(function.Arguments))
	for i, argument := range function.Arguments {
		names[i] = argument.Name
	}
	return strings.Join(names, ", ")
}

//go:embed templates/libvirt_api.go.tmpl
var generatedSourceTemplateText string

var generatedSourceTemplate = template.Must(template.New("libvirt-api").Funcs(template.FuncMap{
	"quote": strconv.Quote,
}).Parse(generatedSourceTemplateText))
