// Package journal provides the native append-only writer for historical
// delivery event type corrections. Human operators and Argus use one interface.
package journal

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var anchorRe = regexp.MustCompile(`^E-[1-9][0-9]*$`)

// CorrectionRequest selects an existing event and its canonical classification.
// The writer derives the original type, digest and next dense sequence itself.
type CorrectionRequest struct {
	Target string
	To     string
}

// Metadata records actual caller attribution; it grants no human attestation.
type Metadata struct {
	Date             string
	Principal        string
	AuthorityVersion string
	Reason           string
}

// RenderCorrections is pure and atomic as a batch: any invalid request or
// resulting shape returns no output. The complete original file is retained.
func RenderCorrections(cfg config.Config, slug string, original []byte, requests []CorrectionRequest, meta Metadata) ([]byte, error) {
	if !slugRe.MatchString(slug) || len(requests) == 0 {
		return nil, fmt.Errorf("a valid problem slug and at least one correction are required")
	}
	for _, value := range []string{meta.Date, meta.Principal, meta.AuthorityVersion, meta.Reason} {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("date, principal, authority version and reason must be nonempty single-line values")
		}
	}
	if len(original) == 0 || original[len(original)-1] != '\n' {
		return nil, fmt.Errorf("the original ledger must end with a newline so its last event remains byte-identical")
	}
	file := path.Join(cfg.Problems, slug, "delivery.md")
	l, defects := argusledger.ParseAs("correct-journal", file, schema.LedgerClasses["delivery-ledger"], original)
	if len(defects) > 0 {
		return nil, fmt.Errorf("read original journal: %s", defects[0].String())
	}
	var appendices strings.Builder
	for i, request := range requests {
		if !anchorRe.MatchString(request.Target) || !argusledger.KnownEntryType(request.To) {
			return nil, fmt.Errorf("invalid correction %q=%q", request.Target, request.To)
		}
		from := argusledger.EntryType(l.Entries[request.Target])
		if from == "" || argusledger.KnownEntryType(from) {
			return nil, fmt.Errorf("%s must be an existing event with an unsupported type", request.Target)
		}
		digest, err := argusledger.EntryDigest(l, request.Target)
		if err != nil {
			return nil, err
		}
		gist := argusledger.TypeCorrection{Target: "delivery:" + slug + "#" + request.Target, SHA256: digest, From: from, To: request.To}
		fmt.Fprintf(&appendices, "\n### E-%d — reconciliation — correct %s event classification\n\n- **Date**: %s\n- **Principal**: %s\n- **Authority-contract version**: %s\n- **Body**: %s\n\n%s\n",
			len(l.Order)+i+1, request.Target, meta.Date, meta.Principal, meta.AuthorityVersion, gist.Render(), meta.Reason)
	}
	candidate := append(append([]byte(nil), original...), []byte(appendices.String())...)
	if defects := boundary.ValidateDeliveryChange("correct-journal", cfg, slug, original, candidate); len(defects) > 0 {
		return nil, fmt.Errorf("correction batch refused: %s", defects[0].String())
	}
	return candidate, nil
}

// CorrectFile validates before a single atomic replacement. It does not touch
// Git refs, the index, authority or derived state. Dry runs return the exact
// proposed bytes without changing the file.
func CorrectFile(ctx context.Context, root string, cfg config.Config, slug string, requests []CorrectionRequest, meta Metadata, dryRun bool) ([]byte, error) {
	return correctFile(ctx, root, cfg, slug, requests, meta, dryRun, diskJournalFiles{})
}

func correctFile(ctx context.Context, root string, cfg config.Config, slug string, requests []CorrectionRequest, meta Metadata, dryRun bool, files journalFiles) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !slugRe.MatchString(slug) {
		return nil, fmt.Errorf("invalid problem slug")
	}
	if !filepath.IsLocal(filepath.FromSlash(cfg.Problems)) {
		return nil, fmt.Errorf("problems root must be a local repository path")
	}
	realRoot, err := files.evalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve repository root: %w", err)
	}
	file := filepath.Join(realRoot, filepath.FromSlash(cfg.Problems), slug, "delivery.md")
	info, err := files.lstat(file)
	if err != nil {
		return nil, fmt.Errorf("stat delivery ledger: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("delivery ledger must be a regular file")
	}
	resolved, err := files.evalSymlinks(file)
	if err != nil {
		return nil, fmt.Errorf("resolve delivery ledger: %w", err)
	}
	absolute, err := files.abs(file)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute delivery path: %w", err)
	}
	resolved, err = files.abs(resolved)
	if err != nil || resolved != absolute {
		return nil, fmt.Errorf("delivery ledger path must not traverse a symlink")
	}
	if !dryRun {
		release, err := files.lock(realRoot, path.Join(cfg.Problems, slug, "delivery.md"))
		if err != nil {
			return nil, fmt.Errorf("serialize journal correction: %w", err)
		}
		defer release()
	}
	original, err := files.readFile(file)
	if err != nil {
		return nil, fmt.Errorf("read delivery ledger: %w", err)
	}
	candidate, err := RenderCorrections(cfg, slug, original, requests, meta)
	if err != nil || dryRun {
		return candidate, err
	}
	tmp, err := files.createTemp(filepath.Dir(file), ".journal-correction-*")
	if err != nil {
		return nil, fmt.Errorf("prepare journal replacement: %w", err)
	}
	defer func() {
		// Cleanup is best effort after an error; a successful rename removed it.
		if err := files.remove(tmp.Name()); err != nil && !os.IsNotExist(err) {
			// The caller's primary write error remains authoritative.
			return
		}
	}()
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return nil, fmt.Errorf("set replacement mode: %w (close: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("set replacement mode: %w", err)
	}
	if _, err := tmp.Write(candidate); err != nil {
		if closeErr := tmp.Close(); closeErr != nil {
			return nil, fmt.Errorf("write replacement: %w (close: %v)", err, closeErr)
		}
		return nil, fmt.Errorf("write replacement: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close replacement: %w", err)
	}
	current, err := files.readFile(file)
	if err != nil || !bytes.Equal(current, original) {
		return nil, fmt.Errorf("delivery ledger changed while preparing its correction: %v", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := files.rename(tmp.Name(), file); err != nil {
		return nil, fmt.Errorf("replace delivery ledger: %w", err)
	}
	return candidate, nil
}
