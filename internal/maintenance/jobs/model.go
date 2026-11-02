package jobs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // Keep IANA identifiers available in static-binary installations.

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"go.yaml.in/yaml/v3"
)

const SchemaVersion = "pyxis-maintenance-jobs/v1"

var (
	ErrInvalid       = errors.New("invalid maintenance jobs document")
	ErrNotFound      = errors.New("maintenance job not found")
	ErrAlreadyExists = errors.New("maintenance job already exists")
	ErrConflict      = errors.New("maintenance jobs conflict")
	ErrUnconfirmed   = errors.New("maintenance jobs commit durability unconfirmed")
	idPattern        = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	remotePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	dailyPattern     = regexp.MustCompile(`^(?:[0-9]|[1-5][0-9]) (?:[0-9]|1[0-9]|2[0-3]) \* \* \*$`)
)

type Document struct {
	SchemaVersion string        `yaml:"schema_version" json:"schema_version"`
	Source        SourceBinding `yaml:"source" json:"source"`
	Jobs          []Job         `yaml:"jobs" json:"jobs"`
}
type SourceBinding struct {
	Remote string `yaml:"remote" json:"remote"`
	Branch string `yaml:"branch" json:"branch"`
}
type Job struct {
	ID       string    `yaml:"id" json:"id"`
	Enabled  bool      `yaml:"enabled" json:"enabled"`
	Schedule Schedule  `yaml:"schedule" json:"schedule"`
	Timeout  string    `yaml:"timeout" json:"timeout"`
	Runner   Runner    `yaml:"runner" json:"runner"`
	Delivery *Delivery `yaml:"delivery,omitempty" json:"delivery,omitempty"`
	Prompt   string    `yaml:"prompt" json:"prompt"`
}
type Schedule struct {
	Cron     string `yaml:"cron" json:"cron"`
	Timezone string `yaml:"timezone" json:"timezone"`
}
type Runner struct {
	Harness entity.Harness                 `yaml:"harness" json:"harness"`
	Routing executiondemand.ProfileMinimum `yaml:"routing" json:"routing"`
}
type Delivery struct {
	Mode   string `yaml:"mode" json:"mode"`
	Branch string `yaml:"branch" json:"branch"`
}

func Decode(data []byte) (Document, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var tree yaml.Node
	if err := decoder.Decode(&tree); err != nil || len(tree.Content) == 0 {
		return Document{}, invalid("decode YAML: %v", err)
	}
	if err := validNode(&tree); err != nil {
		return Document{}, err
	}
	if err := decoder.Decode(new(yaml.Node)); !errors.Is(err, io.EOF) {
		return Document{}, invalid("expected exactly one YAML document")
	}
	var document Document
	decoder = yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return Document{}, invalid("closed YAML document: %v", err)
	}
	if err := document.Validate(); err != nil {
		return Document{}, err
	}
	return document, nil
}

func Encode(document Document) ([]byte, error) {
	if err := document.Validate(); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return nil, invalid("encode YAML: %v", err)
	}
	return output.Bytes(), nil
}

func (document Document) Validate() error {
	if document.SchemaVersion != SchemaVersion || !validSource(document.Source) || document.Jobs == nil {
		return invalid("unsupported schema or invalid source binding")
	}
	ids, branches := map[string]bool{}, map[string]bool{}
	for index, job := range document.Jobs {
		if err := job.validate(); err != nil {
			return invalid("jobs[%d]: %v", index, err)
		}
		if ids[job.ID] {
			return invalid("duplicate job id %q", job.ID)
		}
		ids[job.ID] = true
		if job.Enabled && job.Delivery != nil {
			if branches[job.Delivery.Branch] {
				return invalid("enabled delivery branch %q is shared", job.Delivery.Branch)
			}
			branches[job.Delivery.Branch] = true
		}
	}
	return nil
}

// ValidID reports whether id belongs to the jobs document's identity grammar.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// ValidSource reports whether source is an admitted maintenance policy source.
func ValidSource(source SourceBinding) bool { return validSource(source) }

func (job Job) validate() error {
	duration, durationErr := time.ParseDuration(job.Timeout)
	if !idPattern.MatchString(job.ID) || strings.TrimSpace(job.Prompt) == "" || !dailyPattern.MatchString(job.Schedule.Cron) {
		return errors.New("invalid id, prompt, or daily cron")
	}
	if job.Schedule.Timezone == "" || job.Schedule.Timezone == "Local" {
		return errors.New("timezone is required")
	}
	if _, err := time.LoadLocation(job.Schedule.Timezone); err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	if durationErr != nil || duration <= 0 || !job.Runner.Harness.Known() || !validRouting(job.Runner.Routing) {
		return errors.New("invalid timeout, harness, or routing")
	}
	if job.Delivery != nil && (job.Delivery.Mode != "draft-pr" || !validBranch(job.Delivery.Branch)) {
		return errors.New("invalid delivery")
	}
	return nil
}

func validRouting(p executiondemand.ProfileMinimum) bool {
	return oneOf(p.Capability, executiondemand.CapabilityEconomy, executiondemand.CapabilityStandard, executiondemand.CapabilityStrong, executiondemand.CapabilityFrontier) && oneOf(p.Deliberation, executiondemand.DeliberationLight, executiondemand.DeliberationStandard, executiondemand.DeliberationDeep, executiondemand.DeliberationMaximum) && oneOf(p.Context, executiondemand.ContextStandard, executiondemand.ContextLarge, executiondemand.ContextExtended) && oneOf(p.Assurance, executiondemand.AssuranceOrdinary, executiondemand.AssuranceElevated, executiondemand.AssuranceCritical)
}
func oneOf[T comparable](value T, values ...T) bool {
	return slices.Contains(values, value)
}
func validSource(source SourceBinding) bool {
	return remotePattern.MatchString(source.Remote) && validBranch(source.Branch)
}
func validBranch(branch string) bool {
	if branch == "" || branch == "@" || strings.HasPrefix(branch, "-") || strings.Contains(branch, "..") || strings.Contains(branch, "@{") || strings.ContainsAny(branch, " ~^:?*[\\\x7f") || strings.ContainsFunc(branch, func(character rune) bool { return character < 32 }) {
		return false
	}
	for component := range strings.SplitSeq(branch, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}

func validNode(node *yaml.Node) error {
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return invalid("line %d: aliases and anchors are unsupported", node.Line)
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for index := 0; index < len(node.Content); index += 2 {
			key := node.Content[index]
			if key.Tag != "!!str" || key.Value == "<<" || seen[key.Value] {
				return invalid("line %d: duplicate, merged, or non-string key %q", key.Line, key.Value)
			}
			seen[key.Value] = true
			if key.Value == "enabled" && node.Content[index+1].Tag != "!!bool" {
				return invalid("line %d: enabled must be an explicit boolean", key.Line)
			}
		}
		if seen["schema_version"] && !seen["jobs"] || seen["id"] && !seen["enabled"] {
			return invalid("line %d: missing required jobs or enabled field", node.Line)
		}
	}
	for _, child := range node.Content {
		if err := validNode(child); err != nil {
			return err
		}
	}
	return nil
}
func invalid(format string, values ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, values...))
}
