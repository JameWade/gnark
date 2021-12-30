package div

import (
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
)

type DivCircuit struct {
	A              frontend.Variable
	B              frontend.Variable
	ExpectedResult frontend.Variable `gnark:"data,public"`
}

func (divCircuit *DivCircuit) Define(curveId ecc.ID, cs frontend.API) error {
	a := divCircuit.A
	b := divCircuit.B
	//c := cs.Div2(a, b)
	c := cs.Div3(a, b) //variable.WitnessValue is set. this is illegal in Define
	cs.AssertIsEqual(c, divCircuit.ExpectedResult)
	return nil
}
