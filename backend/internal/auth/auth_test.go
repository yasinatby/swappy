package auth

import (
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	tokens := NewTokens("secret", time.Hour)
	id, err := tokens.Verify(tokens.Issue("00000000-0000-4000-8000-00000000000a"))
	if err != nil || id != "00000000-0000-4000-8000-00000000000a" {
		t.Fatalf("got %q, %v", id, err)
	}
}

func TestTokenRejectsTamperingAndExpiry(t *testing.T) {
	tokens := NewTokens("secret", time.Hour)
	good := tokens.Issue("user-a")

	cases := map[string]string{
		"other secret": NewTokens("other", time.Hour).Issue("user-a"),
		"expired":      NewTokens("secret", -time.Second).Issue("user-a"),
		"no signature": good[:len(good)-5],
		"garbage":      "abc",
		"empty":        "",
	}
	for name, token := range cases {
		if _, err := tokens.Verify(token); err == nil {
			t.Errorf("%s: token accepted", name)
		}
	}
}
