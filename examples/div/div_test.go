package div

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"testing"
)

func TestDivEquation(t *testing.T) {
	assert := test.NewAssert(t)

	var divCircuit DivCircuit

	//assert.ProverSucceeded(&divCircuit, &DivCircuit{
	//	A:              frontend.Value(42),
	//	B:              frontend.Value(2),
	//	ExpectedResult: frontend.Value(21),
	//}) //constraint is not satisfied
	assert.ProverSucceeded(&divCircuit, &DivCircuit{
		A:              frontend.Value(42),
		B:              frontend.Value(4),
		ExpectedResult: frontend.Value(10),
	}) //10944121435919637611123202872628637544274182200208017171849102093287904247819 == 10
}
