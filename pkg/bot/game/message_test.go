package game

import (
	"math/rand"
	"testing"
)

func TestPrepareWordPairMessageProducesBothLayouts(t *testing.T) {
	expected := map[string]bool{
		"Hola  ||_Adios_||\n": false,
		"_Adios_  ||Hola||\n": false,
	}

	messageRand = rand.New(rand.NewSource(1))
	for range 10 {
		msg := PrepareWordPairMessage("Hola", "Adios")
		if _, ok := expected[msg]; ok {
			expected[msg] = true
		}
	}

	for layout, seen := range expected {
		if !seen {
			t.Fatalf("expected layout %q was not produced", layout)
		}
	}
}
