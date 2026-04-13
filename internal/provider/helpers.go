package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func mapFromTF(ctx context.Context, in types.Map) (map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if in.IsNull() || in.IsUnknown() {
		return nil, diags
	}
	out := make(map[string]string)
	d := in.ElementsAs(ctx, &out, false)
	diags.Append(d...)
	return out, diags
}

func mapToTF(ctx context.Context, in map[string]string) (types.Map, diag.Diagnostics) {
	if len(in) == 0 {
		return types.MapNull(types.StringType), nil
	}
	return types.MapValueFrom(ctx, types.StringType, in)
}

func sliceToSetTF(ctx context.Context, in []string) (types.Set, diag.Diagnostics) {
	if len(in) == 0 {
		return types.SetNull(types.StringType), nil
	}
	return types.SetValueFrom(ctx, types.StringType, in)
}

func setFromTF(ctx context.Context, in types.Set) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if in.IsNull() || in.IsUnknown() {
		return nil, diags
	}
	var out []string
	d := in.ElementsAs(ctx, &out, false)
	diags.Append(d...)
	return out, diags
}

func listValueStrings(ctx context.Context, in types.List, diags *diag.Diagnostics) []string {
	if in.IsNull() || in.IsUnknown() {
		return nil
	}
	var out []string
	*diags = append(*diags, in.ElementsAs(ctx, &out, false)...)
	return out
}

func listFromStrings(in []string) (types.List, diag.Diagnostics) {
	if len(in) == 0 {
		return types.ListNull(types.StringType), nil
	}
	vals := make([]attr.Value, 0, len(in))
	for _, v := range in {
		vals = append(vals, types.StringValue(v))
	}
	return types.ListValue(types.StringType, vals)
}

func parseJSONOrNull(raw types.String, out any) diag.Diagnostics {
	var diags diag.Diagnostics
	if raw.IsNull() || raw.IsUnknown() || raw.ValueString() == "" {
		return diags
	}
	if err := json.Unmarshal([]byte(raw.ValueString()), out); err != nil {
		diags.AddError("Invalid JSON", fmt.Sprintf("Could not decode JSON value: %v", err))
	}
	return diags
}

func mustJSON(in any) (types.String, diag.Diagnostics) {
	var diags diag.Diagnostics
	if in == nil {
		return types.StringNull(), diags
	}
	b, err := json.Marshal(in)
	if err != nil {
		diags.AddError("JSON marshal failed", err.Error())
		return types.StringNull(), diags
	}
	return types.StringValue(string(b)), diags
}

// Maps empty API strings to TF null to avoid storing meaningless empty values in state.
func stringOrNull(in string) types.String {
	if in == "" {
		return types.StringNull()
	}
	return types.StringValue(in)
}

func anyString(v any) string {
	s, _ := v.(string)
	return s
}

func anyBool(v any) bool {
	b, _ := v.(bool)
	return b
}

// JSON numbers from the API decode as float64; this safely converts to int64 for TF Int64Attribute fields.
func anyFloatAsInt64(v any) int64 {
	switch t := v.(type) {
	case float64:
		return int64(t)
	case int64:
		return t
	case int:
		return int64(t)
	default:
		return 0
	}
}

func anySliceToStrings(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		if ss, ok := v.([]string); ok {
			return ss
		}
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
