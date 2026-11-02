package audit

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func decoded(t *testing.T, event Event) Event {
	t.Helper()
	data, err := encode(event)
	if err != nil {
		t.Fatal(err)
	}
	var result Event
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestReferenceTruncationIsExplicitAndDoesNotMutateCaller(t *testing.T) {
	event := completeEvent()
	location := strings.Repeat("x", 1025)
	event.References = []Reference{{Kind: "report", Location: location, Digest: strings.Repeat("a", 64)}}
	result := decoded(t, event)
	if !result.ReferencesTruncated || len(result.References) != 1 || len(result.References[0].Location) != 1024 || result.References[0].Digest != event.References[0].Digest {
		t.Fatalf("truncation was not explicit: %+v", result)
	}
	if event.References[0].Location != location {
		t.Fatal("caller-owned references changed")
	}
}

func TestDistinctCauseAndCorrelationFieldsRemainDistinct(t *testing.T) {
	prefix := strings.Repeat("a", 64)
	for _, suffix := range []string{"left", "right"} {
		event := completeEvent()
		event.CauseCode = prefix + suffix
		event.JobID = strings.Repeat("j", 257)
		event.InstallationGeneration = strings.Repeat("g", 900)
		event.Claim = &Claim{Owner: strings.Repeat("o", 900), Ref: "refs/maintenance/daily", RecordOID: strings.Repeat("e", 40)}
		event.Route = &Route{Harness: "codex", Profile: strings.Repeat("p", 513)}
		result := decoded(t, event)
		if result.CauseCode != event.CauseCode || result.JobID != event.JobID || result.InstallationGeneration != event.InstallationGeneration || *result.Claim != *event.Claim || *result.Route != *event.Route {
			t.Fatalf("identity changed: %+v", result)
		}
	}
}

func TestUsagePresenceKeepsObservedZeroAndRejectsNegative(t *testing.T) {
	zero, negative := 0, -1
	event := completeEvent()
	event.Usage = &Usage{Tokens: &zero}
	if got := decoded(t, event); got.Usage == nil || got.Usage.Tokens == nil || *got.Usage.Tokens != 0 {
		t.Fatalf("observed zero usage = %+v", got.Usage)
	}
	event.Usage = &Usage{Turns: &negative}
	if _, err := encode(event); err == nil {
		t.Fatal("negative native usage was accepted")
	}
}

func TestUnrepresentableOrCredentialBearingIdentityIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Event)
	}{
		{"job", func(e *Event) { e.JobID = strings.Repeat("j", MaxIdentityBytes+1) }},
		{"cause", func(e *Event) { e.CauseCode = strings.Repeat("c", MaxIdentityBytes+1) }},
		{"claim", func(e *Event) { e.Claim = &Claim{Owner: strings.Repeat("o", MaxIdentityBytes+1)} }},
		{"hash", func(e *Event) {
			e.References = []Reference{{Kind: "report", Location: "file", Digest: strings.Repeat("d", MaxIdentityBytes+1)}}
		}},
		{"route credential", func(e *Event) { e.Route = &Route{Profile: "PASSWORD=private-value"} }},
		{"invalid job UTF-8", func(e *Event) { e.JobID = "job-\xff" }},
		{"invalid date UTF-8", func(e *Event) { e.JobDate = "date-\xff" }},
		{"invalid timezone UTF-8", func(e *Event) { e.Timezone = "timezone-\xff" }},
		{"invalid generation UTF-8", func(e *Event) { e.InstallationGeneration = "generation-\xff" }},
		{"invalid config UTF-8", func(e *Event) { e.ConfigCommit = "commit-\xff" }},
		{"invalid route UTF-8", func(e *Event) { e.Route = &Route{Profile: "route-\xff"} }},
		{"invalid launch UTF-8", func(e *Event) { e.Launch = &Route{Profile: "launch-\xff"} }},
		{"invalid claim UTF-8", func(e *Event) { e.Claim = &Claim{Owner: "owner-\xff"} }},
		{"invalid branch UTF-8", func(e *Event) { e.Delivery = &Delivery{Branch: "branch-\xff"} }},
		{"invalid head UTF-8", func(e *Event) { e.Delivery = &Delivery{Head: "head-\xff"} }},
		{"invalid URL UTF-8", func(e *Event) { e.Delivery = &Delivery{PRURL: "url-\xff"} }},
		{"invalid digest UTF-8", func(e *Event) {
			e.References = []Reference{{Kind: "report", Location: "file", Digest: "digest-\xff"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := completeEvent()
			tc.edit(&event)
			if _, err := encode(event); err == nil {
				t.Fatal("unsafe identity was changed or accepted")
			}
		})
	}
}

func TestReferenceWorkIsBoundedBeforeSanitizationAndEncoding(t *testing.T) {
	event := completeEvent()
	event.References = make([]Reference, 8192)
	for i := range event.References {
		event.References[i] = Reference{Kind: "report", Location: fmt.Sprintf("file-%d", i), Digest: "PASSWORD=not-a-hash"}
	}
	for i := range MaxReferences {
		event.References[i].Digest = strings.Repeat("a", 64)
	}
	cleaned := sanitize(event)
	if len(cleaned.References) != MaxReferences || !cleaned.ReferencesTruncated {
		t.Fatalf("reference work was not bounded: %d", len(cleaned.References))
	}
	result := decoded(t, event)
	if len(result.References) != MaxReferences || !result.ReferencesTruncated || result.References[MaxReferences-1].Location != fmt.Sprintf("file-%d", MaxReferences-1) {
		t.Fatalf("bounded reference prefix changed: %+v", result)
	}
}

func TestPreviouslyTruncatedDescriptionsKeepTheirIndicators(t *testing.T) {
	event := completeEvent()
	event.Summary, event.SummaryTruncated = "retained prefix", true
	event.Cause, event.CauseTruncated = "retained cause prefix", true
	result := decoded(t, event)
	if !result.SummaryTruncated || !result.CauseTruncated || result.Summary != event.Summary || result.Cause != event.Cause {
		t.Fatalf("known truncation was lost: %+v", result)
	}
}

func TestAuthenticatedURLCredentialsAreRemovedBeforeSharedRedaction(t *testing.T) {
	for _, userinfo := range []string{
		"opaqueCredentialCanary123456:x-oauth-basic",
		"username:password-canary",
		"username-credential-only",
		"user:prefix@middle@opaqueCanary",
		"user:prefix@middle@opaqueCanary@anotherCanary@lastCanary",
		"user:percent%40encoded@opaqueCanary",
	} {
		t.Run(userinfo, func(t *testing.T) {
			url := "https://" + userinfo + "@example.com/org/repo.git"
			event := completeEvent()
			event.Summary, event.Cause = "result: "+url, "cause: "+url
			event.Delivery = &Delivery{PRURL: url}
			event.References = []Reference{{Kind: "report", Location: url}}
			data, err := encode(event)
			if err != nil {
				t.Fatal(err)
			}
			for _, credential := range strings.FieldsFunc(userinfo, func(r rune) bool { return r == ':' || r == '@' }) {
				if strings.Contains(string(data), credential) {
					t.Fatalf("credential survived redaction: %s", data)
				}
			}
			var result Event
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			if result.Delivery.PRURL != "https://example.com/org/repo.git" || result.References[0].Location != result.Delivery.PRURL {
				t.Fatalf("safe locator was not preserved: %+v", result)
			}
		})
	}
}

func TestAuthenticatedURLCredentialsAreRemovedFromAppendedEncodedDescriptions(t *testing.T) {
	for _, tc := range []struct {
		name, input, safe, canary string
	}{
		{
			name:   "ordinary literal",
			input:  `https://operator:ordinaryCanary@example.com/org/repo.git`,
			safe:   `https://example.com/org/repo.git`,
			canary: "ordinaryCanary",
		},
		{
			name:   "escaped solidus",
			input:  `git+https:\/\/operator:escapedCanary@example.com\/org\/repo.git`,
			safe:   `git+https:\/\/example.com\/org\/repo.git`,
			canary: "escapedCanary",
		},
		{
			name:   "unicode and escaped solidus",
			input:  `https:\/\u002Foperator:unicodeCanary@example.com\u002forg\/repo.git`,
			safe:   `https:\//example.com/org\/repo.git`,
			canary: "unicodeCanary",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := ignoredRepository(t)
			writer, err := Open(t.Context(), root, quiet())
			if err != nil {
				t.Fatal(err)
			}
			wrapped := `{"remote":"` + tc.input + `","detail":"safe description"}`
			if !json.Valid([]byte(wrapped)) {
				t.Fatalf("fixture is not valid JSON: %s", wrapped)
			}
			event := completeEvent()
			event.Summary, event.Cause = "summary "+wrapped, "cause "+wrapped
			event.References = []Reference{{Kind: "report", Location: tc.input + "#safe-reference"}}
			if _, err := writer.Append(t.Context(), event, Consequential); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(journalPath)))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte(tc.canary)) {
				t.Fatalf("credential reached journal: %s", data)
			}
			var got Event
			if err := json.Unmarshal(bytes.TrimSuffix(data, []byte{'\n'}), &got); err != nil {
				t.Fatal(err)
			}
			if got.Summary != `summary {"remote":"`+tc.safe+`","detail":"safe description"}` ||
				got.Cause != `cause {"remote":"`+tc.safe+`","detail":"safe description"}` ||
				got.References[0].Location != tc.safe+"#safe-reference" {
				t.Fatalf("safe description changed: %+v", got)
			}
		})
	}
}

func TestInvalidUTF8BranchRefusesAppendWithoutChangingJournal(t *testing.T) {
	root := ignoredRepository(t)
	w, err := Open(t.Context(), root, quiet())
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []byte{0xfe, 0xff} {
		branch := "maintenance/" + string([]byte{suffix})
		git(t, root, "check-ref-format", "refs/heads/"+branch)
		event := completeEvent()
		event.Delivery = &Delivery{Branch: branch}
		if _, err := w.Append(t.Context(), event, Consequential); err == nil {
			t.Fatal("append acknowledged an unrepresentable branch identity")
		}
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(journalPath)))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("refused identity changed the journal: %s", data)
	}
}

func TestURLQueryAndFragmentRemainUsefulWithoutUserinfo(t *testing.T) {
	for _, url := range []string{"https://example.com?email=operator@example.org", "https://example.com#operator@example.org"} {
		event := completeEvent()
		event.Delivery = &Delivery{PRURL: url}
		event.References = []Reference{{Kind: "report", Location: url}}
		result := decoded(t, event)
		if result.Delivery.PRURL != url || result.References[0].Location != url {
			t.Fatalf("safe URL locator changed: %+v", result)
		}
	}
}

func TestProviderCredentialsDoNotReachJournalFields(t *testing.T) {
	for _, canary := range []string{
		"sk-" + strings.Repeat("A", 48),
		"sk-proj-" + strings.Repeat("B", 80),
		"sk-ant-api03-" + strings.Repeat("C", 80),
		"sk-svcacct-" + strings.Repeat("D", 80),
		"ASIA" + strings.Repeat("E", 16),
		"gsk_" + strings.Repeat("F", 52),
		"xai-" + strings.Repeat("G", 80),
		"pplx-" + strings.Repeat("H", 48),
		"hf_" + strings.Repeat("I", 48),
		"r8_" + strings.Repeat("J", 48),
		"nvapi-" + strings.Repeat("K", 64),
		"key_" + strings.Repeat("L", 64),
	} {
		event := completeEvent()
		event.Summary, event.Cause = "upstream rejected "+canary, "upstream rejected "+canary
		event.Delivery = &Delivery{PRURL: "https://example.com/" + canary}
		event.References = []Reference{{Kind: "report-" + canary, Location: "https://example.com/" + canary}}
		data, err := encode(event)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), canary) {
			t.Fatalf("provider credential reached the journal: %s", data)
		}
		event = completeEvent()
		event.Route = &Route{Profile: canary}
		if _, err := encode(event); err == nil {
			t.Fatal("provider credential was accepted as an identity")
		}
	}
}

func TestDescriptionBoundsApplyAfterUTF8Repair(t *testing.T) {
	for _, text := range []string{
		strings.Repeat("\xff", MaxSummaryBytes),
		strings.Repeat("a\xff", MaxSummaryBytes),
		strings.Repeat("\xe2\x82", MaxSummaryBytes/2),
		strings.Repeat("界", MaxSummaryBytes),
		"prefix-\xff-suffix",
	} {
		event := completeEvent()
		event.Summary, event.Cause = text, text
		result := decoded(t, event)
		if !utf8.ValidString(result.Summary) || !utf8.ValidString(result.Cause) || len(result.Summary) > MaxSummaryBytes || len(result.Cause) > MaxSummaryBytes {
			t.Fatalf("decoded descriptions exceeded UTF-8 byte bounds: summary=%d cause=%d", len(result.Summary), len(result.Cause))
		}
		if !result.SummaryTruncated || !result.CauseTruncated {
			t.Fatal("description loss was not indicated")
		}
		if strings.HasPrefix(text, "prefix-") && (!strings.HasPrefix(result.Summary, "prefix-") || !strings.HasSuffix(result.Summary, "-suffix")) {
			t.Fatalf("UTF-8 repair lost useful surrounding text: %q", result.Summary)
		}
	}
	event := completeEvent()
	event.References = []Reference{{Kind: strings.Repeat("\xff", 256), Location: strings.Repeat("\xff", 1024), Digest: strings.Repeat("a", 64)}}
	result := decoded(t, event)
	if !result.ReferencesTruncated || len(result.References[0].Kind) > 256 || len(result.References[0].Location) > 1024 || result.References[0].Digest != event.References[0].Digest {
		t.Fatalf("decoded reference bounds or retained hash changed: %+v", result)
	}
}

func TestCredentialFormsAreRedacted(t *testing.T) {
	canary := "opaqueCredentialCanary" + strings.Repeat("Z", 24)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"credential-canary"}`))
	jwt := header + "." + claims + "." + base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("s", 32)))
	oauth := "ya29." + strings.Repeat("A", 80)
	basic := base64.StdEncoding.EncodeToString([]byte("operator:" + canary + " with spaces"))
	privateKey := base64.StdEncoding.EncodeToString([]byte(canary + canary))
	for _, tc := range []struct {
		name      string
		value     string
		forbidden []string
	}{
		{"quoted password", `{"password":"` + canary + ` tailCanary"}`, []string{canary, "tailCanary"}},
		{"unquoted cookie", "Cookie: session=" + canary, []string{canary}},
		{"set-cookie header", "sEt-CoOkIe: session=" + canary + "; HttpOnly; Secure", []string{canary}},
		{"escaped assignment", `PASSWORD="prefix\"` + canary + `"`, []string{canary}},
		{"truncated assignment", `PASSWORD="prefix\"` + canary, []string{canary}},
		{"CLI credential", "--api-key '" + canary + " tailCanary'", []string{canary, "tailCanary"}},
		{"session assignment", "session_id=" + canary, []string{canary}},
		{"credential-shaped key", `{"sk-proj-` + strings.Repeat("A", 40) + `_password":"` + canary + `"}`, []string{canary}},
		{"escaped quoted key", `{\"api_key\":\"` + canary + `\"}`, []string{canary}},
		{"truncated value", `{"password":"` + canary + `\`, []string{canary}},
		{"numeric password", `{"password":123456789012345}`, []string{"123456789012345"}},
		{"authorization", `{"Authorization":"Basic dXNlcjpwYXNzd29yZENhbmFyeQ=="}`, []string{"dXNlcjpwYXNzd29yZENhbmFyeQ=="}},
		{"cookie", `{"Cookie":"session=` + canary + `"}`, []string{canary}},
		{"multiline value", "{\n\"access_token\":\n\"" + canary + "\ntailCanary\"\n}", []string{canary, "tailCanary"}},
		{"raw JWT", "upstream rejected " + jwt, []string{jwt}},
		{"raw OAuth token", "upstream rejected " + oauth, []string{oauth}},
		{"raw Basic authorization", "upstream rejected bAsIc " + basic, []string{basic}},
		{"incomplete private key", "-----BEGIN OPENSSH PRIVATE KEY-----\n" + privateKey, []string{privateKey}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := completeEvent()
			event.Summary, event.Cause = tc.value, tc.value
			data, err := encode(event)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range tc.forbidden {
				if strings.Contains(string(data), secret) {
					t.Fatalf("credential survived sanitization: %s", data)
				}
			}
		})
	}
}

func TestCompactAuthenticationCredentialsAreRedactedAcrossChannels(t *testing.T) {
	root := ignoredRepository(t)
	writer, err := Open(t.Context(), root, quiet())
	if err != nil {
		t.Fatal(err)
	}
	var tokens []string
	for _, header := range []string{
		`{"alg":"HS256","typ":"JWT"}`,
		`{ "alg":"HS256","typ":"JWT"}`,
		" \t\r\n{\n\"typ\": \"JWT\",\n\"alg\": \"HS256\"\n}\n",
		`{"\u0061lg":"HS256"}`,
	} {
		for _, claims := range []string{`{"sub":"credential-canary"}`, `{}`} {
			unsigned := base64.RawURLEncoding.EncodeToString([]byte(header)) + "." + base64.RawURLEncoding.EncodeToString([]byte(claims))
			mac := hmac.New(sha256.New, []byte(strings.Repeat("s", 32)))
			if _, err := mac.Write([]byte(unsigned)); err != nil {
				t.Fatal(err)
			}
			tokens = append(tokens, unsigned+"."+base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))
		}
	}
	block, err := aes.NewCipher([]byte(strings.Repeat("s", 32)))
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{ "alg":"dir","enc":"A256GCM"}`))
	nonce := bytes.Repeat([]byte{'n'}, aead.NonceSize()) // Deterministic fixture nonce; never used outside this test.
	sealed := aead.Seal(nil, nonce, []byte(`{"sub":"encrypted-credential-canary"}`), []byte(header))
	ciphertext, tag := sealed[:len(sealed)-aead.Overhead()], sealed[len(sealed)-aead.Overhead():]
	tokens = append(tokens, header+".."+base64.RawURLEncoding.EncodeToString(nonce)+"."+base64.RawURLEncoding.EncodeToString(ciphertext)+"."+base64.RawURLEncoding.EncodeToString(tag))
	tokens = append(tokens, "ya29."+strings.Repeat("a", 24)+"."+strings.Repeat("b", 32)+"."+strings.Repeat("c", 30))
	for i, token := range tokens {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			event := completeEvent()
			event.Summary = "upstream rejected " + token
			event.Cause = event.Summary
			event.Delivery = &Delivery{PRURL: "https://example.com/evidence/" + token}
			event.References = []Reference{{Kind: "artifact " + token, Location: "evidence://reports/" + token}}
			clean := decoded(t, event)
			if clean.Summary != "upstream rejected [REDACTED]" || clean.Cause != clean.Summary || clean.Delivery.PRURL != "https://example.com/evidence/[REDACTED]" || clean.References[0].Kind != "artifact [REDACTED]" || clean.References[0].Location != "evidence://reports/[REDACTED]" {
				t.Fatal("complete credential was not removed from every description channel")
			}
			if _, err := writer.Append(t.Context(), event, Consequential); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, filepath.FromSlash(journalPath))
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(before, []byte(token)) {
				t.Fatal("credential reached the journal")
			}
			event.Route = &Route{Profile: token}
			if _, err := writer.Append(t.Context(), event, Consequential); err == nil {
				t.Fatal("credential-shaped route identity was admitted")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("refused identity mutated the journal")
			}
		})
	}
}

func TestEncodedFieldsAndArtifactCredentialsAreRemoved(t *testing.T) {
	canary := "opaqueRuntimeCanary1234567890"
	signature := base64.StdEncoding.EncodeToString([]byte(canary + "+/signature"))
	for _, value := range []string{
		`{"service.account.production_database_password":"` + canary + `"}`,
		`upstream error {"\u0070assword":"` + canary + `"}`,
		`upstream error {"\u0050\u0041\u0053\u0053\u0057\u004f\u0052\u0044":"` + canary + `"}`,
		`upstream error {\"\u0070assword\":\"` + canary + `\"}`,
		"https://account.blob.core.windows.net/evidence?sv=2024-11-04&sp=r&sr=b&se=2026-12-01&sig=" + url.QueryEscape(signature),
		"https://account.blob.core.windows.net/evidence?sv=2024-11-04&%73%69%67=" + url.QueryEscape(signature),
		"https://example.com/evidence?X-Amz-Signature=" + signature,
		"https://example.com/evidence?X%2dAmz%2dSignature=" + signature,
		"https://example.com/evidence?X-Goog-Signature=" + signature,
		"-----BEGIN PGP PRIVATE KEY BLOCK-----\n" + canary + "\n-----END PGP PRIVATE KEY BLOCK-----",
	} {
		t.Run(fmt.Sprint(len(value))+"/"+value[:min(20, len(value))], func(t *testing.T) {
			event := completeEvent()
			event.Summary, event.Cause = value, value
			event.Delivery = &Delivery{PRURL: value}
			event.References = []Reference{{Kind: value, Location: value}}
			data, err := encode(event)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{canary, signature, url.QueryEscape(signature)} {
				if bytes.Contains(data, []byte(secret)) {
					t.Fatal("credential survived a description channel")
				}
			}
			event.Delivery = nil
			event.Route = &Route{Profile: value}
			if _, err := encode(event); err == nil {
				t.Fatal("credential-bearing identity was accepted")
			}
		})
	}
}

func TestTailDiagnosticSurvivesBoundaryOnlyWriteFailure(t *testing.T) {
	for _, validTail := range []bool{false, true} {
		t.Run(fmt.Sprint(validTail), func(t *testing.T) {
			root := ignoredRepository(t)
			writer, err := Open(t.Context(), root, quiet())
			if err != nil {
				t.Fatal(err)
			}
			history := []byte("{\"historical\":\"unfinished\"")
			if validTail {
				history, err = json.Marshal(completeEvent())
				if err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(root, filepath.FromSlash(journalPath))
			if err := os.WriteFile(path, history, 0o600); err != nil {
				t.Fatal(err)
			}
			originalWrite := writer.hooks.write
			writer.hooks.write = func(file *os.File, data []byte) (int, error) {
				n, writeErr := originalWrite(file, data[:1])
				if writeErr != nil {
					return n, writeErr
				}
				return n, errors.New("boundary-only interruption")
			}
			if _, err := writer.Append(t.Context(), lifecycle(identity(t), "interrupted"), Consequential); err == nil {
				t.Fatal("interrupted boundary append was acknowledged")
			}
			fresh, err := Open(t.Context(), root, quiet())
			if err != nil {
				t.Fatal(err)
			}
			result, err := fresh.Append(t.Context(), lifecycle(identity(t), "after interruption"), Consequential)
			if err != nil || !result.HistoricalTailIncomplete {
				t.Fatalf("pending tail diagnostic was lost: %+v %v", result, err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(data, append(append([]byte(nil), history...), '\n')) {
				t.Fatal("historical bytes changed")
			}
			tail := bytes.Index(data, []byte("\"event\":\"journal_tail_incomplete\""))
			normal := bytes.Index(data, []byte("\"summary\":\"after interruption\""))
			if tail < 0 || normal < tail {
				t.Fatalf("normal event preceded its tail diagnostic: %s", data)
			}
		})
	}
}

func TestTailDiagnosticDurabilityFailures(t *testing.T) {
	for _, failure := range []string{"marker write", "marker sync", "diagnostic sync", "marker clear"} {
		t.Run(failure, func(t *testing.T) {
			root := ignoredRepository(t)
			writer, err := Open(t.Context(), root, quiet())
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, filepath.FromSlash(journalPath))
			history := []byte("incomplete historical evidence")
			if err := os.WriteFile(path, history, 0o600); err != nil {
				t.Fatal(err)
			}
			originalOpen, originalSync := writer.hooks.open, writer.hooks.sync
			var lock *os.File
			writer.hooks.open = func(r *os.Root, name string, flag int, mode os.FileMode) (*os.File, error) {
				if filepath.Base(name) == "audit.lock" && failure == "marker write" {
					flag = os.O_RDONLY
				}
				file, err := originalOpen(r, name, flag, mode)
				if err == nil && filepath.Base(name) == "audit.lock" {
					lock = file
				}
				return file, err
			}
			journalSyncs := 0
			writer.hooks.sync = func(file *os.File) error {
				if filepath.Base(file.Name()) == "audit.lock" && failure == "marker sync" {
					return errors.New("injected marker sync refusal")
				}
				if filepath.Base(file.Name()) == "logs.jsonl" {
					journalSyncs++
					if journalSyncs == 2 {
						if failure == "diagnostic sync" {
							return errors.New("injected diagnostic sync refusal")
						}
						if failure == "marker clear" {
							if err := originalSync(file); err != nil {
								return err
							}
							return lock.Close()
						}
					}
				}
				return originalSync(file)
			}
			if _, err := writer.Append(t.Context(), lifecycle(identity(t), "unacknowledged"), Consequential); err == nil {
				t.Fatal("failed tail diagnostic was acknowledged")
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(before, []byte("unacknowledged")) || !bytes.HasPrefix(before, history) {
				t.Fatal("normal event was appended before tail durability, or history changed")
			}
			if (failure == "marker write" || failure == "marker sync") && !bytes.Equal(before, history) {
				t.Fatal("journal changed before pending marker durability")
			}
			fresh, err := Open(t.Context(), root, quiet())
			if err != nil {
				t.Fatal(err)
			}
			if result, err := fresh.Append(t.Context(), lifecycle(identity(t), "after refusal"), Buffered); err != nil || !result.HistoricalTailIncomplete {
				t.Fatalf("pending diagnostic lost: %+v %v", result, err)
			}
			if result, err := fresh.Append(t.Context(), lifecycle(identity(t), "clean append"), Consequential); err != nil || result.HistoricalTailIncomplete {
				t.Fatalf("completed diagnostic remained pending: %+v %v", result, err)
			}
		})
	}
}

func TestInvalidTailMarkerRefusesJournalMutation(t *testing.T) {
	for _, marker := range []string{"?", "11"} {
		t.Run(marker, func(t *testing.T) {
			root := ignoredRepository(t)
			writer, err := Open(t.Context(), root, quiet())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".pyxis/state/maintenance/audit.lock"), []byte(marker), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := writer.Append(t.Context(), lifecycle(identity(t), "refused"), Consequential); err == nil {
				t.Fatal("unknown pending-tail state was accepted")
			}
			if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(journalPath))); err != nil || len(data) != 0 {
				t.Fatalf("journal mutated with unknown marker: %q %v", data, err)
			}
		})
	}
}

func FuzzDescriptionBounds(f *testing.F) {
	for _, text := range []string{"", "ordinary text", "prefix-\xff-suffix", strings.Repeat("\xff", MaxSummaryBytes), strings.Repeat("界", MaxSummaryBytes)} {
		f.Add([]byte(text))
	}
	f.Fuzz(func(t *testing.T, text []byte) {
		text = text[:min(len(text), MaxRecordBytes)]
		event := completeEvent()
		event.Summary, event.Cause = string(text), string(text)
		event.References = []Reference{{Kind: string(text), Location: string(text), Digest: strings.Repeat("a", 64)}}
		data, err := encode(event)
		if err != nil {
			t.Fatal(err)
		}
		var result Event
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		if len(data) > MaxRecordBytes || len(result.Summary) > MaxSummaryBytes || len(result.Cause) > MaxSummaryBytes {
			t.Fatal("decoded descriptions or record exceeded byte bounds")
		}
		if !utf8.Valid(text) && (!result.SummaryTruncated || !result.CauseTruncated) {
			t.Fatal("UTF-8 repair was not indicated")
		}
		for _, reference := range result.References {
			if len(reference.Kind) > 256 || len(reference.Location) > 1024 || reference.Digest != event.References[0].Digest {
				t.Fatal("decoded reference exceeded bounds or changed its hash")
			}
		}
	})
}
