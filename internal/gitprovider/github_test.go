package gitprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-github/v89/github"
)

// The GitHub provider is the path that opens pull requests as the signed-in
// user, and it had no test at all — which is how the go-github v66 -> v89
// migration could have compiled cleanly while sending the wrong requests. v89
// reworked exactly the calls used here: CreateRef and UpdateRef take dedicated
// request structs instead of a *Reference, and UpdateRef addresses the ref by
// name in the URL rather than carrying it in the body.
//
// So assert on the wire: method, path, and JSON body of every request the
// provider makes. A signature change the compiler accepts but GitHub would
// reject fails here.

type capturedRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

// newTestGitHub returns a provider whose client is pointed at a stub GitHub,
// plus the recorded requests. routes maps "METHOD /path" to a JSON response
// body; an unrouted request fails the test rather than 404-ing silently.
func newTestGitHub(t *testing.T, routes map[string]string) (*GitHub, *[]capturedRequest) {
	t.Helper()
	var got []capturedRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("request body for %s %s is not JSON: %v", r.Method, r.URL.Path, err)
			}
		}
		got = append(got, capturedRequest{Method: r.Method, Path: r.URL.Path, Body: body})

		key := r.Method + " " + r.URL.Path
		resp, ok := routes[key]
		if !ok {
			t.Errorf("unexpected request %s (no stub route)", key)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, resp)
	}))
	t.Cleanup(srv.Close)

	base := srv.URL + "/"
	client, err := github.NewClient(github.WithAuthToken("test-token"), github.WithURLs(&base, &base))
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	return &GitHub{client: client, owner: "acme", repo: "gitops"}, &got
}

func TestCreateBranchSendsCreateRefPayload(t *testing.T) {
	g, got := newTestGitHub(t, map[string]string{
		"GET /repos/acme/gitops/git/ref/heads/main": `{"ref":"refs/heads/main","object":{"sha":"basesha"}}`,
		"POST /repos/acme/gitops/git/refs":          `{"ref":"refs/heads/feature"}`,
	})

	if err := g.CreateBranch(context.Background(), "main", "feature"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	reqs := *got
	if len(reqs) != 2 {
		t.Fatalf("made %d requests, want 2: %+v", len(reqs), reqs)
	}

	// v89's CreateRef body is {ref, sha} as plain strings. The old *Reference
	// shape nested the sha under "object", which GitHub would reject.
	create := reqs[1]
	if create.Method != "POST" || create.Path != "/repos/acme/gitops/git/refs" {
		t.Errorf("create = %s %s", create.Method, create.Path)
	}
	if create.Body["ref"] != "refs/heads/feature" {
		t.Errorf(`body["ref"] = %v, want "refs/heads/feature"`, create.Body["ref"])
	}
	if create.Body["sha"] != "basesha" {
		t.Errorf(`body["sha"] = %v, want the base branch head "basesha"`, create.Body["sha"])
	}
	if _, nested := create.Body["object"]; nested {
		t.Errorf("body still uses the pre-v89 nested object shape: %+v", create.Body)
	}
}

func TestCommitFilesSendsTreeCommitAndRefUpdate(t *testing.T) {
	g, got := newTestGitHub(t, map[string]string{
		"GET /repos/acme/gitops/git/ref/heads/feature":    `{"ref":"refs/heads/feature","object":{"sha":"headsha"}}`,
		"GET /repos/acme/gitops/git/commits/headsha":      `{"sha":"headsha","tree":{"sha":"treesha"}}`,
		"POST /repos/acme/gitops/git/trees":               `{"sha":"newtree"}`,
		"POST /repos/acme/gitops/git/commits":             `{"sha":"newcommit"}`,
		"PATCH /repos/acme/gitops/git/refs/heads/feature": `{"ref":"refs/heads/feature"}`,
	})

	files := []File{
		{Path: "apps/demo/app.yaml", Content: []byte("kind: App\n")},
		{Path: "apps/demo/old.yaml", Delete: true},
	}
	if err := g.CommitFiles(context.Background(), "feature", files, "add demo"); err != nil {
		t.Fatalf("CommitFiles: %v", err)
	}

	reqs := *got
	if len(reqs) != 5 {
		t.Fatalf("made %d requests, want 5: %+v", len(reqs), reqs)
	}

	tree := reqs[2]
	if tree.Body["base_tree"] != "treesha" {
		t.Errorf(`tree base_tree = %v, want "treesha"`, tree.Body["base_tree"])
	}
	entries, _ := tree.Body["tree"].([]any)
	if len(entries) != 2 {
		t.Fatalf("tree has %d entries, want 2", len(entries))
	}
	// A delete entry must send an EXPLICIT null sha — not omit the field.
	// go-github marshals a TreeEntry whose SHA and Content are both nil through
	// treeEntryWithFileDelete, which drops `omitempty` from sha for exactly this
	// reason: GitHub removes the path only when it sees `"sha": null`. Omitting
	// the key instead leaves the file in the tree, so this is the assertion that
	// catches a regression in the delete path.
	del, _ := entries[1].(map[string]any)
	sha, hasSHA := del["sha"]
	if !hasSHA {
		t.Errorf("delete entry must send an explicit sha key, got %+v", del)
	}
	if sha != nil {
		t.Errorf("delete entry sha = %v, want null", sha)
	}
	if _, hasContent := del["content"]; hasContent {
		t.Errorf("delete entry should omit content, got %+v", del)
	}

	// The non-delete entry is the mirror image: content present, sha absent.
	add, _ := entries[0].(map[string]any)
	if add["content"] != "kind: App\n" {
		t.Errorf("add entry content = %v", add["content"])
	}
	if _, hasSHA := add["sha"]; hasSHA {
		t.Errorf("add entry should omit sha, got %+v", add)
	}

	commit := reqs[3]
	if commit.Body["message"] != "add demo" {
		t.Errorf("commit message = %v", commit.Body["message"])
	}

	// The migration's sharpest edge: v89 puts the ref name in the URL and only
	// the sha in the body. Passing the old mutated *Reference would have sent
	// the sha nested under "object" to a URL with no ref in it.
	update := reqs[4]
	if update.Method != "PATCH" {
		t.Errorf("update method = %s, want PATCH", update.Method)
	}
	if update.Path != "/repos/acme/gitops/git/refs/heads/feature" {
		t.Errorf("update path = %s, want the ref name in the URL", update.Path)
	}
	if update.Body["sha"] != "newcommit" {
		t.Errorf(`update body["sha"] = %v, want "newcommit"`, update.Body["sha"])
	}
	if _, nested := update.Body["object"]; nested {
		t.Errorf("update body still uses the pre-v89 nested object shape: %+v", update.Body)
	}
}

func TestOpenPRAndCommentPR(t *testing.T) {
	g, got := newTestGitHub(t, map[string]string{
		"POST /repos/acme/gitops/pulls":             `{"number":7,"html_url":"https://github.com/acme/gitops/pull/7"}`,
		"POST /repos/acme/gitops/issues/7/comments": `{"id":1}`,
	})

	pr, err := g.OpenPR(context.Background(), "main", "feature", "Add demo", "body text")
	if err != nil {
		t.Fatalf("OpenPR: %v", err)
	}
	if pr.Number != 7 || pr.URL != "https://github.com/acme/gitops/pull/7" {
		t.Errorf("pr = %+v", pr)
	}

	if err := g.CommentPR(context.Background(), 7, "rendered output"); err != nil {
		t.Fatalf("CommentPR: %v", err)
	}

	reqs := *got
	open := reqs[0]
	for key, want := range map[string]string{
		"title": "Add demo",
		"head":  "feature",
		"base":  "main",
		"body":  "body text",
	} {
		if open.Body[key] != want {
			t.Errorf("pull body[%q] = %v, want %q", key, open.Body[key], want)
		}
	}
	if reqs[1].Body["body"] != "rendered output" {
		t.Errorf("comment body = %v", reqs[1].Body)
	}
}

func TestReadFileNotFoundMapsToErrNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	t.Cleanup(srv.Close)

	base := srv.URL + "/"
	client, err := github.NewClient(github.WithAuthToken("t"), github.WithURLs(&base, &base))
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	g := &GitHub{client: client, owner: "acme", repo: "gitops"}

	if _, _, err := g.ReadFile(context.Background(), "main", "missing.yaml"); err != ErrNotFound {
		t.Errorf("ReadFile err = %v, want ErrNotFound", err)
	}
	if _, err := g.ReadTree(context.Background(), "main", "missing/"); err != ErrNotFound {
		t.Errorf("ReadTree err = %v, want ErrNotFound", err)
	}
}

func TestCurrentUser(t *testing.T) {
	g, _ := newTestGitHub(t, map[string]string{
		"GET /user": `{"login":"octocat","avatar_url":"https://example.test/a.png","name":"Mona"}`,
	})
	u, err := g.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	if u.Login != "octocat" || u.Name != "Mona" || u.AvatarURL != "https://example.test/a.png" {
		t.Errorf("user = %+v", u)
	}
}

func TestNewGitHubReturnsUsableClient(t *testing.T) {
	g, err := NewGitHub(context.Background(), "tok", "acme", "gitops")
	if err != nil {
		t.Fatalf("NewGitHub: %v", err)
	}
	if g.client == nil || g.owner != "acme" || g.repo != "gitops" {
		t.Errorf("provider = %+v", g)
	}
}
