package ulid

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/oklog/ulid"
)

func randomSource() *rand.Rand {
	return rand.New(rand.NewSource(time.Now().UnixNano())) //nolint: gosec // It's not a security context
}

// New will create a new ulid based ID service using the default random source.
func New() ports.IDService {
	return &ulidIDService{
		rnd: randomSource(),
	}
}

// New will create a new ulid based ID service using the supplied random source.
func NewWithRand(rnd *rand.Rand) ports.IDService {
	return &ulidIDService{
		rnd: rnd,
	}
}

type ulidIDService struct {
	rnd *rand.Rand
}

// GenerateRandom will generate a random identifier using ulid.
func (u *ulidIDService) GenerateRandom() (string, error) {
	entropy := ulid.Monotonic(u.rnd, 0)

	newID, err := ulid.New(ulid.Now(), entropy)
	if err != nil {
		return "", fmt.Errorf("generating microvm id: %w", err)
	}

	return newID.String(), nil
}
