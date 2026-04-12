package auth

import (
	"github.com/dhenkes/argon2id"
	"github.com/dhenkes/binge-os-watch/internal/model"
)

// Argon2idParams holds the configurable parameters for Argon2id hashing.
type Argon2idParams = argon2id.Params

// PasswordHasher implements model.PasswordHasher using the argon2id package.
type PasswordHasher struct {
	params argon2id.Params
}

var _ model.PasswordHasher = (*PasswordHasher)(nil)

func NewPasswordHasher(params argon2id.Params) *PasswordHasher {
	return &PasswordHasher{params: params}
}

func (h *PasswordHasher) Hash(password string) (string, error) {
	return argon2id.Hash(password, h.params)
}

func (h *PasswordHasher) Verify(password, hash string) (bool, error) {
	return argon2id.Verify(password, hash)
}
