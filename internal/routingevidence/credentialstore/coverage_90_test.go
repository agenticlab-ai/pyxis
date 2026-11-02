package credentialstore

import "testing"

func TestCoverage90NilStoreRejectsPut(t *testing.T) {
	var store *OSStore
	if err := store.Put(t.Context(), "account", []byte("secret")); err == nil {
		t.Fatal("nil store accepted Put")
	}
}
