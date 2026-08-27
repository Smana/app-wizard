package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"

	"github.com/Smana/app-wizard/internal/api"
)

// resolvePath reports whether a dotted field key (e.g. "image.repository",
// "route.internetFacing") resolves to a property in the JSON Schema derived
// from the App XRD. Walks nested `properties` one segment at a time.
func resolvePath(schema map[string]any, dotted string) bool {
	cur := schema
	for _, seg := range strings.Split(dotted, ".") {
		props, ok := cur["properties"].(map[string]any)
		if !ok {
			return false
		}
		next, ok := props[seg].(map[string]any)
		if !ok {
			return false
		}
		cur = next
	}
	return true
}

// TestUIHintsNoDriftFromXRD is the drift guard: the App XRD is the single source
// of truth for the form's fields. ui-hints.yaml only adds presentation, so every
// field key it references MUST exist in the XRD. A renamed/removed XRD field that
// leaves a stale hint key fails this test — drift cannot be merged.
// loadHints reads the bundled presentation overlay.
func loadHints(t *testing.T) api.UIHints {
	t.Helper()
	hb, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(uiHintsPath)))
	if err != nil {
		t.Fatalf("read %s: %v", uiHintsPath, err)
	}
	var hints api.UIHints
	if err := yaml.Unmarshal(hb, &hints); err != nil {
		t.Fatalf("parse %s: %v", uiHintsPath, err)
	}
	return hints
}

func TestUIHintsNoDriftFromXRD(t *testing.T) {
	jsonSchema, _, err := ConvertXRD(loadXRD(t))
	if err != nil {
		t.Fatalf("convert XRD: %v", err)
	}
	root := repoRoot(t)
	hb, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(uiHintsPath)))
	if err != nil {
		t.Fatalf("read %s: %v", uiHintsPath, err)
	}
	var hints api.UIHints
	if err := yaml.Unmarshal(hb, &hints); err != nil {
		t.Fatalf("parse ui-hints.yaml: %v", err)
	}
	if len(hints.Fields) == 0 {
		t.Fatal("ui-hints.yaml has no fields — did the parse shape change?")
	}
	for key := range hints.Fields {
		if !resolvePath(jsonSchema, key) {
			t.Errorf("ui-hints.yaml references field %q that does not exist in the App XRD (schema drift)", key)
		}
	}
}

// TestLoadBearingUIKeysExist guards the field names the FRONTEND references by
// hand (not via ui-hints): the secrets editor (env, externalSecrets) and the
// public-exposure warning (route.internetFacing). If the XRD renames any of
// these, the UI feature silently breaks — this test turns that into a build
// failure. Keep this list in sync with the hardcoded keys in ui/src/form/.
func TestLoadBearingUIKeysExist(t *testing.T) {
	jsonSchema, _, err := ConvertXRD(loadXRD(t))
	if err != nil {
		t.Fatalf("convert XRD: %v", err)
	}
	keys := []string{
		"env",                  // WizardForm SECRET_KEYS + SecretsEditor envPath
		"externalSecrets",      // WizardForm SECRET_KEYS + SecretsEditor secretsPath
		"route.internetFacing", // Field.tsx public-exposure warning
		"service.port",         // basic-tier leaf
		"image.repository",     // ImageField parses into it (required)
		"image.tag",            // ImageField parses into it
		"image.pullPolicy",     // ImageField renders it in the advanced sub-option
	}
	for _, key := range keys {
		if !resolvePath(jsonSchema, key) {
			t.Errorf("load-bearing UI field %q is missing from the App XRD — the UI references it by name", key)
		}
	}
}

// TestBasicTierObjectsHaveBasicChildren is the guard the other direction.
//
// TestUIHintsNoDriftFromXRD only checks hint -> XRD: every hint key must exist
// in the schema. It cannot catch the opposite mistake, which is what actually
// shipped: an object field hinted `basic` whose CHILDREN carry no hints at all.
// Field.tsx limits an object's children on the first screen to those that are
// basic-tier or required by the schema, so such a field renders as a labelled
// box containing nothing — the form looks broken and the values are unreachable
// without expanding an advanced group that does not contain them either.
func TestBasicTierObjectsHaveBasicChildren(t *testing.T) {
	jsonSchema, _, err := ConvertXRD(loadXRD(t))
	if err != nil {
		t.Fatalf("convert XRD: %v", err)
	}
	hints := loadHints(t)

	props, _ := jsonSchema["properties"].(map[string]any)
	for key, hint := range hints.Fields {
		if hint.Tier != "basic" || strings.Contains(key, ".") {
			continue // only top-level basic fields have a first-screen children list
		}
		node, ok := props[key].(map[string]any)
		if !ok {
			continue
		}
		children, ok := node["properties"].(map[string]any)
		if !ok || len(children) == 0 {
			continue // scalar, array, or additionalProperties map — no child filter
		}

		required := map[string]bool{}
		if req, ok := node["required"].([]any); ok {
			for _, r := range req {
				if s, ok := r.(string); ok {
					required[s] = true
				}
			}
		}

		reachable := 0
		for child := range children {
			if required[child] || hints.Fields[key+"."+child].Tier == "basic" {
				reachable++
			}
		}
		if reachable == 0 {
			t.Errorf("field %q is hinted basic but none of its %d children are basic-tier or required — "+
				"it renders as an empty box on the first screen; add %q.<child> hints",
				key, len(children), key)
		}
	}
}
