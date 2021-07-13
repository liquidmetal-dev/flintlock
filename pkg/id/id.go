package id

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/oklog/ulid"
)

// New will generate a new unique identifer with a random source based on the unix time now.
func New() (string, error) {
	return NewWithRand(rand.New(rand.NewSource(time.Now().UnixNano())))
}

// NewWithRand will generate a unique identifer with a specific random source.
func NewWithRand(rnd *rand.Rand) (string, error) {
	entropy := ulid.Monotonic(rnd, 0)
	newID, err := ulid.New(ulid.Now(), entropy)
	if err != nil {
		return "", fmt.Errorf("generating microvm id: %w", err)
	}

	return newID.String(), nil
}
