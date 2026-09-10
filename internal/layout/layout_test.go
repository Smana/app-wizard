package layout

import "testing"

func TestExpandDefault(t *testing.T) {
	if got := Expand("", "team-a", "myapp"); got != "apps/team-a/myapp" {
		t.Errorf("Expand default = %q", got)
	}
	if got := Expand(Default, "team-a", "myapp"); got != "apps/team-a/myapp" {
		t.Errorf("Expand(Default) = %q", got)
	}
}

func TestExpandCustom(t *testing.T) {
	got := Expand("tenants/{stack}/apps/{app}", "team-a", "myapp")
	if got != "tenants/team-a/apps/myapp" {
		t.Errorf("Expand custom = %q", got)
	}
}

func TestStackDirIsParentOfAppDir(t *testing.T) {
	// "workloads/{app}" used to be a case here, showing StackDir happily
	// collapsing to "workloads" regardless of stack. That shape is now
	// rejected by Validate (TestValidate below) precisely because every stack
	// reading the same directory is the bug, not a mechanic worth blessing in
	// this table. Replaced with a decorated-segment shape, which Validate
	// accepts and StackDir must still resolve correctly.
	cases := map[string]string{
		"":                           "apps/team-a",
		"apps/{stack}/{app}":         "apps/team-a",
		"tenants/{stack}/apps/{app}": "tenants/team-a/apps",
		"apps/{stack}/{app}-app":     "apps/team-a",
	}
	for l, want := range cases {
		if got := StackDir(l, "team-a"); got != want {
			t.Errorf("StackDir(%q) = %q, want %q", l, got, want)
		}
	}
}

// TestValidate covers the two rules Validate enforces: a legal layout must
// name {stack} (or every stack's walk reads the same directory) and must put
// {app} in its last path segment (or StackDir leaks the "app" expansion
// sentinel into the directory the inventory reads).
func TestValidate(t *testing.T) {
	accept := []string{
		Default,
		"tenants/{stack}/apps/{app}",
		"apps/{stack}/{app}-app",
		"apps/{stack}/app-{app}",
	}
	for _, tmpl := range accept {
		if err := Validate(tmpl); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", tmpl, err)
		}
	}

	reject := []string{
		"workloads/{app}",              // no {stack}: every stack lists the same directory
		"apps/{stack}/{app}/manifests", // {app} not in the last segment: StackDir leaks "app"
		"apps/{stack}/static",          // no {app} at all
	}
	for _, tmpl := range reject {
		if err := Validate(tmpl); err == nil {
			t.Errorf("Validate(%q) = nil, want an error", tmpl)
		}
	}
}
