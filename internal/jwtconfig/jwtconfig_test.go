package jwtconfig

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"redditclone/internal/model"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-at-least-32-bytes-long!!"

// decodePayload вытаскивает и декодирует JSON-payload токена без проверки подписи.
func decodePayload(t *testing.T, tokenStr string) []byte {
	t.Helper()
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		t.Fatalf("token does not have 3 parts: got %d, token=%q", len(parts), tokenStr)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return payload
}

// signToken подписывает произвольные Claims, минуя NewClaims — нужно для
// крафта «плохих» токенов (просроченных, без user и т.п.).
func signToken(t *testing.T, secret []byte, claims jwt.Claims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString(secret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func TestNewClaims(t *testing.T) {
	u := &model.User{Id: "user-1", Username: "alice"}

	before := time.Now()
	c := NewClaims(u)
	after := time.Now()

	if c.User == nil {
		t.Fatal("User is nil")
	}
	if c.User.Id != u.Id || c.User.Username != u.Username {
		t.Errorf("User = %+v, want %+v", c.User, u)
	}
	if c.IssuedAt == nil {
		t.Fatal("IssuedAt is nil")
	}
	if c.ExpiresAt == nil {
		t.Fatal("ExpiresAt is nil")
	}

	iat := c.IssuedAt.Time
	exp := c.ExpiresAt.Time

	// jwt.NewNumericDate усекает до секунд, поэтому допускаем ±1 секунду.
	if iat.Before(before.Add(-time.Second)) || iat.After(after.Add(time.Second)) {
		t.Errorf("iat = %v not in [%v, %v]", iat, before, after)
	}
	if exp.Sub(iat) != 7*24*time.Hour {
		t.Errorf("lifetime = %v, want 7 days", exp.Sub(iat))
	}
	if !exp.After(iat) {
		t.Errorf("exp (%v) must be after iat (%v)", exp, iat)
	}
}

func TestManager_GenerateToken(t *testing.T) {
	m := NewManager([]byte(testSecret))
	u := &model.User{Id: "user-1", Username: "alice"}

	t.Run("Returns non-empty JWT", func(t *testing.T) {
		token, err := m.GenerateToken(u)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token == "" {
			t.Fatal("token is empty")
		}
		if strings.Count(token, ".") != 2 {
			t.Errorf("token does not look like a JWT: %q", token)
		}
	})

	t.Run("Payload contains user nested, iat and exp", func(t *testing.T) {
		token, err := m.GenerateToken(u)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}

		payload := decodePayload(t, token)

		var raw map[string]any
		if err := json.Unmarshal(payload, &raw); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}

		// user должен лежать в поле "user", а не на верхнем уровне.
		userRaw, ok := raw["user"]
		if !ok {
			t.Fatal(`payload has no "user" field`)
		}
		userObj, ok := userRaw.(map[string]any)
		if !ok {
			t.Fatalf(`"user" is not an object: %T`, userRaw)
		}
		if userObj["id"] != u.Id {
			t.Errorf("user.id = %v, want %q", userObj["id"], u.Id)
		}
		if userObj["username"] != u.Username {
			t.Errorf("user.username = %v, want %q", userObj["username"], u.Username)
		}

		// Нельзя дублировать id/username на верхнем уровне.
		if _, ok := raw["id"]; ok {
			t.Error(`payload must not have top-level "id" field`)
		}
		if _, ok := raw["username"]; ok {
			t.Error(`payload must not have top-level "username" field`)
		}

		// iat и exp должны присутствовать как целые числа (unix seconds).
		iatRaw, ok := raw["iat"]
		if !ok {
			t.Fatal(`payload has no "iat"`)
		}
		expRaw, ok := raw["exp"]
		if !ok {
			t.Fatal(`payload has no "exp"`)
		}
		iat, ok := iatRaw.(float64)
		if !ok {
			t.Fatalf("iat is not a number: %T", iatRaw)
		}
		exp, ok := expRaw.(float64)
		if !ok {
			t.Fatalf("exp is not a number: %T", expRaw)
		}
		if int64(exp)-int64(iat) != int64((7*24*time.Hour)/time.Second) {
			t.Errorf("exp-iat = %d seconds, want 604800", int64(exp)-int64(iat))
		}
	})

	t.Run("Password hash never appears in the token", func(t *testing.T) {
		u := &model.User{Id: "user-1", Username: "alice", PasswordHash: "super-secret-bcrypt-hash"}

		token, err := m.GenerateToken(u)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if strings.Contains(token, "super-secret-bcrypt-hash") {
			t.Fatalf("password hash leaked into raw JWT: %q", token)
		}

		payload := decodePayload(t, token)
		if strings.Contains(strings.ToLower(string(payload)), "password") {
			t.Fatalf("payload mentions password: %s", payload)
		}
	})
}

func TestManager_ParseToken(t *testing.T) {
	secret := []byte(testSecret)
	m := NewManager(secret)
	u := &model.User{Id: "user-1", Username: "alice"}

	validToken, err := m.GenerateToken(u)
	if err != nil {
		t.Fatalf("generate valid token: %v", err)
	}

	t.Run("Valid token round-trips user", func(t *testing.T) {
		got, err := m.ParseToken(validToken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("got nil user")
		}
		if got.Id != u.Id {
			t.Errorf("id = %q, want %q", got.Id, u.Id)
		}
		if got.Username != u.Username {
			t.Errorf("username = %q, want %q", got.Username, u.Username)
		}
	})

	t.Run("Empty string is rejected", func(t *testing.T) {
		if _, err := m.ParseToken(""); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Garbage token is rejected", func(t *testing.T) {
		for _, tok := range []string{"not-a-jwt", "a.b.c", validToken + "x"} {
			if _, err := m.ParseToken(tok); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("token %q: expected ErrInvalidToken, got %v", tok, err)
			}
		}
	})

	t.Run("Token signed with different secret is rejected", func(t *testing.T) {
		other := NewManager([]byte("a-completely-different-secret-32!!"))
		otherToken, err := other.GenerateToken(u)
		if err != nil {
			t.Fatalf("generate other: %v", err)
		}
		if _, err := m.ParseToken(otherToken); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Expired token is rejected", func(t *testing.T) {
		token := signToken(t, secret, &Claims{
			User: u,
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			},
		})
		if _, err := m.ParseToken(token); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Token without exp is rejected", func(t *testing.T) {
		token := signToken(t, secret, &Claims{
			User: u,
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt: jwt.NewNumericDate(time.Now()),
			},
		})
		if _, err := m.ParseToken(token); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Token with non-HS256 algorithm is rejected", func(t *testing.T) {
		for _, method := range []*jwt.SigningMethodHMAC{
			jwt.SigningMethodHS384,
			jwt.SigningMethodHS512,
		} {
			tok := jwt.NewWithClaims(method, NewClaims(u))
			signed, err := tok.SignedString(secret)
			if err != nil {
				t.Fatalf("%s sign: %v", method.Alg(), err)
			}
			if _, err := m.ParseToken(signed); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("%s: expected ErrInvalidToken, got %v", method.Alg(), err)
			}
		}
	})

	t.Run("Token with alg=none is rejected", func(t *testing.T) {
		tok := jwt.NewWithClaims(jwt.SigningMethodNone, NewClaims(u))
		signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
		if err != nil {
			t.Fatalf("sign none: %v", err)
		}
		if _, err := m.ParseToken(signed); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Token without user is rejected", func(t *testing.T) {
		token := signToken(t, secret, &Claims{
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})
		if _, err := m.ParseToken(token); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Token with empty user id is rejected", func(t *testing.T) {
		token := signToken(t, secret, &Claims{
			User: &model.User{Id: "", Username: "alice"},
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})
		if _, err := m.ParseToken(token); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Token with empty username is rejected", func(t *testing.T) {
		token := signToken(t, secret, &Claims{
			User: &model.User{Id: "user-1", Username: ""},
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})
		if _, err := m.ParseToken(token); !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("Admin flag round-trips", func(t *testing.T) {
		admin := &model.User{Id: "admin-1", Username: "admin", Admin: true}
		token, err := m.GenerateToken(admin)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		got, err := m.ParseToken(token)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if !got.Admin {
			t.Errorf("admin flag lost in round-trip")
		}
	})
}

func TestManager_ConcurrentUseDoesNotRace(t *testing.T) {
	m := NewManager([]byte(testSecret))
	u := &model.User{Id: "user-1", Username: "alice"}

	token, err := m.GenerateToken(u)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, _ = m.ParseToken(token)
			_, _ = m.GenerateToken(u)
		}()
	}
	wg.Wait()
}
