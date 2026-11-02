package lock

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

const maxDescriptionBytes = 60000 // below the code host's 65,536-byte limit

// Request is the exact-version approval view. Complete contract texts remain
// linked as literal Documents in the same proposal.
func Request(result compile.Result, sources readiness.Sources, verdict readiness.Verdict, repoURL, head string, documents map[string]string) (string, error) {
	return requestWith(result, sources, verdict, repoURL, head, documents, OtherChanges{})
}

// requestWith adds what the proposal carries beside the compiled contract.
func requestWith(result compile.Result, sources readiness.Sources, verdict readiness.Verdict, repoURL, head string, documents map[string]string, other OtherChanges) (string, error) {
	u, err := url.Parse(repoURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("code-host repository URL %q is not plain HTTPS", repoURL)
	}
	link := func(label, file string) string {
		return fmt.Sprintf("[%s](%s)", label, strings.TrimSuffix(repoURL, "/")+"/blob/"+head+"/"+path.Clean(file))
	}
	return renderRequest(result, sources, verdict, head, documents, other, maxDescriptionBytes, link)
}

// LocalRequest presents the same readiness and change report with retained Git
// object addresses. It never manufactures a hosted repository or PR identity.
func LocalRequest(result compile.Result, sources readiness.Sources, verdict readiness.Verdict, head string, documents map[string]string) (string, error) {
	return localRequestWith(result, sources, verdict, head, documents, OtherChanges{})
}

func localRequestWith(result compile.Result, sources readiness.Sources, verdict readiness.Verdict, head string, documents map[string]string, other OtherChanges) (string, error) {
	return renderRequest(result, sources, verdict, head, documents, other, 0, func(label, file string) string {
		return fmt.Sprintf("%s: `git show %s:%s`", label, head, path.Clean(file))
	})
}

func renderRequest(result compile.Result, sources readiness.Sources, verdict readiness.Verdict, head string, documents map[string]string, other OtherChanges, limit int, link func(string, string) string) (string, error) {
	prd, tech := documents["prd"], documents["tech-spec"]
	if prd == "" || tech == "" || head == "" {
		return "", fmt.Errorf("approval request needs the committed PRD and tech spec Documents")
	}
	var prdBody []byte
	for _, entry := range result.Records {
		if entry.Draft.Kind == record.Document && entry.Draft.Subject == "prd/"+result.Feature && entry.Draft.Family == "prd" {
			prdBody = entry.Draft.Body
			break
		}
	}
	readToRule := sectionZero(prdBody)
	if readToRule == "" {
		return "", fmt.Errorf("approval request needs the PRD's §0 Read to rule")
	}
	verdictJSON, err := json.MarshalIndent(verdict, "", "  ")
	if err != nil {
		return "", fmt.Errorf("render readiness verdict: %w", err)
	}
	render := func(short bool) string {
		var b strings.Builder
		b.WriteString("# Lock request — " + result.Feature + "\n\n")
		b.WriteString("Exact draft digest: `" + result.Digest + "` · mainline: `" + result.Mainline + "` · prepared head: `" + head + "`\n\n")
		b.WriteString("Complete contract: " + link("PRD", prd) + " · " + link("tech spec", tech))
		if problems := documents["problems"]; problems != "" {
			b.WriteString(" · " + link("problem rulings", problems))
		}
		b.WriteString("\n\n## Also in this pull request\n\n")
		if len(other.Paths) == 0 {
			b.WriteString("Nothing beyond the compiled contract.\n")
		}
		if other.Plan != "" {
			tasks := "none of its task charters change"
			if len(other.Tasks) > 0 {
				tasks = "changed tasks: " + strings.Join(other.Tasks, ", ")
			}
			b.WriteString("The live plan " + link("plan", other.Plan) + " moves onto this version; " + tasks + ". Merging updates those tasks' issues in place.\n\n")
		}
		for i, p := range other.Paths {
			if short && i == 40 {
				fmt.Fprintf(&b, "- … and %d more\n", len(other.Paths)-i)
				break
			}
			b.WriteString("- `" + p + "`\n")
		}
		b.WriteString("\n## PRD §0 — Read to rule\n\n")
		if short && len(readToRule) > 12000 {
			b.WriteString(clip(readToRule, 12000) + "\n\nThe complete §0 is in the linked PRD.\n")
		} else {
			b.WriteString(readToRule)
			b.WriteString("\n")
		}
		b.WriteString("\n## Readiness\n\n```json\n")
		b.Write(verdictJSON)
		b.WriteString("\n```\n\n## Requirement changes\n\n")
		changes := readiness.AuthorizedChanges(result, sources)
		if len(changes) == 0 {
			b.WriteString("No requirement row changed after its first appearance.\n")
		}
		for _, change := range changes {
			b.WriteString("- **" + change.ID + "** (" + shortCommit(change.FromCommit) + " → " + shortCommit(change.ToCommit) + "): ")
			if change.Unruled || change.Ruling == nil {
				b.WriteString("**unruled**\n")
				continue
			}
			r := change.Ruling
			reason, decision := oneLine(r.Reason), oneLine(r.Change)
			if short {
				reason, decision = clip(reason, 160), clip(decision, 160)
			}
			b.WriteString("P-" + strings.TrimPrefix(r.Problem, "P-") + " · declared author **" + oneLine(r.Author) + "** · " + reason + " · " + decision + "\n")
		}
		b.WriteString("\n## Declined blocking findings\n\n")
		count := 0
		latest := map[string]compile.Review{}
		for _, review := range result.Reviews {
			if previous, ok := latest[review.Part]; !ok || previous.Path < review.Path {
				latest[review.Part] = review
			}
		}
		for _, part := range []string{"prd", "tech-spec"} {
			review, ok := latest[part]
			if !ok {
				continue
			}
			for _, finding := range review.Findings {
				if !finding.Blocking || !strings.HasPrefix(strings.ToLower(finding.Disposition), "declined — ") {
					continue
				}
				count++
				b.WriteString("- **" + review.Path + "#" + finding.ID + "**: " + oneLine(finding.Disposition) + "\n")
			}
		}
		if count == 0 {
			b.WriteString("None.\n")
		}
		return b.String()
	}
	body := render(false)
	if limit > 0 && len(body) > limit {
		body = render(true)
	}
	if limit > 0 && len(body) > limit {
		return "", fmt.Errorf("approval request is %d bytes after compact rendering; reduce §0 or split the draft's rulings before requesting review", len(body))
	}
	return body, nil
}

func sectionZero(body []byte) string {
	lines := strings.Split(string(body), "\n")
	start := -1
	end := len(lines)
	for i, line := range lines {
		if start < 0 && (strings.HasPrefix(line, "## 0.") || line == "## 0") {
			start = i
			continue
		}
		if start >= 0 && strings.HasPrefix(line, "## ") {
			end = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

func shortCommit(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "|", "\\|")), " ")
}

func clip(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit-1] + "…"
}
