package model

import "testing"

func TestEditPatchNamesOnlyExplicitFields(t *testing.T) {
	value := ""
	if (EditPatch{}).Names() {
		t.Fatal("empty patch names a field")
	}
	if !(EditPatch{Title: &value}).Names() || !(EditPatch{Body: &value}).Names() {
		t.Fatal("explicit title or body was not named")
	}
}
