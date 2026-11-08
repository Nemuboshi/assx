package main

import (
	"strings"
	"testing"
)

func TestParamSpec(t *testing.T) {
	for _, test := range []struct {
		name    string
		row     param
		scope   uint8
		want    string
		wantErr string
	}{
		{"integer range", param{Index: "1", Kind: "int", Value: "integer", Min: "0", Max: "3"}, 7,
			`{index:1,scope:7,value:"integer",min:0,max:3,hasRange:true}`, ""},
		{"value only", param{Index: "1", Kind: "number", Value: "number"}, 7,
			`{index:1,scope:7,value:"number"}`, ""},
		{"renderer override", param{Index: "1", Value: "integer", Renderer: "xy"}, 7,
			`{index:1,scope:2,value:"integer"}`, ""},
		{"empty row", param{Index: "1", Kind: "int"}, 7, "", ""},
		{"unknown value", param{Index: "1", Value: "decimal"}, 7, "", "invalid value kind"},
		{"half range", param{Index: "1", Value: "integer", Min: "0"}, 7, "", "min and max"},
		{"range on hex", param{Index: "1", Value: "hex", Min: "0", Max: "1"}, 7, "", "only to integer or number"},
		{"inverted range", param{Index: "1", Value: "integer", Min: "5", Max: "1"}, 7, "", "exceeds max"},
		{"bad index", param{Index: "0", Value: "integer"}, 7, "", "invalid argument index"},
		{"scope escapes signature", param{Index: "1", Value: "integer", Renderer: "vsm"}, 1, "", "exceeds signature"},
	} {
		got, _, _, err := paramSpec(test.row, test.scope)
		if test.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("%s: err = %v, want %q", test.name, err, test.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", test.name, err)
		}
		if got != test.want {
			t.Errorf("%s: got %q, want %q", test.name, got, test.want)
		}
	}
}
