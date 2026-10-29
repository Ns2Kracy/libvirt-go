package generator

import (
	"encoding/xml"
	"testing"
)

func TestMacroConstantsFromXML(t *testing.T) {
	var document apiDocument
	err := xml.Unmarshal([]byte(`<api><symbols>
 <macro name="VIR_NAME" string="a&amp;quot;&#10;b"/>
 <macro name="VIR_EMPTY" string=""/>
 <macro name="VIR_LIMIT" raw="9007199254740991LL /* = INT64_MAX &gt;&gt; 10 */"/>
 <macro name="VIR_LENGTH" raw="(36+1)"/>
 <macro name="VIR_ALIAS" raw="VIR_LENGTH"/>
 <macro name="VIR_FUNCTION" params="x" raw="((x) + 1)"/>
 <macro name="_virStruct" raw="_virOtherStruct"/>
 </symbols></api>`), &document)
	if err != nil {
		t.Fatal(err)
	}
	constants, err := constantMacros(document.Macros)
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]string)
	for _, constant := range constants {
		got[constant.Name] = constant.Value
	}
	for name, want := range map[string]string{"VIR_NAME": `"a&quot;\nb"`, "VIR_EMPTY": `""`, "VIR_LIMIT": "9007199254740991", "VIR_LENGTH": "(36+1)", "VIR_ALIAS": "VIR_LENGTH"} {
		if got[name] != want {
			t.Errorf("%s = %q, want %q", name, got[name], want)
		}
	}
	if len(got) != 5 {
		t.Fatalf("generated nonconstant macros: %v", got)
	}
	if _, err := constantMacros([]apiMacro{{Name: "VIR_BAD", Raw: "call()"}}); err == nil {
		t.Fatal("accepted a nonconstant macro expression")
	}
}
