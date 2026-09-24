package provider

import (
	"testing"

	"github.com/lonelyman0108/cfst-ddns/internal/schema"
)

func TestRegistry(t *testing.T) {
	want := []string{"cloudflare", "dnspod", "tencentcloud", "alidns", "huaweicloud", "godaddy"}
	metas := Metas()
	if len(metas) != len(want) {
		t.Fatalf("want %d providers, got %d", len(want), len(metas))
	}
	for i, m := range metas {
		if m.Type != want[i] {
			t.Fatalf("order[%d] = %s, want %s", i, m.Type, want[i])
		}
		if m.Name == "" || m.DocsURL == "" {
			t.Errorf("%s: missing name or docs url", m.Type)
		}
		cfg := schema.Config{}
		for _, f := range m.Fields {
			if f.Type == schema.Password && !f.Secret {
				t.Errorf("%s.%s: password field should be secret", m.Type, f.Key)
			}
			if f.Type == schema.Select && f.Default != "" {
				ok := false
				for _, o := range f.Options {
					ok = ok || o.Value == f.Default
				}
				if !ok {
					t.Errorf("%s.%s: default %q not in options", m.Type, f.Key, f.Default)
				}
			}
			if f.Required && f.Default == "" {
				cfg[f.Key] = "x"
			}
		}
		if _, err := New(m.Type, cfg); err != nil {
			t.Errorf("%s: New with required fields: %v", m.Type, err)
		}
	}
}
