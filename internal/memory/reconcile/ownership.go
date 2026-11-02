package reconcile

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	planclass "github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

// ContextSource is one authored artifact whose document identity belongs to a
// feature closeout. Subject remains the source reconciler's canonical address;
// context records are never re-homed onto the PRD subject.
type ContextSource struct {
	Path    string
	Subject string
}

var (
	ownershipHeaderRowRE    = regexp.MustCompile(`^\|\s*\*\*([^*]+)\*\*\s*\|\s*(.*?)\s*\|\s*$`)
	ownershipTableHeaderRE  = regexp.MustCompile(`^\|\s*Field\s*\|\s*Value\s*\|\s*$`)
	ownershipTableDividerRE = regexp.MustCompile(`^\|\s*:?-{3,}:?\s*\|\s*:?-{3,}:?\s*\|\s*$`)
	prdReviewSubjectRE      = regexp.MustCompile(`^prd:([a-z][a-z0-9-]*)/([0-9]{3})$`)
	typedPRDTokenRE         = regexp.MustCompile(`prd:[a-z][a-z0-9-]*`)
	designBrainstormNameRE  = regexp.MustCompile(`^([a-z][a-z0-9-]*)-tech-design(?:-[1-9][0-9]*)?\.md$`)
	planSourceNameRE        = regexp.MustCompile(`^[0-9]{3}-plan\.md$`)
)

// FeatureContextSources projects exact feature ownership from pinned artifact
// schemas. It uses directory identity only for directory-scoped classes and
// typed header joins for flat corpora; titles, prose, and loose filename
// prefixes never imply ownership.
func FeatureContextSources(s Snapshot, feature string) ([]ContextSource, error) {
	if !validFeature(feature) {
		return nil, incompletef("feature %q is not a canonical feature slug", feature)
	}
	if _, err := exactFeaturePRD(feature, s.PRDs); err != nil {
		return nil, err
	}
	owners, err := deriveContextSourceOwners(s, feature)
	if err != nil {
		return nil, err
	}
	opts := optionDefaults(s.SourceOptions)
	var out []ContextSource
	for source, features := range owners {
		if !containsString(features, feature) {
			continue
		}
		subject, _ := sourceAddress(source, opts)
		out = append(out, ContextSource{Path: source, Subject: subject})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// ContextSourceOwners returns the complete owner set for each typed feature
// context path in a snapshot. A shared path stays shared; retirement callers
// can therefore require a trusted closeout from every owner.
func ContextSourceOwners(s Snapshot) (map[string][]string, error) {
	return deriveContextSourceOwners(s, "")
}

func deriveContextSourceOwners(s Snapshot, strictFeature string) (map[string][]string, error) {
	opts := optionDefaults(s.SourceOptions)
	authored := make(map[string]bool, len(s.AuthoredSources))
	for _, source := range s.AuthoredSources {
		authored[source] = true
	}
	featureSet := make(map[string]bool, len(s.PRDs))
	for _, prd := range s.PRDs {
		if featureSet[prd.Feature] {
			return nil, incompletef("feature %s has more than one compiled PRD ledger", prd.Feature)
		}
		featureSet[prd.Feature] = true
	}

	owned := map[string]map[string]bool{}
	add := func(source string, features ...string) error {
		if !authored[source] {
			return incompletef("owned feature context %s is not in the pinned authored-source inventory", source)
		}
		if owned[source] == nil {
			owned[source] = map[string]bool{}
		}
		for _, feature := range features {
			owned[source][feature] = true
		}
		return nil
	}

	// A PRD ledger directory is the feature's primary identity.
	for _, prd := range s.PRDs {
		prdDir := path.Dir(prd.Source)
		found := false
		for _, source := range s.AuthoredSources {
			if path.Dir(source) != prdDir || !ledgerNameRE.MatchString(path.Base(source)) {
				continue
			}
			if err := add(source, prd.Feature); err != nil {
				return nil, err
			}
			found = true
		}
		if !found {
			return nil, incompletef("feature %s has no pinned authored PRD ledger sources", prd.Feature)
		}
	}

	// Technical designs own their source through exact Serves joins. The same
	// map resolves flat design-review subjects below.
	designOwners := map[string][]string{}
	for _, design := range s.Designs {
		for _, serves := range design.Serves {
			subject, _, ok := memoryPRDRef(serves)
			if !ok {
				continue
			}
			feature := strings.TrimPrefix(subject, "prd/")
			designOwners[design.Slug] = append(designOwners[design.Slug], feature)
		}
		designOwners[design.Slug] = compactSorted(designOwners[design.Slug])
		if len(designOwners[design.Slug]) > 0 {
			if err := add(design.Source, designOwners[design.Slug]...); err != nil {
				return nil, err
			}
		}
	}

	// One valid plan source binds its whole directory, including proof and
	// disposition companions. Generated JSON projections are not authored and
	// never enter s.AuthoredSources.
	validPlanDirs := map[string]string{}
	for _, source := range s.AuthoredSources {
		rel, ok := directChild(opts.Plans, source)
		if !ok || !planSourceNameRE.MatchString(path.Base(rel)) {
			continue
		}
		feature := path.Dir(rel)
		doc, defects := planclass.Parse(source, s.Files[source].Body)
		if len(defects) > 0 || doc == nil {
			if feature == strictFeature {
				return nil, incompletef("feature plan %s has invalid ownership metadata: %s", source, defectSummary(defects))
			}
			continue
		}
		prdDir := path.Join(opts.PRDs, feature)
		if doc.Feature != feature || doc.TemplateVersion != planclass.PlanTemplateVersion || path.Clean(doc.PRD) != prdDir || !featureSet[feature] {
			if feature == strictFeature || doc.Feature == strictFeature {
				return nil, incompletef("feature plan %s does not bind directory, Feature, PRD, and template version consistently", source)
			}
			continue
		}
		validPlanDirs[path.Dir(source)] = feature
	}
	for dir, feature := range validPlanDirs {
		for _, source := range s.AuthoredSources {
			if path.Dir(source) == dir {
				if err := add(source, feature); err != nil {
					return nil, err
				}
			}
		}
	}

	// Design-brainstorm Basis is the bounded bridge for shared and cross-slug
	// lifecycle context. Resolve every design bridge before inspecting stage-1
	// records so ownership is independent of source ordering and a malformed
	// cross-slug stage-1 record cannot evade a strict feature closeout.
	type brainstormSource struct {
		source string
		header map[string]string
		err    error
	}
	var brainstorms []brainstormSource
	for _, source := range s.AuthoredSources {
		if path.Dir(source) != path.Clean(opts.Brainstorms) {
			continue
		}
		header, err := ownershipHeader(s.Files[source].Body)
		brainstorms = append(brainstorms, brainstormSource{source: source, header: header, err: err})
	}

	contextOwners := map[string][]string{}
	for _, brainstorm := range brainstorms {
		if brainstorm.err != nil || brainstorm.header["Template version"] != "design-brainstorm-record/v1" {
			continue
		}
		owners, context, ownerErr := designBrainstormOwners(brainstorm.source, brainstorm.header)
		if ownerErr != nil {
			if brainstormNameCouldOwn(path.Base(brainstorm.source), strictFeature) {
				return nil, incompletef("design brainstorm %s has invalid ownership metadata: %v", brainstorm.source, ownerErr)
			}
			continue
		}
		if err := add(brainstorm.source, owners...); err != nil {
			return nil, err
		}
		contextOwners[context] = compactSorted(append(contextOwners[context], owners...))
	}

	for _, brainstorm := range brainstorms {
		source := brainstorm.source
		context := strings.TrimSuffix(path.Base(source), path.Ext(source))
		couldOwnStrict := brainstormNameCouldOwn(path.Base(source), strictFeature) || containsString(contextOwners[context], strictFeature)
		if brainstorm.err != nil {
			if couldOwnStrict {
				return nil, incompletef("brainstorm %s has invalid ownership metadata: %v", source, brainstorm.err)
			}
			continue
		}
		switch brainstorm.header["Template version"] {
		case "brainstorm-record/v1":
			owners, ownerErr := stageOneBrainstormOwners(source, brainstorm.header, opts)
			if ownerErr != nil {
				if couldOwnStrict {
					return nil, incompletef("brainstorm %s has invalid ownership metadata: %v", source, ownerErr)
				}
				continue
			}
			owners = compactSorted(append(owners, contextOwners[context]...))
			if len(owners) > 0 {
				if err := add(source, owners...); err != nil {
					return nil, err
				}
			}
		case "design-brainstorm-record/v1":
			// Already validated and projected in the first pass.
		default:
			if couldOwnStrict {
				return nil, incompletef("brainstorm %s has unknown template version %q", source, brainstorm.header["Template version"])
			}
		}
	}

	// Review reports join through exact typed subjects; a design review inherits
	// every feature served by that exact indexed design.
	for _, source := range s.AuthoredSources {
		if path.Dir(source) != path.Clean(opts.Reviews) {
			continue
		}
		owners, err := reviewOwners(source, s.Files[source].Body, designOwners)
		if err != nil {
			if reviewNameCouldOwn(path.Base(source), strictFeature, designOwners) {
				return nil, incompletef("review %s has invalid ownership metadata: %v", source, err)
			}
			continue
		}
		if err := add(source, owners...); err != nil {
			return nil, err
		}
	}

	if err := addProblemContexts(s, featureSet, contextOwners, owned, authored, opts, strictFeature); err != nil {
		return nil, err
	}

	out := make(map[string][]string, len(owned))
	for source, ownerSet := range owned {
		for owner := range ownerSet {
			out[source] = append(out[source], owner)
		}
		sort.Strings(out[source])
	}
	return out, nil
}

func addProblemContexts(s Snapshot, features map[string]bool, bridged map[string][]string, owned map[string]map[string]bool, authored map[string]bool, opts Options, strictFeature string) error {
	contexts := map[string]bool{}
	for _, source := range s.AuthoredSources {
		rel := strings.TrimPrefix(source, strings.TrimSuffix(path.Clean(opts.Problems), "/")+"/")
		if rel != source && strings.Count(rel, "/") == 1 {
			contexts[path.Dir(rel)] = true
		}
	}
	for context := range contexts {
		owners := append([]string(nil), bridged[context]...)
		if features[context] {
			owners = append(owners, context)
		}
		owners = compactSorted(owners)
		if len(owners) == 0 {
			continue
		}
		root := path.Join(opts.Problems, context)
		problemPath, deliveryPath := path.Join(root, "problem.md"), path.Join(root, "delivery.md")
		unsupported := ""
		for _, source := range s.AuthoredSources {
			if strings.HasPrefix(source, root+"/") && source != problemPath && source != deliveryPath {
				unsupported = source
				break
			}
		}
		if unsupported != "" {
			if containsString(owners, strictFeature) {
				return incompletef("problem directory contains unsupported authored context %s", unsupported)
			}
			continue
		}
		if !authored[problemPath] || !authored[deliveryPath] {
			if containsString(owners, strictFeature) {
				return incompletef("owned problem context %s must contain both problem.md and delivery.md", root)
			}
			continue
		}
		valid := true
		for _, source := range []string{problemPath, deliveryPath} {
			ledger, defects := argusledger.Parse(opts.Problems, "memory-closeout", source, s.Files[source].Body)
			if len(defects) > 0 || ledger == nil {
				if containsString(owners, strictFeature) {
					return incompletef("feature problem ledger %s is invalid: %s", source, defectSummary(defects))
				}
				valid = false
				break
			}
			header, err := ownershipHeader(s.Files[source].Body)
			if err != nil || header["Slug"] != context || header["Template version"] != ledger.Class.TemplateVersion {
				if containsString(owners, strictFeature) {
					return incompletef("feature problem ledger %s does not bind Slug and exact template version to %s", source, context)
				}
				valid = false
				break
			}
			fields := []string{"Slug", "Template version"}
			if ledger.Class.Name == "problem-ledger" {
				fields = []string{"Slug", "Principal", "Authority-contract version", "Envelope override", "Template version"}
			}
			if err := requireHeaderFields(header, fields); err != nil {
				if containsString(owners, strictFeature) {
					return incompletef("feature problem ledger %s has invalid header: %v", source, err)
				}
				valid = false
				break
			}
			if owned[source] == nil {
				owned[source] = map[string]bool{}
			}
			for _, owner := range owners {
				owned[source][owner] = true
			}
		}
		if !valid {
			delete(owned, problemPath)
			delete(owned, deliveryPath)
			continue
		}
	}
	return nil
}

func stageOneBrainstormOwners(file string, header map[string]string, opts Options) ([]string, error) {
	if err := requireHeaderFields(header, []string{"In one line", "Relevant-when", "Idea", "Operator", "Date(s)", "Status", "Tracking", "Feeds", "Prior records", "Template version"}); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	feed := strings.TrimSpace(header["Feeds"])
	if feed == "—" || feed == "-" {
		return nil, nil
	}
	feed, ok := exactCodeOrBare(feed)
	if !ok {
		return nil, fmt.Errorf("%s: Feeds is not one exact path", file)
	}
	root := strings.TrimSuffix(path.Clean(opts.PRDs), "/") + "/"
	if !strings.HasPrefix(feed, root) || !strings.HasSuffix(feed, "/") {
		return nil, fmt.Errorf("%s: Feeds is not an exact PRD directory or an explicit dash", file)
	}
	feature := strings.TrimSuffix(strings.TrimPrefix(feed, root), "/")
	if strings.Contains(feature, "/") || !validFeature(feature) {
		return nil, fmt.Errorf("%s: Feeds does not name one canonical feature", file)
	}
	return []string{feature}, nil
}

func designBrainstormOwners(file string, header map[string]string) ([]string, string, error) {
	if err := requireHeaderFields(header, []string{"In one line", "Relevant-when", "Basis", "Authority", "Date(s)", "Status", "Tracking", "Feeds", "Prior records", "Template version"}); err != nil {
		return nil, "", fmt.Errorf("%s: %w", file, err)
	}
	owners, err := typedPRDFeatures(header["Basis"])
	if err != nil {
		return nil, "", fmt.Errorf("%s: Basis: %w", file, err)
	}
	match := designBrainstormNameRE.FindStringSubmatch(path.Base(file))
	if match == nil {
		return nil, "", fmt.Errorf("%s: filename is not <context>-tech-design[-N].md", file)
	}
	return owners, match[1], nil
}

func reviewOwners(file string, body []byte, designOwners map[string][]string) ([]string, error) {
	header, err := ownershipHeader(body)
	if err != nil {
		return nil, err
	}
	if err := requireHeaderFields(header, []string{"In one line", "Relevant-when", "Subject", "Moment", "Reviewer", "Date(s)", "Verdict", "Tracking", "Template version"}); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	stem := strings.TrimSuffix(path.Base(file), path.Ext(file))
	switch header["Template version"] {
	case "review-report/v1":
		subject, ok := exactCodeOrBare(strings.TrimSpace(header["Subject"]))
		if !ok {
			return nil, fmt.Errorf("%s: PRD review Subject is not one exact typed reference", file)
		}
		match := prdReviewSubjectRE.FindStringSubmatch(subject)
		if match == nil || stem != match[1]+"-"+match[2] {
			return nil, fmt.Errorf("%s: PRD review Subject and filename disagree", file)
		}
		return []string{match[1]}, nil
	case "design-review-report/v1":
		token, ok := leadingTypedToken(header["Subject"])
		if !ok || !strings.HasPrefix(token, "design:") {
			return nil, fmt.Errorf("%s: design review Subject has no exact leading design reference", file)
		}
		slug := strings.TrimPrefix(token, "design:")
		if !validFeature(slug) || stem != slug || len(designOwners[slug]) == 0 {
			return nil, fmt.Errorf("%s: design review Subject does not resolve to the same pinned design", file)
		}
		return append([]string(nil), designOwners[slug]...), nil
	default:
		return nil, fmt.Errorf("%s: unknown review template version", file)
	}
}

// ownershipHeader parses exactly the first header table. It refuses content
// before the H1, malformed or non-bold data rows, a second table, duplicate
// fields, and any material between the table and the first H2.
func ownershipHeader(body []byte) (map[string]string, error) {
	lines := strings.Split(strings.TrimSuffix(string(schema.Normalize(body)), "\n"), "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || !strings.HasPrefix(strings.TrimSpace(lines[i]), "# ") || strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
		return nil, fmt.Errorf("document does not open with one H1")
	}
	i++
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || !ownershipTableHeaderRE.MatchString(strings.TrimSpace(lines[i])) {
		return nil, fmt.Errorf("first material after H1 is not the Field/Value header table")
	}
	i++
	if i >= len(lines) || !ownershipTableDividerRE.MatchString(strings.TrimSpace(lines[i])) {
		return nil, fmt.Errorf("header table has no two-column divider")
	}
	i++
	header := map[string]string{}
	rows := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
		line := strings.TrimSpace(lines[i])
		match := ownershipHeaderRowRE.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("header table row %d is not one bold Field/Value row", i+1)
		}
		field, value := strings.TrimSpace(match[1]), strings.TrimSpace(match[2])
		if _, duplicate := header[field]; duplicate {
			return nil, fmt.Errorf("duplicate header field %q", field)
		}
		header[field] = value
		rows++
		i++
	}
	if rows == 0 {
		return nil, fmt.Errorf("header table has no bold Field/Value rows")
	}
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "## ") {
			return header, nil
		}
		if line != "" {
			return nil, fmt.Errorf("material appears outside the first header table before the first H2")
		}
		i++
	}
	return nil, fmt.Errorf("document has no H2 after its header table")
}

func requireHeaderFields(header map[string]string, fields []string) error {
	allowed := make(map[string]bool, len(fields))
	for _, field := range fields {
		allowed[field] = true
		if strings.TrimSpace(header[field]) == "" {
			return fmt.Errorf("required header field %q is absent", field)
		}
	}
	for field := range header {
		if !allowed[field] {
			return fmt.Errorf("unknown header field %q", field)
		}
	}
	return nil
}

func typedPRDFeatures(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("no typed PRD reference")
	}
	if !strings.HasPrefix(value, "prd:") && !strings.HasPrefix(value, "`prd:") {
		return nil, fmt.Errorf("does not start with a typed PRD reference")
	}
	if strings.Count(value, "`")%2 != 0 {
		return nil, fmt.Errorf("contains an unclosed inline-code span")
	}
	var out []string
	for _, loc := range typedPRDTokenRE.FindAllStringIndex(value, -1) {
		if !typedTokenBoundary(value, loc[0], loc[1]) {
			continue
		}
		feature := strings.TrimPrefix(value[loc[0]:loc[1]], "prd:")
		if !validFeature(feature) {
			return nil, fmt.Errorf("invalid typed PRD feature %q", feature)
		}
		out = append(out, feature)
	}
	out = compactSorted(out)
	if len(out) == 0 || len(out) != strings.Count(value, "prd:") {
		return nil, fmt.Errorf("contains an invalid or ambiguous typed PRD reference")
	}
	return out, nil
}

func brainstormNameCouldOwn(name, feature string) bool {
	if feature == "" {
		return false
	}
	stem := strings.TrimSuffix(name, path.Ext(name))
	if stem == feature {
		return true
	}
	match := designBrainstormNameRE.FindStringSubmatch(name)
	return match != nil && match[1] == feature
}

func reviewNameCouldOwn(name, feature string, designOwners map[string][]string) bool {
	if feature == "" {
		return false
	}
	stem := strings.TrimSuffix(name, path.Ext(name))
	if strings.HasPrefix(stem, feature+"-") {
		return true
	}
	return containsString(designOwners[stem], feature)
}

func typedTokenBoundary(value string, start, end int) bool {
	boundary := func(r rune) bool {
		if unicode.IsSpace(r) {
			return true
		}
		switch r {
		case '`', ',', ';', '(', ')', '·', '—':
			return true
		}
		return false
	}
	if start > 0 {
		r, size := utf8.DecodeLastRuneInString(value[:start])
		if r == utf8.RuneError && size == 1 || !boundary(r) {
			return false
		}
	}
	if end < len(value) {
		r, size := utf8.DecodeRuneInString(value[end:])
		if r == utf8.RuneError && size == 1 || !boundary(r) {
			return false
		}
	}
	return true
}

func exactCodeOrBare(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	if value[0] != '`' {
		return value, !strings.Contains(value, "`") && !strings.ContainsAny(value, " \t\r\n")
	}
	if len(value) < 3 || value[len(value)-1] != '`' || strings.Contains(value[1:len(value)-1], "`") {
		return "", false
	}
	return value[1 : len(value)-1], true
}

func leadingTypedToken(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	if value[0] == '`' {
		end := strings.IndexByte(value[1:], '`')
		if end < 1 {
			return "", false
		}
		token := value[1 : end+1]
		rest := strings.TrimSpace(value[end+2:])
		if rest != "" && !strings.HasPrefix(rest, "·") && !strings.HasPrefix(rest, "(") {
			return "", false
		}
		return token, true
	}
	end := len(value)
	for i, r := range value {
		if r == '·' || r == '(' || r == ' ' || r == '\t' {
			end = i
			break
		}
	}
	return value[:end], end > 0
}

func directChild(root, source string) (string, bool) {
	root = strings.TrimSuffix(path.Clean(root), "/")
	rel := strings.TrimPrefix(source, root+"/")
	return rel, rel != source && strings.Count(rel, "/") == 1
}

func compactSorted(values []string) []string {
	sort.Strings(values)
	return compactStrings(values)
}

func defectSummary[T fmt.Stringer](defects []T) string {
	if len(defects) == 0 {
		return "no parsed document"
	}
	parts := make([]string, 0, len(defects))
	for _, defect := range defects {
		parts = append(parts, defect.String())
	}
	return strings.Join(parts, "; ")
}

func containsString(values []string, want string) bool {
	return slices.Contains(values, want)
}
