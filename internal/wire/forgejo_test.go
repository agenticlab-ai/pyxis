package wire

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/plangate/dialect"
)

type forgeRoundTrip func(*http.Request) (*http.Response, error)

func (f forgeRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func forgeResponse(r *http.Request, status int, b string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"X-Total-Count": []string{"0"}}, Body: io.NopCloser(strings.NewReader(b)), Request: r}
}

func TestForgejoWiringRefusesBeforeAnyWriteWhenIdentityIsUnavailable(t *testing.T) {
	profile := forgeProfile()
	bound, err := profile.CodeHost("")
	if err != nil {
		t.Fatal(err)
	}
	op := Op{Name: OpEnsureLabels, Transport: TransportForgejo, Connection: bound, Slug: bound.Repository}
	for _, defect := range []string{"repository", "token", "identity"} {
		broken := op
		var options []ExecOption
		switch defect {
		case "repository":
			broken.Slug = "not-a-coordinate"
		case "token":
			t.Setenv(bound.TokenEnv, "")
		case "identity":
			options = append(options, WithForgejoClientFactory(func(integrations.ResolvedRepository) (*forgejo.Client, error) {
				return forgejo.NewWithHTTP(bound.BaseURL, "test", &http.Client{Transport: forgeRoundTrip(func(r *http.Request) (*http.Response, error) {
					if r.Method != http.MethodGet || r.URL.Path != "/api/v1/user" {
						t.Fatal("identity refusal reached repository mutation")
					}
					return forgeResponse(r, 403, `{}`), nil
				})})
			}))
		}
		if audit, err := Execute(t.Context(), []Op{broken}, nil, options...); err == nil || len(audit.Operations) != 0 {
			t.Fatalf("%s refusal reached write phase: %#v %v", defect, audit, err)
		}
	}
	t.Setenv(bound.TokenEnv, "test")
	if _, err := defaultForgejoClient(bound); err != nil {
		t.Fatal(err)
	}
	x := &executor{now: time.Now, forgejoClient: func(integrations.ResolvedRepository) (*forgejo.Client, error) {
		return nil, errors.New("credential expired")
	}}
	if _, err := x.runForgejoOp(t.Context(), op, &walkState{}); err == nil {
		t.Fatal("credential loss between preflight and write was ignored")
	}
}

func TestWiringExistingRepositoryUsesSelectedAccountWithoutRecreation(t *testing.T) {
	profile := forgeProfile()
	bound, err := profile.CodeHost("")
	if err != nil {
		t.Fatal(err)
	}
	factory := func(integrations.ResolvedRepository) (*forgejo.Client, error) {
		return forgejo.NewWithHTTP(bound.BaseURL, "test", &http.Client{Transport: forgeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodGet {
				t.Fatal("existing native repository was recreated")
			}
			body := `{"id":1}`
			if r.URL.Path == "/api/v1/user" {
				body = `{"id":2,"login":"assistant"}`
			}
			return forgeResponse(r, 200, body), nil
		})})
	}
	op := Op{Name: OpCreateRepo, Transport: TransportForgejo, Connection: bound, Slug: bound.Repository}
	audit, err := Execute(t.Context(), []Op{op}, nil, WithForgejoClientFactory(factory))
	if err != nil || audit.Operations[0].Outcome != OutcomeSkipped {
		t.Fatalf("existing native repository %#v %v", audit, err)
	}
	dir := t.TempDir()
	stub := "#!/bin/sh\n[ \"$1\" = --version ] && exit 0\n[ \"$GH_HOST\" = github.com ] && [ \"$GH_TOKEN\" = selected-code ] || exit 2\n[ \"$1\" = repo ] && [ \"$2\" = view ] || exit 3\nprintf '{}\\n'\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(stub), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("GH_HOST", "wrong.example")
	t.Setenv("GH_TOKEN", "ambient")
	t.Setenv("TEST_WIRING_CODE", "selected-code")
	op.Transport = TransportGH
	op.Connection = integrations.ResolvedRepository{Provider: integrations.GitHub, BaseURL: "https://github.com", Repository: "o/code", TokenEnv: "TEST_WIRING_CODE"}
	audit, err = Execute(t.Context(), []Op{op}, GHRunner{})
	if err != nil || audit.Operations[0].Outcome != OutcomeSkipped {
		t.Fatalf("selected GitHub repository %#v %v", audit, err)
	}
	if os.Getenv("GH_TOKEN") != "ambient" || os.Getenv("GH_HOST") != "wrong.example" {
		t.Fatal("wiring changed parent authentication")
	}
}

func forgeProfile() integrations.Config {
	c := integrations.Config{SchemaVersion: integrations.SchemaVersion, Connections: map[string]integrations.Connection{"local": {Provider: integrations.Forgejo, BaseURL: "http://127.0.0.1:3000", TokenEnv: "TEST_WIRE_TOKEN"}}}
	c.Bindings.Tasks = integrations.Binding{Connection: "local", Repository: "owner/tasks"}
	c.Bindings.CodeHost = integrations.Binding{Connection: "local", Repository: "owner/code"}
	return c
}

func TestForgejoTableRoutesTaskAndPolicySeparately(t *testing.T) {
	cfg := Config{Repo: config.Config{Knobs: config.Knobs{Tracker: "forgejo", CIMode: "both", Wiring: config.Wiring{Repo: "create", Protection: "checks+review"}}}, OnboardingItems: []WorkItem{{Slug: "docs", Title: "write docs", Body: "instructions"}}}
	ops, err := TableForIntegrations(cfg, forgeProfile())
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 6 {
		t.Fatalf("%#v", ops)
	}
	for _, op := range ops {
		if op.Name == OpPushWorkflow {
			continue
		}
		want := "owner/tasks"
		if op.Name == OpCreateRepo || op.Name == OpSetProtection {
			want = "owner/code"
		}
		if op.Slug != want || op.Transport != TransportForgejo {
			t.Fatalf("wrong host for %#v", op)
		}
		if strings.Contains(previewEntry(op), "gh ") {
			t.Fatalf("Forgejo preview contains gh: %s", previewEntry(op))
		}
	}
	local := forgeProfile()
	local.Bindings.Tasks = integrations.Binding{Provider: "carina", Root: "work-items"}
	ops, err = TableForIntegrations(cfg, local)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		if op.Name == OpEnsureLabels || op.Name == OpWelcomeItem || op.Name == OpOnboardingItems {
			t.Fatalf("Carina task operation sent to host: %#v", op)
		}
	}
}

func TestExplicitCIModesSelectRequiredContexts(t *testing.T) {
	for _, provider := range []string{integrations.Forgejo, integrations.GitHub} {
		for _, explicitMode := range []bool{false, true} {
			for _, mode := range []string{"local", "actions", "both"} {
				t.Run(provider+"/"+mode+"/explicit="+strconv.FormatBool(explicitMode), func(t *testing.T) {
					profile := forgeProfile()
					hosted := ForgejoRequiredCheckContext
					if provider == integrations.GitHub {
						profile.Connections["local"] = integrations.Connection{Provider: integrations.GitHub}
						hosted = RequiredCheckContext
					}
					legacyMode := mode
					if explicitMode {
						profile.CI.Mode = mode
						legacyMode = "local"
						if mode == "local" {
							legacyMode = "both"
						}
					}
					want := []string{hosted}
					switch mode {
					case "local":
						want = []string{"pyxis/local-gate"}
					case "both":
						want = append(want, "pyxis/local-gate")
					}
					ops, err := TableForIntegrations(Config{Repo: config.Config{Knobs: config.Knobs{Tracker: provider, CIMode: legacyMode, Wiring: config.Wiring{Protection: "required-checks"}}}}, profile)
					if err != nil {
						t.Fatal(err)
					}
					found, workflow := false, false
					for _, op := range ops {
						if op.Name == OpPushWorkflow {
							workflow = true
						}
						if op.Name == OpSetProtection {
							found = true
							if strings.Join(op.Contexts, ",") != strings.Join(want, ",") {
								t.Fatalf("contexts %v, want %v", op.Contexts, want)
							}
						}
					}
					if !found || workflow != (mode != "local") {
						t.Fatalf("protection=%v workflow=%v mode=%s", found, workflow, mode)
					}
				})
			}
		}
	}
}

func TestForgejoRequiredContextMatchesInstalledWorkflow(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(payload.ForgejoWorkflow)))
	if err != nil {
		t.Fatal(err)
	}
	workflow := regexp.MustCompile(`(?m)^name: ([A-Za-z0-9_-]+)$`).FindSubmatch(b)
	job := regexp.MustCompile(`(?m)^jobs:\n  ([A-Za-z0-9_-]+):`).FindSubmatch(b)
	if len(workflow) != 2 || len(job) != 2 || !regexp.MustCompile(`(?m)^  pull_request:$`).Match(b) {
		t.Fatal("installed Forgejo workflow must declare a workflow, job and pull_request trigger")
	}
	if regexp.MustCompile(`(?m)^    name:`).Match(b) {
		t.Fatal("installed Forgejo job overrides name; required context must track the emitted job name")
	}
	got := string(workflow[1]) + " / " + string(job[1]) + " (pull_request)"
	if got != ForgejoRequiredCheckContext {
		t.Fatalf("installed Forgejo context %q differs from protection %q", got, ForgejoRequiredCheckContext)
	}
}

func TestForgejoNativeWiringAndReadback(t *testing.T) {
	profile := forgeProfile()
	target, err := profile.Resolve(profile.Bindings.CodeHost)
	if err != nil {
		t.Fatal(err)
	}
	protectionWrites := 0
	var savedPolicy forgejoProtection
	factory := func(integrations.ResolvedRepository) (*forgejo.Client, error) {
		return forgejo.NewWithHTTP(target.BaseURL, "test", &http.Client{Transport: forgeRoundTrip(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/api/v1/user":
				return forgeResponse(r, 200, `{"id":1,"login":"owner"}`), nil
			case "/api/v1/repos/owner/code/branch_protections/main":
				if protectionWrites > 0 {
					b, err := json.Marshal(savedPolicy)
					if err != nil {
						t.Fatal(err)
					}
					return forgeResponse(r, 200, string(b)), nil
				}
				return forgeResponse(r, 404, `{}`), nil
			case "/api/v1/repos/owner/code/branch_protections":
				if r.Method != "POST" {
					t.Fatalf("%s", r.Method)
				}
				var p forgejoProtection
				if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
					t.Fatal(err)
				}
				if p.RequiredApprovals != 1 || p.EnablePush || !p.EnableStatusCheck || len(p.StatusCheckContexts) != 1 || p.StatusCheckContexts[0] != RequiredCheckContext || !p.DismissStaleApprovals {
					t.Fatalf("unsafe policy %#v", p)
				}
				protectionWrites++
				savedPolicy = p
				return forgeResponse(r, 201, `{}`), nil
			default:
				t.Fatalf("unexpected native path %s", r.URL)
				return nil, nil
			}
		})})
	}
	op := Op{Name: OpSetProtection, Transport: TransportForgejo, Connection: target, Slug: target.Repository, Branch: "main", Contexts: []string{RequiredCheckContext}, RequireReview: true}
	audit, err := Execute(context.Background(), []Op{op}, nil, WithForgejoClientFactory(factory))
	if err != nil || audit.Partial || protectionWrites != 1 {
		t.Fatalf("audit %#v err %v", audit, err)
	}
}

func TestForgejoCreationFailureDoesNotDeferIndependentTracker(t *testing.T) {
	profile := forgeProfile()
	host, err := profile.Resolve(profile.Bindings.CodeHost)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := profile.Resolve(profile.Bindings.Tasks)
	if err != nil {
		t.Fatal(err)
	}
	labelsWritten := 0
	factory := func(target integrations.ResolvedRepository) (*forgejo.Client, error) {
		return forgejo.NewWithHTTP(target.BaseURL, "test", &http.Client{Transport: forgeRoundTrip(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/api/v1/user":
				return forgeResponse(r, 200, `{"id":1,"login":"owner"}`), nil
			case "/api/v1/repos/owner/code":
				return forgeResponse(r, 404, `{}`), nil
			case "/api/v1/user/repos":
				return forgeResponse(r, 403, `{}`), nil
			case "/api/v1/repos/owner/tasks/labels":
				if r.Method == http.MethodGet {
					return forgeResponse(r, 200, `[]`), nil
				}
				labelsWritten++
				return forgeResponse(r, 201, `{}`), nil
			default:
				t.Fatalf("unexpected request %s %s", r.Method, r.URL)
				return nil, nil
			}
		})})
	}
	ops := []Op{
		{Name: OpCreateRepo, Transport: TransportForgejo, Connection: host, Slug: host.Repository},
		{Name: OpEnsureLabels, Transport: TransportForgejo, Connection: tasks, Slug: tasks.Repository, Labels: []dialect.Label{{Name: "ready", Color: "123456"}}},
		{Name: OpSetProtection, Transport: TransportForgejo, Connection: host, Slug: host.Repository, Branch: "main", Contexts: []string{RequiredCheckContext}},
	}
	audit, err := Execute(context.Background(), ops, nil, WithForgejoClientFactory(factory))
	if err != nil || labelsWritten != 1 || !audit.Partial || audit.Operations[1].Outcome != OutcomeApplied || audit.Operations[2].Outcome != OutcomeHandedOff {
		t.Fatalf("audit %#v labels %d err %v", audit, labelsWritten, err)
	}
	if !strings.Contains(audit.HandOff, "/branch_protections") {
		t.Fatalf("missing deferred policy remedy: %s", audit.HandOff)
	}
}

func TestForgejoOnboardingRefusesDuplicateMarker(t *testing.T) {
	profile := forgeProfile()
	target, err := profile.Resolve(profile.Bindings.Tasks)
	if err != nil {
		t.Fatal(err)
	}
	factory := func(integrations.ResolvedRepository) (*forgejo.Client, error) {
		return forgejo.NewWithHTTP(target.BaseURL, "test", &http.Client{Transport: forgeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodGet {
				t.Fatalf("ambiguous marker caused write: %s", r.Method)
			}
			if r.URL.Path == "/api/v1/user" {
				return forgeResponse(r, 200, `{"id":1,"login":"owner"}`), nil
			}
			issues := []forgejoIssue{{Number: 1, Body: WelcomeMarker}, {Number: 2, Body: WelcomeMarker}}
			b, err := json.Marshal(issues)
			if err != nil {
				t.Fatal(err)
			}
			return forgeResponse(r, 200, string(b)), nil
		})})
	}
	audit, err := Execute(context.Background(), []Op{{Name: OpWelcomeItem, Transport: TransportForgejo, Connection: target, Slug: target.Repository, Title: "Welcome", Body: WelcomeMarker}}, nil, WithForgejoClientFactory(factory))
	if err == nil || !strings.Contains(err.Error(), "multiple issues") || !audit.Partial {
		t.Fatalf("audit %#v err %v", audit, err)
	}
}

func TestForgejoPermissionHandoffDoesNotRetryWrite(t *testing.T) {
	profile := forgeProfile()
	target, err := profile.Resolve(profile.Bindings.CodeHost)
	if err != nil {
		t.Fatal(err)
	}
	writes := 0
	factory := func(integrations.ResolvedRepository) (*forgejo.Client, error) {
		return forgejo.NewWithHTTP(target.BaseURL, "never-print-me", &http.Client{Transport: forgeRoundTrip(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/v1/user" {
				return forgeResponse(r, 200, `{"id":1,"login":"owner"}`), nil
			}
			if r.Method == "GET" {
				return forgeResponse(r, 200, `{}`), nil
			}
			writes++
			return forgeResponse(r, 403, `{"message":"never-print-me"}`), nil
		})})
	}
	op := Op{Name: OpSetProtection, Transport: TransportForgejo, Connection: target, Slug: target.Repository, Branch: "main", Contexts: []string{RequiredCheckContext}}
	audit, err := Execute(context.Background(), []Op{op}, nil, WithForgejoClientFactory(factory))
	if err != nil || !audit.Partial || writes != 1 || !strings.Contains(audit.HandOff, "curl --fail") || strings.Contains(audit.HandOff, "never-print-me") || !strings.Contains(audit.HandOff, "${TEST_WIRE_TOKEN}") {
		t.Fatalf("writes %d audit %#v err %v", writes, audit, err)
	}
}

func TestForgejoHandoffKeepsOnlyPendingWrites(t *testing.T) {
	for _, operation := range []string{OpEnsureLabels, OpOnboardingItems} {
		t.Run(operation, func(t *testing.T) {
			profile := forgeProfile()
			target, err := profile.Resolve(profile.Bindings.Tasks)
			if err != nil {
				t.Fatal(err)
			}
			var writes []string
			factory := func(integrations.ResolvedRepository) (*forgejo.Client, error) {
				return forgejo.NewWithHTTP(target.BaseURL, "secret", &http.Client{Transport: forgeRoundTrip(func(r *http.Request) (*http.Response, error) {
					if r.URL.Path == "/api/v1/user" {
						return forgeResponse(r, 200, `{"id":1,"login":"owner"}`), nil
					}
					if r.Method == http.MethodGet {
						body := `[{"name":"existing"}]`
						if operation == OpOnboardingItems {
							body = `[{"body":"` + OnboardingMarker("existing") + `","html_url":"https://example.test/existing"}]`
						}
						return forgeResponse(r, 200, body), nil
					}
					b, readErr := io.ReadAll(r.Body)
					if readErr != nil {
						t.Fatal(readErr)
					}
					writes = append(writes, string(b))
					if len(writes) == 2 {
						return forgeResponse(r, 403, `{}`), nil
					}
					return forgeResponse(r, 201, `{"html_url":"https://example.test/completed"}`), nil
				})})
			}
			op := Op{Name: operation, Transport: TransportForgejo, Connection: target, Slug: target.Repository}
			for _, name := range []string{"existing", "completed", "failed", "remaining"} {
				op.Labels = append(op.Labels, dialect.Label{Name: name, Color: "aaaaaa"})
				op.Items = append(op.Items, WorkItem{Slug: name, Title: name, Body: OnboardingMarker(name)})
			}
			audit, err := Execute(context.Background(), []Op{op}, nil, WithForgejoClientFactory(factory))
			if err != nil || !audit.Partial || len(writes) != 2 {
				t.Fatalf("writes=%v audit=%#v err=%v", writes, audit, err)
			}
			for _, name := range []string{"failed", "remaining"} {
				if !strings.Contains(audit.HandOff, `"`+name+`"`) {
					t.Errorf("handoff omits %s: %s", name, audit.HandOff)
				}
			}
			for _, name := range []string{"existing", "completed"} {
				if strings.Contains(audit.HandOff, `"`+name+`"`) {
					t.Errorf("handoff would replay %s: %s", name, audit.HandOff)
				}
			}
		})
	}
}
