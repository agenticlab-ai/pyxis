package attribution

import (
	"path"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// verifiedCarinaItemOwner admits one feature item's path only when its
// merge-base content and its current content both name the drive. The merge
// base is outside the current drive's writable proposal range. An item first
// introduced in this proposal cannot claim itself as the trusted owner and
// grants no scope. An unreadable or malformed item also grants no scope.
func verifiedCarinaItemOwner(root, base string, cfg config.Config, target, slug string) bool {
	if !boundary.IsFeatureItemPath(cfg, target) || slug == "" || !existsAtBase(root, base, target) {
		return false
	}
	original, err := gitShowBytes(root, base, target)
	if err != nil || carinaItemOwner(cfg, target, original) != slug {
		return false
	}
	current, err := gitShowBytes(root, "HEAD", target)
	return err == nil && carinaItemOwner(cfg, target, current) == slug
}

// carinaItemOwner accepts the two canonical Argus feature pointers the
// installed tracker has written. It reads only the first body line of a valid
// feature item; a planted citation or later log line cannot claim ownership.
func carinaItemOwner(cfg config.Config, target string, raw []byte) string {
	item, defects := localfs.Parse("argus-item-owner", target, raw)
	if len(defects) != 0 || item.Type != model.TypeFeature {
		return ""
	}
	id, err := strconv.Atoi(strings.TrimSuffix(path.Base(target), ".md"))
	if err != nil || item.ID != id {
		return ""
	}
	first, _, _ := strings.Cut(strings.TrimSpace(item.Body), "\n")
	if owner, ok := strings.CutPrefix(first, "Argus problem "); ok {
		slug, _, _ := strings.Cut(owner, ".")
		if slugRe.MatchString(slug) && (owner == slug || strings.HasPrefix(owner, slug+".")) {
			return slug
		}
	}
	prefix := "Argus intake: " + strings.TrimPrefix(path.Clean(cfg.Problems), "./") + "/"
	if owner, ok := strings.CutPrefix(first, prefix); ok {
		slug, rest, found := strings.Cut(owner, "/problem.md")
		if found && slugRe.MatchString(slug) && (rest == "" || strings.HasPrefix(rest, ".")) {
			return slug
		}
	}
	return ""
}
