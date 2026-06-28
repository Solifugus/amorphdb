package interpreter

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/solifugus/amorphdb/internal/types"
)

func TestCryptoHashPassword_HappyPath(t *testing.T) {
	result := evalHashPassword([]interface{}{types.Text{Value: "hunter2"}})
	rec, ok := result.(types.Record)
	if !ok {
		t.Fatalf("expected Record, got %T (%v)", result, result)
	}

	hashField, ok := rec.Fields["hash"].(types.Text)
	if !ok {
		t.Fatalf("expected hash field of type Text, got %T", rec.Fields["hash"])
	}
	saltField, ok := rec.Fields["salt"].(types.Text)
	if !ok {
		t.Fatalf("expected salt field of type Text, got %T", rec.Fields["salt"])
	}

	hashBytes, err := hex.DecodeString(hashField.Value)
	if err != nil {
		t.Fatalf("hash is not valid hex: %v", err)
	}
	if len(hashBytes) != int(argon2KeyLen) {
		t.Errorf("expected hash length %d bytes, got %d", argon2KeyLen, len(hashBytes))
	}

	saltBytes, err := hex.DecodeString(saltField.Value)
	if err != nil {
		t.Fatalf("salt is not valid hex: %v", err)
	}
	if len(saltBytes) != argon2SaltLen {
		t.Errorf("expected salt length %d bytes, got %d", argon2SaltLen, len(saltBytes))
	}
}

func TestCryptoHashPassword_DifferentSaltsProduceDifferentHashes(t *testing.T) {
	r1 := evalHashPassword([]interface{}{types.Text{Value: "samepw"}}).(types.Record)
	r2 := evalHashPassword([]interface{}{types.Text{Value: "samepw"}}).(types.Record)

	if r1.Fields["salt"].(types.Text).Value == r2.Fields["salt"].(types.Text).Value {
		t.Fatalf("two hash_password calls produced the same salt — randomness broken")
	}
	if r1.Fields["hash"].(types.Text).Value == r2.Fields["hash"].(types.Text).Value {
		t.Fatalf("two hash_password calls produced the same hash — salt not influencing output")
	}
}

func TestCryptoHashPassword_ArgErrors(t *testing.T) {
	cases := []struct {
		name string
		args []interface{}
	}{
		{"no args", nil},
		{"two args", []interface{}{types.Text{Value: "a"}, types.Text{Value: "b"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := evalHashPassword(tc.args)
			if _, ok := result.(types.Unknown); !ok {
				t.Fatalf("expected Unknown, got %T (%v)", result, result)
			}
		})
	}
}

func TestCryptoVerifyPassword_RoundTrip(t *testing.T) {
	rec := evalHashPassword([]interface{}{types.Text{Value: "correct horse battery staple"}}).(types.Record)
	hashTxt := rec.Fields["hash"].(types.Text)
	saltTxt := rec.Fields["salt"].(types.Text)

	result := evalVerifyPassword([]interface{}{
		types.Text{Value: "correct horse battery staple"},
		hashTxt,
		saltTxt,
	})
	b, ok := result.(types.Boolean)
	if !ok {
		t.Fatalf("expected Boolean, got %T (%v)", result, result)
	}
	if !b.Value {
		t.Fatalf("verify_password returned false for correct password")
	}
}

func TestCryptoVerifyPassword_WrongPasswordReturnsFalse(t *testing.T) {
	rec := evalHashPassword([]interface{}{types.Text{Value: "secret"}}).(types.Record)
	hashTxt := rec.Fields["hash"].(types.Text)
	saltTxt := rec.Fields["salt"].(types.Text)

	result := evalVerifyPassword([]interface{}{
		types.Text{Value: "WRONG"},
		hashTxt,
		saltTxt,
	})
	b, ok := result.(types.Boolean)
	if !ok {
		t.Fatalf("expected Boolean, got %T (%v)", result, result)
	}
	if b.Value {
		t.Fatalf("verify_password returned true for wrong password")
	}
}

func TestCryptoVerifyPassword_BadHexReturnsFalse(t *testing.T) {
	cases := []struct {
		name, hash, salt string
	}{
		{"non-hex hash", "not-hex!!", strings.Repeat("ab", argon2SaltLen)},
		{"non-hex salt", strings.Repeat("ab", int(argon2KeyLen)), "zzzz"},
		{"empty hash", "", strings.Repeat("ab", argon2SaltLen)},
		{"empty salt", strings.Repeat("ab", int(argon2KeyLen)), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := evalVerifyPassword([]interface{}{
				types.Text{Value: "anything"},
				types.Text{Value: tc.hash},
				types.Text{Value: tc.salt},
			})
			b, ok := result.(types.Boolean)
			if !ok {
				t.Fatalf("expected Boolean, got %T (%v)", result, result)
			}
			if b.Value {
				t.Fatalf("verify_password returned true for malformed inputs")
			}
		})
	}
}

func TestCryptoVerifyPassword_ArgErrors(t *testing.T) {
	cases := []struct {
		name string
		args []interface{}
	}{
		{"no args", nil},
		{"one arg", []interface{}{types.Text{Value: "p"}}},
		{"two args", []interface{}{types.Text{Value: "p"}, types.Text{Value: "h"}}},
		{"four args", []interface{}{
			types.Text{Value: "p"}, types.Text{Value: "h"},
			types.Text{Value: "s"}, types.Text{Value: "x"},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := evalVerifyPassword(tc.args)
			if _, ok := result.(types.Unknown); !ok {
				t.Fatalf("expected Unknown, got %T (%v)", result, result)
			}
		})
	}
}

func TestCryptoGenerateToken_Length(t *testing.T) {
	result := evalGenerateToken(nil)
	tok, ok := result.(types.Text)
	if !ok {
		t.Fatalf("expected Text, got %T (%v)", result, result)
	}
	if len(tok.Value) != tokenByteLen*2 {
		t.Errorf("expected token of %d hex chars, got %d", tokenByteLen*2, len(tok.Value))
	}
	if _, err := hex.DecodeString(tok.Value); err != nil {
		t.Fatalf("token is not valid hex: %v", err)
	}
}

func TestCryptoGenerateToken_Uniqueness(t *testing.T) {
	seen := make(map[string]struct{}, 16)
	for n := 0; n < 16; n++ {
		tok := evalGenerateToken(nil).(types.Text).Value
		if _, dup := seen[tok]; dup {
			t.Fatalf("duplicate token in 16 calls: %s", tok)
		}
		seen[tok] = struct{}{}
	}
}

func TestCryptoGenerateToken_RejectsArgs(t *testing.T) {
	result := evalGenerateToken([]interface{}{types.Text{Value: "x"}})
	if _, ok := result.(types.Unknown); !ok {
		t.Fatalf("expected Unknown, got %T (%v)", result, result)
	}
}

func TestCryptoLibraryDispatch(t *testing.T) {
	tree := &MockTree{}
	interp := New(tree, 1001)

	t.Run("hash_password routes through dispatcher", func(t *testing.T) {
		result := interp.evalCryptoLibraryProcedure(
			[]string{"hash_password"},
			[]interface{}{types.Text{Value: "p"}},
		)
		if _, ok := result.(types.Record); !ok {
			t.Fatalf("expected Record, got %T (%v)", result, result)
		}
	})

	t.Run("generate_token routes through dispatcher", func(t *testing.T) {
		result := interp.evalCryptoLibraryProcedure(
			[]string{"generate_token"},
			nil,
		)
		if _, ok := result.(types.Text); !ok {
			t.Fatalf("expected Text, got %T (%v)", result, result)
		}
	})

	t.Run("verify_password routes through dispatcher", func(t *testing.T) {
		rec := evalHashPassword([]interface{}{types.Text{Value: "pp"}}).(types.Record)
		result := interp.evalCryptoLibraryProcedure(
			[]string{"verify_password"},
			[]interface{}{
				types.Text{Value: "pp"},
				rec.Fields["hash"],
				rec.Fields["salt"],
			},
		)
		b, ok := result.(types.Boolean)
		if !ok || !b.Value {
			t.Fatalf("expected Boolean(true), got %T (%v)", result, result)
		}
	})

	t.Run("unknown procedure", func(t *testing.T) {
		result := interp.evalCryptoLibraryProcedure(
			[]string{"obliterate_all_secrets"},
			nil,
		)
		u, ok := result.(types.Unknown)
		if !ok {
			t.Fatalf("expected Unknown, got %T (%v)", result, result)
		}
		if !strings.Contains(u.Reason, "unknown crypto procedure") {
			t.Errorf("expected reason to mention unknown crypto procedure, got %q", u.Reason)
		}
	})

	t.Run("empty path", func(t *testing.T) {
		result := interp.evalCryptoLibraryProcedure(nil, nil)
		if _, ok := result.(types.Unknown); !ok {
			t.Fatalf("expected Unknown, got %T (%v)", result, result)
		}
	})

	t.Run("unknown sub-library still rejected", func(t *testing.T) {
		// Ensures the wider computer dispatcher still rejects garbage paths
		// even with crypto registered.
		result := interp.evalComputerLibraryProcedure(
			[]string{"my", "computer", "no_such_lib"},
			nil,
		)
		if _, ok := result.(types.Unknown); !ok {
			t.Fatalf("expected Unknown, got %T (%v)", result, result)
		}
	})
}
