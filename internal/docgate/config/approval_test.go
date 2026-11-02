package config

import (
	"strings"
	"testing"
)

func TestHostedApprovalConfigurationIsClosedAndExplicit(t *testing.T) {
	cfg, defects := Parse([]byte("approval:\n  operator_login: operator\n  argus_login: driver\n"))
	if len(defects) != 0 || cfg.Approval != (ApprovalSettings{OperatorLogin: "operator", ArgusLogin: "driver"}) {
		t.Fatalf("approval=%+v defects=%v", cfg.Approval, defects)
	}
	if defaults := Defaults().Approval; defaults != (ApprovalSettings{}) {
		t.Fatalf("a default approval identity shipped: %+v", defaults)
	}
	// A repository that names no operator still loads; approval is where it
	// fails closed, not config load.
	if cfg, defects := Parse([]byte("schema_version: pyxis-config/v2\n")); len(defects) != 0 || cfg.Approval.OperatorLogin != "" {
		t.Fatalf("approval=%+v defects=%v", cfg.Approval, defects)
	}
	for _, body := range []string{
		"approval:\n  argus_logn: driver\n",
		"approval:\n  operator_login: driver\n  argus_login: DRIVER\n",
		"approval:\n  argus_login: owner/repo\n",
		"approval:\n  operator_login: ''\n",
	} {
		if _, defects := Parse([]byte(body)); len(defects) == 0 {
			t.Fatalf("invalid approval config accepted: %s", strings.TrimSpace(body))
		}
	}
}
