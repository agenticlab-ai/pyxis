package model

import "strings"

// Signal is one entry in the store's governed signal vocabulary — a name, its
// role, and presentation (docs/tech-design/local-work-tracker.md §3.5). Items
// carry signal *names* in their header; the vocabulary defines what each name
// means. Signals are typed control state, replacing a hosted tracker's
// label-encoded state (RW-3, GOV-5).
type Signal struct {
	Name        string
	Role        Role
	Color       string // presentation
	Description string // presentation
}

// Role is how a signal is interpreted — the closed role set (§3.5, §3.14).
// Roles are load-bearing, not decoration: authorization gates the ready-set,
// routing carries "who implements this", status projects the lifecycle, and
// classification is the operand-extensible change taxonomy.
type Role string

const (
	RoleRouting        Role = "routing"
	RoleAuthorization  Role = "authorization"
	RoleStatus         Role = "status"
	RoleClassification Role = "classification"
)

// Roles is the closed role set, in canonical order.
var Roles = []Role{RoleRouting, RoleAuthorization, RoleStatus, RoleClassification}

// ValidRole reports whether r is in the closed role set.
func ValidRole(r Role) bool { return in(Roles, r) }

// The classification namespaces — the only axis an operand may extend (§3.14,
// DINV-11). A classification signal's name is prefixed kind: or area:.
const (
	KindPrefix = "kind:"
	AreaPrefix = "area:"
)

// IsClassificationName reports whether name is in the classification namespace
// (kind:… or area:…).
func IsClassificationName(name string) bool {
	return strings.HasPrefix(name, KindPrefix) || strings.HasPrefix(name, AreaPrefix)
}

// SeedSignals is the canonical, human-authored seed vocabulary pyxis ships — the
// governed input the engine keys on (GOV-5/AP-24, §3.14). It is code, never
// rebuilt from the store file: governance is never rebuilt from the governed.
// The classification (kind:) entries are the defaults on the operand-extensible
// axis; every other entry is engine-recognized and cannot be redefined.
func SeedSignals() []Signal {
	return []Signal{
		{Name: "agent", Role: RoleAuthorization, Color: "#2563eb", Description: "the factory may implement and merge this item"},
		{Name: "needs-plan", Role: RoleRouting, Color: "#7c3aed", Description: "routes to planning before implementation"},
		{Name: "operator", Role: RoleRouting, Color: "#7c3aed", Description: "routes to a human operator"},
		{Name: "retriage", Role: RoleRouting, Color: "#7c3aed", Description: "routes back through triage"},
		{Name: "hold", Role: RoleStatus, Color: "#6b7280", Description: "work is paused"},
		{Name: "worker-failed", Role: RoleStatus, Color: "#dc2626", Description: "an implementation worker failed"},
		{Name: "merge-conflict", Role: RoleStatus, Color: "#dc2626", Description: "the proposal conflicts with its base"},
		{Name: "verify-pending", Role: RoleStatus, Color: "#d97706", Description: "awaiting verification"},
		{Name: "feedback-stalled", Role: RoleStatus, Color: "#d97706", Description: "blocked awaiting reviewer feedback"},
		{Name: "empty-proposal", Role: RoleStatus, Color: "#dc2626", Description: "the proposal introduced no change"},
		{Name: "kind:feat", Role: RoleClassification, Color: "#16a34a", Description: "a feature change"},
		{Name: "kind:bug", Role: RoleClassification, Color: "#16a34a", Description: "a bug fix"},
		{Name: "kind:docs", Role: RoleClassification, Color: "#16a34a", Description: "a documentation change"},
		{Name: "kind:chore", Role: RoleClassification, Color: "#16a34a", Description: "a chore or maintenance change"},
	}
}

// EngineRecognizedSignal reports whether name is an engine-recognized seed
// signal — a seed entry whose role is not the operand-extensible classification
// axis. These names are governed and cannot be redefined by an operand
// (DINV-11).
func EngineRecognizedSignal(name string) bool {
	for _, s := range SeedSignals() {
		if s.Name == name && s.Role != RoleClassification {
			return true
		}
	}
	return false
}

// SignalVocabularySchema tags the machine projection of the signal vocabulary
// (prd:local-work-tracker/FR-15; AP-6). Additive evolution only.
const SignalVocabularySchema = "worktracker.signal-vocabulary/v1"

// SignalView is the machine (JSON) projection of one signal.
type SignalView struct {
	Name        string `json:"name"`
	Role        string `json:"role"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// View projects a Signal onto its machine form.
func (s Signal) View() SignalView {
	return SignalView{Name: s.Name, Role: string(s.Role), Color: s.Color, Description: s.Description}
}

// VocabularyView is the schema-versioned machine projection of the whole signal
// vocabulary — the seed merged with the operand's classification extensions.
type VocabularyView struct {
	Schema  string       `json:"schema"`
	Signals []SignalView `json:"signals"`
}

// Vocabulary projects a signal list onto its schema-versioned machine form.
func Vocabulary(sigs []Signal) VocabularyView {
	v := VocabularyView{Schema: SignalVocabularySchema, Signals: []SignalView{}}
	for _, s := range sigs {
		v.Signals = append(v.Signals, s.View())
	}
	return v
}
