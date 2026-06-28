package interpreter

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/argon2"

	"github.com/solifugus/amorphdb/internal/types"
)

// Argon2id parameters follow current OWASP guidance. They are tuned for
// interactive login latency: roughly tens of milliseconds on typical
// developer hardware. Increasing time or memory raises attacker cost
// proportionally — change with care, as tokens hashed under one parameter
// set must continue to verify even after parameters are tuned (the salt is
// stored separately, but the hash itself encodes nothing about parameters,
// so any change here invalidates existing hashes).
const (
	argon2Time    uint32 = 2
	argon2Memory  uint32 = 64 * 1024 // 64 MiB
	argon2Threads uint8  = 1
	argon2KeyLen  uint32 = 32
	argon2SaltLen        = 16
	tokenByteLen         = 32
)

// evalCryptoLibraryProcedure dispatches my.computer.crypto.* calls.
func (i *Interpreter) evalCryptoLibraryProcedure(path []string, args []interface{}) interface{} {
	if len(path) == 0 {
		return types.Unknown{Reason: "incomplete crypto library path"}
	}

	procedure := path[0]

	switch procedure {
	case "hash_password":
		return evalHashPassword(args)
	case "verify_password":
		return evalVerifyPassword(args)
	case "generate_token":
		return evalGenerateToken(args)
	default:
		return types.Unknown{Reason: fmt.Sprintf("unknown crypto procedure: %s", procedure)}
	}
}

// evalHashPassword implements my.computer.crypto.hash_password(password).
// Returns a record { hash: "<hex>", salt: "<hex>" } using Argon2id.
func evalHashPassword(args []interface{}) interface{} {
	if len(args) != 1 {
		return types.Unknown{Reason: "hash_password requires exactly one argument: password"}
	}
	pw := types.CoerceToText(args[0])
	if !pw.Ok {
		return types.Unknown{Reason: "hash_password password must be text"}
	}
	password := pw.Value.(types.Text).Value

	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return types.Unknown{Reason: fmt.Sprintf("hash_password salt generation failed: %v", err)}
	}

	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	return types.Record{Fields: map[string]interface{}{
		"hash": types.Text{Value: hex.EncodeToString(hash)},
		"salt": types.Text{Value: hex.EncodeToString(salt)},
	}}
}

// evalVerifyPassword implements my.computer.crypto.verify_password(password, hash, salt).
// Returns Boolean true on match, false otherwise. Uses constant-time comparison.
func evalVerifyPassword(args []interface{}) interface{} {
	if len(args) != 3 {
		return types.Unknown{Reason: "verify_password requires exactly three arguments: password, hash, salt"}
	}
	pw := types.CoerceToText(args[0])
	if !pw.Ok {
		return types.Unknown{Reason: "verify_password password must be text"}
	}
	hashArg := types.CoerceToText(args[1])
	if !hashArg.Ok {
		return types.Unknown{Reason: "verify_password hash must be text"}
	}
	saltArg := types.CoerceToText(args[2])
	if !saltArg.Ok {
		return types.Unknown{Reason: "verify_password salt must be text"}
	}

	expected, err := hex.DecodeString(hashArg.Value.(types.Text).Value)
	if err != nil || len(expected) == 0 {
		return types.Boolean{Value: false}
	}
	salt, err := hex.DecodeString(saltArg.Value.(types.Text).Value)
	if err != nil || len(salt) == 0 {
		return types.Boolean{Value: false}
	}

	candidate := argon2.IDKey([]byte(pw.Value.(types.Text).Value), salt,
		argon2Time, argon2Memory, argon2Threads, uint32(len(expected)))

	return types.Boolean{Value: subtle.ConstantTimeCompare(candidate, expected) == 1}
}

// evalGenerateToken implements my.computer.crypto.generate_token().
// Returns 32 bytes of CSPRNG output as a 64-character lowercase hex string.
func evalGenerateToken(args []interface{}) interface{} {
	if len(args) != 0 {
		return types.Unknown{Reason: "generate_token takes no arguments"}
	}

	buf := make([]byte, tokenByteLen)
	if _, err := rand.Read(buf); err != nil {
		return types.Unknown{Reason: fmt.Sprintf("generate_token failed: %v", err)}
	}
	return types.Text{Value: hex.EncodeToString(buf)}
}
