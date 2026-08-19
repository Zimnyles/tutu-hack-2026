package security

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordSaltLength = 16
	argonTime          = 3
	argonMemory        = 64 * 1024
	argonThreads       = 2
	argonKeyLength     = 32
	passwordHashParts  = 6
	algorithmIndex     = 1
	versionIndex       = 2
	parametersIndex    = 3
	saltPartIndex      = 4
	hashPartIndex      = 5
	parameterParts     = 3
)

var (
	ErrPasswordHashFormat = errors.New("unsupported password hash format")

	dummyHash = mustDummyHash() //nolint:gochecknoglobals // constant cost reference hash.
)

type argonParameters struct {
	memory  uint32
	time    uint32
	threads uint8
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		argonTime,
		argonMemory,
		argonThreads,
		argonKeyLength,
	)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonTime,
		argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func VerifyPassword(encoded, password string) bool {
	salt, want, parameters, err := parsePasswordHash(encoded)
	if err != nil {
		return false
	}

	got := argon2.IDKey(
		[]byte(password),
		salt,
		parameters.time,
		parameters.memory,
		parameters.threads,
		uint32(len(want)),
	)

	return subtle.ConstantTimeCompare(got, want) == 1
}

func NeedsRehash(encoded string) bool {
	_, _, parameters, err := parsePasswordHash(encoded)
	if err != nil {
		return true
	}

	return parameters.time < argonTime ||
		parameters.memory < argonMemory ||
		parameters.threads < argonThreads
}

func EqualizeVerificationCost(password string) {
	_ = VerifyPassword(dummyHash, password)
}

func parsePasswordHash(encoded string) ([]byte, []byte, argonParameters, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != passwordHashParts || parts[algorithmIndex] != "argon2id" {
		return nil, nil, argonParameters{}, ErrPasswordHashFormat
	}

	var version int
	if _, err := fmt.Sscanf(parts[versionIndex], "v=%d", &version); err != nil ||
		version != argon2.Version {
		return nil, nil, argonParameters{}, ErrPasswordHashFormat
	}

	parameters, err := parseArgonParameters(parts[parametersIndex])
	if err != nil {
		return nil, nil, argonParameters{}, err
	}

	salt, saltError := base64.RawStdEncoding.DecodeString(parts[saltPartIndex])
	want, hashError := base64.RawStdEncoding.DecodeString(parts[hashPartIndex])

	if saltError != nil || hashError != nil || len(salt) == 0 || len(want) == 0 {
		return nil, nil, argonParameters{}, ErrPasswordHashFormat
	}

	return salt, want, parameters, nil
}

func parseArgonParameters(raw string) (argonParameters, error) {
	segments := strings.Split(raw, ",")
	if len(segments) != parameterParts {
		return argonParameters{}, ErrPasswordHashFormat
	}

	var parameters argonParameters

	if _, err := fmt.Sscanf(segments[0], "m=%d", &parameters.memory); err != nil {
		return argonParameters{}, ErrPasswordHashFormat
	}

	if _, err := fmt.Sscanf(segments[1], "t=%d", &parameters.time); err != nil {
		return argonParameters{}, ErrPasswordHashFormat
	}

	if _, err := fmt.Sscanf(segments[2], "p=%d", &parameters.threads); err != nil {
		return argonParameters{}, ErrPasswordHashFormat
	}

	if parameters.memory == 0 || parameters.time == 0 || parameters.threads == 0 {
		return argonParameters{}, ErrPasswordHashFormat
	}

	return parameters, nil
}

func mustDummyHash() string {
	hash, err := HashPassword("argon2id-cost-reference")
	if err != nil {
		panic(err)
	}

	return hash
}
