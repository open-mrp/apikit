package id

import (
	"fmt"
	"strings"
	"testing"
)

const (
	vocUser    = "us"
	vocAccount = "ac"
	vocAPI     = "ap"
	vocKey     = "ke"
)

var (
	userIDPrefix    = ComposePrefix(vocUser)
	accountIDPrefix = ComposePrefix(vocAccount)
	apiKeyIDPrefix  = ComposePrefix(vocAPI, vocKey)
)

func TestGenID_DefaultLength(t *testing.T) {
	t.Parallel()
	id, apiErr := GenID(userIDPrefix, nil)
	if apiErr != nil {
		t.Fatalf("unexpected error: %v", apiErr)
	}

	prefix := string(userIDPrefix) + "_"
	if !strings.HasPrefix(id, prefix) {
		t.Errorf("expected prefix %q, got %q", prefix, id)
	}
	if nanoIDPart := strings.TrimPrefix(id, prefix); len(nanoIDPart) != int(IDLength12) {
		t.Errorf("expected nano ID length %d, got %d (%q)", IDLength12, len(nanoIDPart), nanoIDPart)
	}
}

func TestGenID_CustomLengths(t *testing.T) {
	t.Parallel()
	for _, length := range []IDLength{IDLength12, IDLength19, IDLength22} {
		t.Run(fmt.Sprintf("length_%d", length), func(t *testing.T) {
			id, apiErr := GenID(accountIDPrefix, &length)
			if apiErr != nil {
				t.Fatalf("unexpected error: %v", apiErr)
			}
			if nanoIDPart := strings.TrimPrefix(id, string(accountIDPrefix)+"_"); len(nanoIDPart) != int(length) {
				t.Errorf("expected nano ID length %d, got %d (%q)", length, len(nanoIDPart), nanoIDPart)
			}
		})
	}
}

func TestGenID_UsesCorrectCharset(t *testing.T) {
	t.Parallel()
	for range 50 {
		id, apiErr := GenID(apiKeyIDPrefix, nil)
		if apiErr != nil {
			t.Fatalf("unexpected error: %v", apiErr)
		}
		for _, c := range strings.TrimPrefix(id, string(apiKeyIDPrefix)+"_") {
			if !strings.ContainsRune(charset, c) {
				t.Errorf("character %c not in charset %q (id: %q)", c, charset, id)
			}
		}
	}
}

func TestGenID_Uniqueness(t *testing.T) {
	t.Parallel()
	const count = 1000
	seen := make(map[string]struct{}, count)
	for range count {
		id, apiErr := GenID(userIDPrefix, nil)
		if apiErr != nil {
			t.Fatalf("unexpected error: %v", apiErr)
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate ID generated: %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestComposePrefix(t *testing.T) {
	t.Parallel()
	if apiKeyIDPrefix != "apke" {
		t.Errorf("got %q, want apke", apiKeyIDPrefix)
	}
	for _, bad := range [][]string{nil, {"u"}, {"usr"}, {"US"}, {"u1"}, {"ac", "x"}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("ComposePrefix(%q) did not panic", bad)
				}
			}()
			ComposePrefix(bad...)
		}()
	}
}

func TestGenNanoID(t *testing.T) {
	t.Parallel()
	for _, length := range []IDLength{IDLength12, IDLength19, IDLength22} {
		id, apiErr := genNanoID(length)
		if apiErr != nil {
			t.Fatalf("unexpected error: %v", apiErr)
		}
		if len(id) != int(length) {
			t.Errorf("expected length %d, got %d (%q)", length, len(id), id)
		}
	}
}
