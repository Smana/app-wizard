// Package layout owns the file-layout template that decides where an app's
// manifests live in the GitOps repo. Both writers (the PR service) and readers
// (the inventory) go through it, so they cannot disagree.
package layout

import (
	"path"
	"strings"
)

// Default reproduces the historical layout.
const Default = "apps/{stack}/{app}"

// Expand substitutes {stack} and {app} in the template and returns the app
// directory. An empty template means Default. Convention (enforced by the
// config loader): the last path segment is the app directory.
func Expand(layout, stack, app string) string {
	if layout == "" {
		layout = Default
	}
	return path.Clean(strings.NewReplacer("{stack}", stack, "{app}", app).Replace(layout))
}

// StackDir is the directory that holds every app of a stack: the parent of an
// expanded app directory. Listing it and reading <entry>/app.yaml is how the
// inventory discovers apps.
func StackDir(layout, stack string) string {
	return path.Dir(Expand(layout, stack, "app"))
}
