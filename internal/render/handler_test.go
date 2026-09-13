package render

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Smana/app-wizard/internal/api"
)

// fakeStacks mirrors apps/stacks.yaml's shape: a stack always carries the
// namespace its claims land in.
type fakeStacks struct{}

func (fakeStacks) Stack(_ context.Context, name string) (api.Stack, bool, error) {
	if name == "team-a" {
		return api.Stack{Name: "team-a", Namespace: "apps-team-a", OwnerTeam: "team-a"}, true, nil
	}
	return api.Stack{}, false, nil
}

func (fakeStacks) GVK(_ context.Context) (api.GVK, error) {
	return api.GVK{APIVersion: "cloud.ogenki.io/v1alpha1", Kind: "App"}, nil
}

func postPreview(t *testing.T, r Renderer, body api.RenderPreviewRequest) api.RenderPreviewResponse {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/render-preview", bytes.NewReader(raw))
	Handler(r, fakeStacks{}, true).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp api.RenderPreviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// A claim with no stack has no namespace, and `crossplane render` does not
// default metadata.namespace the way the API server does. The App Composition
// then read Undefined and died mid-render:
//
//	invalid value 'UndefinedType' to load attribute 'startswith'
//
// Rejecting it here is cheaper than rendering it, and the message tells the
// developer what to do instead of surfacing a KCL stack trace.
func TestPreviewWithoutAStackIsRejectedBeforeRendering(t *testing.T) {
	fake := &FakeRenderer{}
	resp := postPreview(t, fake, api.RenderPreviewRequest{Name: "myapp"})

	if resp.OK {
		t.Fatal("OK = true, want false: a stack-less preview has no namespace to render into")
	}
	if !strings.Contains(strings.ToLower(resp.Error), "stack") {
		t.Errorf("error = %q, want it to name the missing stack", resp.Error)
	}
	if len(fake.Calls) != 0 {
		t.Errorf("renderer ran %d time(s), want 0 — the claim should never have been built", len(fake.Calls))
	}
}

// The namespace is not decoration: every resource the Composition renders is
// placed in it, and the route block reads it to detect preview environments.
func TestPreviewClaimCarriesTheStackNamespace(t *testing.T) {
	fake := &FakeRenderer{Resources: []api.RenderedResource{{Kind: "Deployment", Name: "myapp"}}}
	resp := postPreview(t, fake, api.RenderPreviewRequest{Name: "myapp", Stack: "team-a"})

	if !resp.OK {
		t.Fatalf("OK = false, error = %q", resp.Error)
	}
	if len(fake.Calls) != 1 {
		t.Fatalf("renderer ran %d time(s), want 1", len(fake.Calls))
	}
	if got := string(fake.Calls[0]); !strings.Contains(got, "namespace: apps-team-a") {
		t.Errorf("claim is missing the stack's namespace:\n%s", got)
	}
}

// An unknown stack is a friendly ok=false too, not an HTTP error — and it must
// not reach the renderer either.
func TestPreviewWithUnknownStackDoesNotRender(t *testing.T) {
	fake := &FakeRenderer{}
	resp := postPreview(t, fake, api.RenderPreviewRequest{Name: "myapp", Stack: "nope"})

	if resp.OK {
		t.Fatal("OK = true, want false for an unknown stack")
	}
	if len(fake.Calls) != 0 {
		t.Errorf("renderer ran %d time(s), want 0", len(fake.Calls))
	}
}
