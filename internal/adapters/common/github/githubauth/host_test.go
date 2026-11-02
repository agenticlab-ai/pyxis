package githubauth

import (
	"strings"
	"testing"
)

func TestExplicitHostPreservesGithubAuthAndExcludesEnterprise(t *testing.T) {
	t.Setenv("GH_TOKEN", "github-token")
	t.Setenv("GH_HOST", "enterprise.invalid")
	t.Setenv("GH_ENTERPRISE_TOKEN", "enterprise-token")
	env, err := EnvironmentForHost("", "github.com")
	if err != nil {
		t.Fatal(err)
	}
	values := map[string]string{}
	for _, entry := range env {
		k, v, _ := strings.Cut(entry, "=")
		values[k] = v
	}
	if values["GH_HOST"] != "github.com" || values["GH_TOKEN"] != "github-token" || values["GH_ENTERPRISE_TOKEN"] != "" {
		t.Fatal("explicit host was not scoped")
	}
	if _, err := EnvironmentForHost("", "enterprise.invalid"); err == nil {
		t.Fatal("accepted unsupported host")
	}
}
