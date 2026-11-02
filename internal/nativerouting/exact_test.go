package nativerouting

import "testing"

func TestExactSelectionPreservesPairAndRefusesSubstitution(t *testing.T) {
	first := resolverEntry("first", 1, Provides{}, false)
	second := resolverEntry("second", 2, Provides{}, true)
	catalog := resolverCatalog(RoleImplementation, first, second)
	got, err := catalog.SelectExact(second.NativeModel, second.NativeEffort)
	if err != nil || got.NativeProfileID != second.NativeProfileID {
		t.Fatalf("exact selection: %+v %v", got, err)
	}
	if _, err := catalog.SelectExact(first.NativeModel, second.NativeEffort); err == nil {
		t.Fatal("substituted a different model/effort pair")
	}
	catalog.Catalog.Entries = append(catalog.Catalog.Entries, second)
	if _, err := catalog.SelectExact(second.NativeModel, second.NativeEffort); err == nil {
		t.Fatal("accepted ambiguous native pair")
	}
	catalog.Identity = ""
	if _, err := catalog.SelectExact(first.NativeModel, first.NativeEffort); err == nil {
		t.Fatal("accepted an unvalidated catalog")
	}
}
