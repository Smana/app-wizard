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
	cases := map[string]string{
		"":                           "apps/team-a",
		"apps/{stack}/{app}":         "apps/team-a",
		"tenants/{stack}/apps/{app}": "tenants/team-a/apps",
		"workloads/{app}":            "workloads",
	}
	for l, want := range cases {
		if got := StackDir(l, "team-a"); got != want {
			t.Errorf("StackDir(%q) = %q, want %q", l, got, want)
		}
	}
}
