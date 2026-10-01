// Package auth issues and verifies bearer tokens and puts the caller's user id
// into the request context. Real sign-in (Apple, email) comes later; until
// then tokens are only handed out by the dev login.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidToken = errors.New("invalid token")

// Tokens are "<base64url(userID|expiryUnix)>.<base64url(HMAC-SHA256)>".
type Tokens struct {
	secret []byte
	ttl    time.Duration
}

func NewTokens(secret string, ttl time.Duration) Tokens {
	return Tokens{secret: []byte(secret), ttl: ttl}
}

func (t Tokens) Issue(userID string) string {
	payload := userID + "|" + strconv.FormatInt(time.Now().Add(t.ttl).Unix(), 10)
	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(payload)) + "." + enc.EncodeToString(t.sign(payload))
}

func (t Tokens) Verify(token string) (string, error) {
	enc := base64.RawURLEncoding
	payloadPart, sigPart, ok := strings.Cut(token, ".")
	if !ok {
		return "", ErrInvalidToken
	}
	payload, err := enc.DecodeString(payloadPart)
	if err != nil {
		return "", ErrInvalidToken
	}
	sig, err := enc.DecodeString(sigPart)
	if err != nil || !hmac.Equal(sig, t.sign(string(payload))) {
		return "", ErrInvalidToken
	}
	userID, expiry, ok := strings.Cut(string(payload), "|")
	if !ok {
		return "", ErrInvalidToken
	}
	exp, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil || time.Now().Unix() >= exp {
		return "", ErrInvalidToken
	}
	return userID, nil
}

func (t Tokens) sign(payload string) []byte {
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}

type contextKey struct{}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, contextKey{}, userID)
}

// UserID returns the authenticated user id; empty if the request is anonymous.
func UserID(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}
