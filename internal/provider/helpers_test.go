package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestMapFromTF_Null(t *testing.T) {
	m, diags := mapFromTF(context.Background(), types.MapNull(types.StringType))
	if diags.HasError() {
		t.Fatal(diags)
	}
	if m != nil {
		t.Errorf("expected nil, got %v", m)
	}
}

func TestMapToTF_Empty(t *testing.T) {
	m, diags := mapToTF(context.Background(), nil)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !m.IsNull() {
		t.Error("expected null for nil input")
	}
}

func TestMapRoundTrip(t *testing.T) {
	ctx := context.Background()
	input := map[string]string{"a": "1", "b": "2"}
	tfMap, diags := mapToTF(ctx, input)
	if diags.HasError() {
		t.Fatal(diags)
	}
	output, diags := mapFromTF(ctx, tfMap)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if len(output) != 2 || output["a"] != "1" || output["b"] != "2" {
		t.Errorf("roundtrip failed: %v", output)
	}
}

func TestSliceToSetTF_Empty(t *testing.T) {
	s, diags := sliceToSetTF(context.Background(), nil)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !s.IsNull() {
		t.Error("expected null for nil input")
	}
}

func TestSetRoundTrip(t *testing.T) {
	ctx := context.Background()
	input := []string{"x", "y"}
	tfSet, diags := sliceToSetTF(ctx, input)
	if diags.HasError() {
		t.Fatal(diags)
	}
	output, diags := setFromTF(ctx, tfSet)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if len(output) != 2 {
		t.Errorf("roundtrip got %d elements", len(output))
	}
}

func TestListFromStrings(t *testing.T) {
	l, diags := listFromStrings(nil)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !l.IsNull() {
		t.Error("expected null for nil input")
	}

	l, diags = listFromStrings([]string{"a", "b"})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if l.IsNull() {
		t.Fatal("expected non-null")
	}
	if len(l.Elements()) != 2 {
		t.Errorf("expected 2 elements, got %d", len(l.Elements()))
	}
}

func TestParseJSONOrNull(t *testing.T) {
	tests := []struct {
		name    string
		input   types.String
		wantErr bool
	}{
		{"null", types.StringNull(), false},
		{"empty", types.StringValue(""), false},
		{"valid", types.StringValue(`[1,2,3]`), false},
		{"invalid", types.StringValue(`{bad`), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out []any
			diags := parseJSONOrNull(tt.input, &out)
			if tt.wantErr && !diags.HasError() {
				t.Error("expected error")
			}
			if !tt.wantErr && diags.HasError() {
				t.Errorf("unexpected error: %v", diags)
			}
		})
	}
}

func TestMustJSON(t *testing.T) {
	s, diags := mustJSON(nil)
	if diags.HasError() {
		t.Fatal(diags)
	}
	if !s.IsNull() {
		t.Error("expected null for nil")
	}

	s, diags = mustJSON(map[string]any{"key": "value"})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if s.IsNull() {
		t.Fatal("expected non-null")
	}
	if s.ValueString() == "" {
		t.Error("expected non-empty JSON")
	}
}

func TestStringOrNull(t *testing.T) {
	if !stringOrNull("").IsNull() {
		t.Error("expected null for empty string")
	}
	if stringOrNull("hello").ValueString() != "hello" {
		t.Error("expected 'hello'")
	}
}

func TestAnyString(t *testing.T) {
	if anyString(42) != "" {
		t.Error("expected empty for non-string")
	}
	if anyString("hello") != "hello" {
		t.Error("expected 'hello'")
	}
}

func TestAnyBool(t *testing.T) {
	if anyBool("not-bool") {
		t.Error("expected false for non-bool")
	}
	if !anyBool(true) {
		t.Error("expected true")
	}
}

func TestAnyFloatAsInt64(t *testing.T) {
	tests := []struct {
		in   any
		want int64
	}{
		{float64(42), 42},
		{int64(7), 7},
		{int(3), 3},
		{"str", 0},
	}
	for _, tt := range tests {
		got := anyFloatAsInt64(tt.in)
		if got != tt.want {
			t.Errorf("anyFloatAsInt64(%v) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestAnySliceToStrings(t *testing.T) {
	if s := anySliceToStrings(nil); s != nil {
		t.Errorf("expected nil, got %v", s)
	}
	if s := anySliceToStrings([]string{"a"}); len(s) != 1 {
		t.Errorf("expected [a], got %v", s)
	}
	if s := anySliceToStrings([]any{"x", 42, "y"}); len(s) != 2 {
		t.Errorf("expected [x y], got %v", s)
	}
}
