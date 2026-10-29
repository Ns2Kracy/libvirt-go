package generator

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type apiMacro struct {
	Name   string  `xml:"name,attr"`
	String *string `xml:"string,attr"`
	Raw    string  `xml:"raw,attr"`
	Params *string `xml:"params,attr"`
}

var cIntegerSuffix = regexp.MustCompile(`(?i)\b(0x[0-9a-f]+|[0-9]+)(ull|llu|ll|ul|lu|u|l)\b`)

func constantMacros(macros []apiMacro) ([]apiEnum, error) {
	result := make([]apiEnum, 0, len(macros))
	seen := make(map[string]string)
	for _, macro := range macros {
		// Function macros and C struct-tag aliases are not value constants.
		if macro.Params != nil || strings.HasPrefix(macro.Name, "_") {
			continue
		}
		value := ""
		if macro.String != nil {
			value = strconv.Quote(*macro.String)
		} else {
			raw, _, _ := strings.Cut(macro.Raw, "/*")
			value = cIntegerSuffix.ReplaceAllString(strings.TrimSpace(raw), "$1")
			expr, err := parser.ParseExpr(value)
			if err != nil {
				return nil, fmt.Errorf("macro %s: %w", macro.Name, err)
			}
			valid := true
			ast.Inspect(expr, func(node ast.Node) bool {
				switch n := node.(type) {
				case nil, *ast.ParenExpr, *ast.Ident:
				case *ast.BasicLit:
					valid = valid && n.Kind == token.INT
				case *ast.BinaryExpr:
					valid = valid && strings.Contains(" + - * / % << >> & | ^ ", " "+n.Op.String()+" ")
				case *ast.UnaryExpr:
					valid = valid && (n.Op == token.ADD || n.Op == token.SUB || n.Op == token.XOR)
				default:
					valid = false
				}
				return valid
			})
			if !valid {
				return nil, fmt.Errorf("macro %s has unsupported constant expression %q", macro.Name, macro.Raw)
			}
		}
		if previous, ok := seen[macro.Name]; ok {
			if previous != value {
				return nil, fmt.Errorf("conflicting macro %s", macro.Name)
			}
			continue
		}
		seen[macro.Name] = value
		result = append(result, apiEnum{Name: macro.Name, Value: value})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
