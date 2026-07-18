package pr

import "testing"

func TestNamingGate(t *testing.T) {
	s3 := map[string]any{"s3Bucket": map[string]any{"enabled": true}}
	sql := map[string]any{"sqlInstance": map[string]any{"enabled": true}}
	s3Off := map[string]any{"s3Bucket": map[string]any{"enabled": false}}
	plain := map[string]any{"image": map[string]any{"repository": "x"}}

	cases := []struct {
		name    string
		app     string
		spec    map[string]any
		blocked bool
	}{
		{"s3 unprefixed is blocked", "outline", s3, true},
		{"s3 prefixed is allowed", "xplane-outline", s3, false},
		{"sqlInstance unprefixed is blocked", "outline", sql, true},
		{"sqlInstance prefixed is allowed", "xplane-outline", sql, false},
		{"no AWS backend, unprefixed is allowed", "podinfo", plain, false},
		{"s3 disabled, unprefixed is allowed", "podinfo", s3Off, false},
		{"nil spec is allowed", "podinfo", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ge := NamingGate(c.app, c.spec)
			if c.blocked && ge == nil {
				t.Fatalf("expected NamingGate to block %q with spec %v", c.app, c.spec)
			}
			if !c.blocked && ge != nil {
				t.Fatalf("unexpected block for %q: %s", c.app, ge.Message)
			}
		})
	}
}
