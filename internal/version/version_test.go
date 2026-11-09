package version

import "testing"

func TestForUserAgent(t *testing.T) {
	cases := []struct{ injected, module, want string }{
		{"1.22.0", "(devel)", "1.22.0"},
		{"1.22.0", "v1.20.0", "1.22.0"},
		{"dev", "v1.22.0", "1.22.0"},
		{"dev", "v1.22.1-0.20261108120000-2d97a0537ed2+dirty", "1.22.1-0.20261108120000-2d97a0537ed2+dirty"},
		{"dev", "(devel)", "dev"},
		{"dev", "", "dev"},
	}
	for _, c := range cases {
		if got := forUserAgent(c.injected, c.module); got != c.want {
			t.Errorf("forUserAgent(%q, %q) = %q, want %q", c.injected, c.module, got, c.want)
		}
	}
}

func TestUserAgentPrefix(t *testing.T) {
	if got := UserAgent(); len(got) <= len("CubeCLI/") || got[:8] != "CubeCLI/" {
		t.Errorf("UserAgent() = %q", got)
	}
}
