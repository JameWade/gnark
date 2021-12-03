package multihint

import (
	"testing"

	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/test"
)

func TestMultiHint(t *testing.T) {
	assert := test.NewAssert(t)

	var multiCircuit Circuit

	assert.ProverSucceeded(&multiCircuit, &Circuit{
		A0: 5,
		A1: 7,
		B0: 10,
		B1: 14,
	}, test.WithProverOpts(backend.WithHints(multiHint)))

	assert.ProverFailed(&multiCircuit, &Circuit{
		A0: 5,
		A1: 7,
		B0: 9,
		B1: 13,
	}, test.WithProverOpts(backend.WithHints(multiHint)))

}

func TestExpCircuit(t *testing.T) {
	assert := test.NewAssert(t)
	var expC ExpCircuit
	assert.ProverSucceeded(&expC, &ExpCircuit{
		X: 2,
		E: 12,
		Y: 4096,
	})
}

func TestRangeCircuit(t *testing.T) {
	assert := test.NewAssert(t)
	var rC rangeCheckCircuit
	assert.ProverSucceeded(&rC, &rangeCheckCircuit{
		X:     10,
		Y:     4,
		Bound: 161,
	})
}

func TestIsZero(t *testing.T) {
	assert := test.NewAssert(t)
	var c zeroCircuit
	assert.ProverSucceeded(&c, &zeroCircuit{
		X: 0,
		Y: 0,
	})
}
