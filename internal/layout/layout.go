// Package layout owns the file-layout template that decides where an app's
// manifests live in the GitOps repo. Both writers (the PR service) and readers
// (the inventory) go through it, so they cannot disagree.
package layout

import (
	"fmt"
	"path"
	"strings"
)

// Default reproduces the historical layout.
const Default = "apps/{stack}/{app}"

// Expand substitutes {stack} and {app} in the template and returns the app
// directory. An empty template means Default. Convention (enforced by the
// config loader): the last path segment is the app directory.
func Expand(layout, stack, app string) string {
	layout = withDefault(layout)
	return path.Clean(strings.NewReplacer("{stack}", stack, "{app}", app).Replace(layout))
}

// StackDir is the directory that holds every app of a stack: the parent of an
// expanded app directory. Listing it and reading <entry>/app.yaml is how the
// inventory discovers apps.
func StackDir(layout, stack string) string {
	return path.Dir(Expand(layout, stack, "app"))
}

// Validate reports whether a layout template is usable. Two rules, both learned
// from shapes that silently produced an empty or duplicated inventory:
//
//   - it must contain {stack}. StackDir is the same directory for every stack
//     without it, so the inventory's per-stack walk would read one directory N
//     times and file one app under N stacks.
//   - its last path segment must contain {app}. StackDir takes the parent of an
//     expanded path, so a template whose app directory is not last leaks the
//     expansion sentinel into the directory the inventory reads.
//
// A decorated last segment is fine: apps/{stack}/{app}-app resolves and lists.
func Validate(tmpl string) error {
	tmpl = withDefault(tmpl)
	if !strings.Contains(tmpl, "{stack}") {
		return fmt.Errorf("layout %q must contain the {stack} token", tmpl)
	}
	if !strings.Contains(path.Base(tmpl), "{app}") {
		return fmt.Errorf("layout %q must contain the {app} token in its last path segment", tmpl)
	}
	return nil
}

// withDefault treats an empty template as Default.
func withDefault(tmpl string) string {
	if tmpl == "" {
		return Default
	}
	return tmpl
}
