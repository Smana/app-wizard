package schema

import (
	"strings"
	"testing"
)

// TestParseGVK derives the claim GVK from assorted XRD shapes — nothing is
// special-cased to any particular group, so a foreign XRD yields its own GVK
// (FR-001, SC-002/SC-003).
func TestParseGVK(t *testing.T) {
	cases := []struct {
		name            string
		doc             string
		wantAPIVersion  string
		wantKind        string
		wantErrContains string
	}{
		{
			name: "v2 namespaced XR (names.kind, served version)",
			doc: `
spec:
  group: example.com
  scope: Namespaced
  names:
    kind: App
  versions:
    - name: v1alpha1
      served: true
`,
			wantAPIVersion: "example.com/v1alpha1",
			wantKind:       "App",
		},
		{
			name: "v1 claim (claimNames.kind wins over names.kind)",
			doc: `
spec:
  group: acme.example.com
  names:
    kind: XDatabase
  claimNames:
    kind: Database
  versions:
    - name: v1
      served: true
`,
			wantAPIVersion: "acme.example.com/v1",
			wantKind:       "Database",
		},
		{
			name: "foreign group + beta version (no special-casing)",
			doc: `
spec:
  group: platform.example.com
  names:
    kind: Service
  versions:
    - name: v1alpha1
      served: false
    - name: v1beta1
      served: true
`,
			wantAPIVersion: "platform.example.com/v1beta1",
			wantKind:       "Service",
		},
		{
			name: "first version when none flagged served",
			doc: `
spec:
  group: x.io
  names:
    kind: Thing
  versions:
    - name: v1
`,
			wantAPIVersion: "x.io/v1",
			wantKind:       "Thing",
		},
		{
			name:            "no group",
			doc:             "spec:\n  names:\n    kind: App\n  versions:\n    - name: v1\n",
			wantErrContains: "group",
		},
		{
			name:            "no kind",
			doc:             "spec:\n  group: x.io\n  versions:\n    - name: v1\n",
			wantErrContains: "kind",
		},
		{
			name:            "no versions",
			doc:             "spec:\n  group: x.io\n  names:\n    kind: App\n",
			wantErrContains: "versions",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gvk, err := ParseGVK([]byte(tc.doc))
			if tc.wantErrContains != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrContains) {
					t.Fatalf("err = %v, want containing %q", err, tc.wantErrContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseGVK: %v", err)
			}
			if gvk.APIVersion != tc.wantAPIVersion || gvk.Kind != tc.wantKind {
				t.Errorf("GVK = %s/%s, want %s/%s", gvk.APIVersion, gvk.Kind, tc.wantAPIVersion, tc.wantKind)
			}
		})
	}
}
