/*
Copyright © 2021 ConsenSys Software Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plonk

import (
	"fmt"
	"math/big"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/backend/hint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/internal/backend/compiled"
	"github.com/consensys/gnark/internal/parser"
)

// API represents the available functions to circuit developers

// Add returns res = i1+i2+...in
func (cs *SparseR1CRefactor) Add(i1, i2 interface{}, in ...interface{}) frontend.Variable {

	zero := big.NewInt(0)
	vars, k := cs.filterConstantSum(append([]interface{}{i1, i2}, in...))
	if len(vars) == 0 {
		return k
	}
	if k.Cmp(zero) == 0 {
		return cs.splitSum(vars[0], vars[1:])
	}
	cl, _, _ := vars[0].Unpack()
	kID := cs.CoeffID(&k)
	o := cs.newInternalVariable()
	cs.addPlonkConstraint(vars[0], 0, o, cl, compiled.CoeffIdZero, compiled.CoeffIdZero, compiled.CoeffIdZero, compiled.CoeffIdMinusOne, kID)
	return cs.splitSum(o, vars[1:])

}

// neg returns -in...
func (cs *SparseR1CRefactor) neg(in ...interface{}) []frontend.Variable {

	res := make([]frontend.Variable, len(in))

	for i := 0; i < len(in); i++ {
		res[i] = cs.Neg(in[i])
	}
	return res
}

// Sub returns res = i1 - i2 - ...in
func (cs *SparseR1CRefactor) Sub(i1, i2 interface{}, in ...interface{}) frontend.Variable {
	r := cs.neg(append([]interface{}{i2}, in...))
	return cs.Add(i1, r[0], r[1:])
}

// Neg returns -i
func (cs *SparseR1CRefactor) Neg(i1 interface{}) frontend.Variable {
	if cs.IsConstant(i1) {
		k := cs.ConstantValue(i1)
		k.Neg(k)
		return *k
	} else {
		v := i1.(compiled.Term)
		c, _, _ := v.Unpack()
		coef := cs.Coeffs[c]
		coef.Neg(&coef)
		c = cs.CoeffID(&coef)
		v.SetCoeffID(c)
		return v
	}
}

// Mul returns res = i1 * i2 * ... in
func (cs *SparseR1CRefactor) Mul(i1, i2 interface{}, in ...interface{}) frontend.Variable {

	zero := big.NewInt(0)

	vars, k := cs.filterConstantProd(append([]interface{}{i1, i2}, in...))
	if len(vars) == 0 {
		return k
	}
	if k.Cmp(zero) == 0 {
		return cs.splitProd(vars[0], vars[1:])
	}
	l := cs.mulConstant(vars[0], &k)
	return cs.splitProd(l, vars[1:])

}

// returns t*m
func (cs *SparseR1CRefactor) mulConstant(t compiled.Term, m *big.Int) compiled.Term {
	cid, _, _ := t.Unpack()
	coef := cs.Coeffs[cid]
	coef.Mul(m, &coef).Mod(&coef, cs.CurveID().Info().Fr.Modulus())
	cid = cs.CoeffID(&coef)
	t.SetCoeffID(cid)
	return t
}

// returns t/m
func (cs *SparseR1CRefactor) divConstant(t compiled.Term, m *big.Int) compiled.Term {
	cid, _, _ := t.Unpack()
	coef := cs.Coeffs[cid]
	var _m big.Int
	q := cs.CurveID().Info().Fr.Modulus()
	_m.Set(m).
		ModInverse(&_m, q).
		Mul(&_m, &coef).
		Mod(&_m, q)
	cid = cs.CoeffID(&coef)
	t.SetCoeffID(cid)
	return t
}

// DivUnchecked returns i1 / i2 . if i1 == i2 == 0, returns 0
func (cs *SparseR1CRefactor) DivUnchecked(i1, i2 interface{}) frontend.Variable {
	if cs.IsConstant(i1) && cs.IsConstant(i2) {
		l := frontend.FromInterface(i1)
		r := frontend.FromInterface(i2)
		q := cs.CurveID().Info().Fr.Modulus()
		return r.ModInverse(&r, q).
			Mul(&l, &r).
			Mod(&l, q)
	}
	if cs.IsConstant(i2) {
		c := frontend.FromInterface(i2)
		t := i1.(compiled.Term)
		return cs.divConstant(t, &c)
	}
	if cs.IsConstant(i1) {
		t := i2.(compiled.Term)
		cidr, _, _ := t.Unpack()
		res := cs.newInternalVariable()
		c := frontend.FromInterface(i1)
		cidl := cs.CoeffID(&c)
		cs.addPlonkConstraint(res, t, 0, compiled.CoeffIdZero, compiled.CoeffIdZero, cidl, cidr, compiled.CoeffIdZero, compiled.CoeffIdMinusOne)
		return res
	}
	res := cs.newInternalVariable()
	t1 := i1.(compiled.Term)
	t2 := i2.(compiled.Term)
	cl, _, _ := t1.Unpack()
	cr, _, _ := t2.Unpack()
	cs.addPlonkConstraint(t1, t2, 0, compiled.CoeffIdZero, compiled.CoeffIdZero, cl, cr, compiled.CoeffIdZero, compiled.CoeffIdMinusOne)
	return res
}

// Div returns i1 / i2
func (cs *SparseR1CRefactor) Div(i1, i2 interface{}) frontend.Variable {
	// TODO check that later
	return cs.DivUnchecked(i1, i2)
}

// Inverse returns res = 1 / i1
func (cs *SparseR1CRefactor) Inverse(i1 interface{}) frontend.Variable {
	if cs.IsConstant(i1) {
		c := frontend.FromInterface(i1)
		c.ModInverse(&c, cs.CurveID().Info().Fr.Modulus())
		return c
	}
	t := i1.(compiled.Term)
	cr, _, _ := t.Unpack()
	res := cs.newInternalVariable()
	cs.addPlonkConstraint(res, t, 0, compiled.CoeffIdZero, compiled.CoeffIdZero, compiled.CoeffIdOne, cr, compiled.CoeffIdZero, compiled.CoeffIdMinusOne)
	return res
}

// ---------------------------------------------------------------------------------------------
// Bit operations

// ToBinary unpacks a frontend.Variable in binary,
// n is the number of bits to select (starting from lsb)
// n default value is fr.Bits the number of bits needed to represent a field element
//
// The result in in little endian (first bit= lsb)
func (cs *SparseR1CRefactor) ToBinary(i1 interface{}, n ...int) []frontend.Variable {

	// nbBits
	nbBits := cs.BitLen()
	if len(n) == 1 {
		nbBits = n[0]
		if nbBits < 0 {
			panic("invalid n")
		}
	}

	// if a is a constant, work with the big int value.
	if cs.IsConstant(i1) {
		c := frontend.FromInterface(i1)
		b := make([]frontend.Variable, nbBits)
		for i := 0; i < len(b); i++ {
			b[i] = c.Bit(i)
		}
		return b
	}

	a := i1.(compiled.Term)
	return cs.toBinary(a, nbBits, false)
}

func (cs *SparseR1CRefactor) toBinary(a compiled.Term, nbBits int, unsafe bool) []frontend.Variable {

	// allocate the resulting frontend.Variables and bit-constraint them
	b := make([]frontend.Variable, nbBits)
	sb := make([]interface{}, nbBits)
	var c big.Int
	c.SetUint64(1)
	for i := 0; i < nbBits; i++ {
		b[i] = cs.NewHint(hint.IthBit, a, i)
		sb[i] = cs.Mul(b[i], c)
		c.Lsh(&c, 1)
		if !unsafe {
			cs.AssertIsBoolean(b[i])
		}
	}

	//var Σbi compiled.Term
	var Σbi frontend.Variable
	if nbBits == 1 {
		cs.AssertIsEqual(sb[0], a)
	} else if nbBits == 2 {
		Σbi = cs.Add(sb[0], sb[1])
	} else {
		Σbi = cs.Add(sb[0], sb[1], sb[2:]...)
	}
	cs.AssertIsEqual(Σbi, a)

	// record the constraint Σ (2**i * b[i]) == a
	return b

}

// FromBinary packs b, seen as a fr.Element in little endian
func (cs *SparseR1CRefactor) FromBinary(b ...interface{}) frontend.Variable {
	_b := make([]frontend.Variable, len(b))
	var c big.Int
	c.SetUint64(1)
	for i := 0; i < len(b); i++ {
		_b[0] = cs.Mul(b[i], c)
		c.Lsh(&c, 1)
	}
	if len(b) == 1 {
		return b[0]
	}
	if len(b) == 1 {
		return cs.Add(_b[0], _b[1])
	}
	return cs.Add(_b[0], _b[1], _b[2:])
}

// Xor returns a ^ b
// a and b must be 0 or 1
func (cs *SparseR1CRefactor) Xor(a, b frontend.Variable) frontend.Variable {
	if cs.IsConstant(a) && cs.IsConstant(b) {
		_a := frontend.FromInterface(a)
		_b := frontend.FromInterface(b)
		_a.Xor(&_a, &_b)
		return _a
	}
	res := cs.newInternalVariable()
	if cs.IsConstant(a) {
		a, b = b, a
	}
	if cs.IsConstant(b) {
		l := a.(compiled.Term)
		r := l
		_b := frontend.FromInterface(b)
		one := big.NewInt(1)
		_b.Lsh(&_b, 1).Sub(&_b, one)
		idl := cs.CoeffID(&_b)
		cs.addPlonkConstraint(l, r, res, idl, compiled.CoeffIdZero, compiled.CoeffIdZero, compiled.CoeffIdZero, compiled.CoeffIdOne, compiled.CoeffIdZero)
		return res
	}
	l := a.(compiled.Term)
	r := b.(compiled.Term)
	cs.addPlonkConstraint(l, r, res, compiled.CoeffIdMinusOne, compiled.CoeffIdMinusOne, compiled.CoeffIdTwo, compiled.CoeffIdOne, compiled.CoeffIdOne, compiled.CoeffIdZero)
	return res
}

// Or returns a | b
// a and b must be 0 or 1
func (cs *SparseR1CRefactor) Or(a, b frontend.Variable) frontend.Variable {
	if cs.IsConstant(a) && cs.IsConstant(b) {
		_a := frontend.FromInterface(a)
		_b := frontend.FromInterface(b)
		_a.Or(&_a, &_b)
		return _a
	}
	res := cs.newInternalVariable()
	if cs.IsConstant(a) {
		a, b = b, a
	}
	if cs.IsConstant(b) {
		l := a.(compiled.Term)
		r := l
		_b := frontend.FromInterface(b)
		one := big.NewInt(1)
		_b.Sub(&_b, one)
		idl := cs.CoeffID(&_b)
		cs.addPlonkConstraint(l, r, res, idl, compiled.CoeffIdZero, compiled.CoeffIdZero, compiled.CoeffIdZero, compiled.CoeffIdOne, compiled.CoeffIdZero)
		return res
	}
	l := a.(compiled.Term)
	r := b.(compiled.Term)
	cs.addPlonkConstraint(l, r, res, compiled.CoeffIdMinusOne, compiled.CoeffIdMinusOne, compiled.CoeffIdOne, compiled.CoeffIdOne, compiled.CoeffIdOne, compiled.CoeffIdZero)
	return res
}

// Or returns a & b
// a and b must be 0 or 1
func (cs *SparseR1CRefactor) And(a, b frontend.Variable) frontend.Variable {
	return cs.Mul(a, b)
}

// ---------------------------------------------------------------------------------------------
// Conditionals

// Select if b is true, yields i1 else yields i2
func (cs *SparseR1CRefactor) Select(b interface{}, i1, i2 interface{}) frontend.Variable {

	if cs.IsConstant(b) {
		_b := frontend.FromInterface(b)
		var t big.Int
		one := big.NewInt(1)
		if _b.Cmp(&t) != 0 && _b.Cmp(one) != 0 {
			panic("b should be a boolean")
		}
		if _b.Cmp(&t) == 0 {
			return i2
		}
		return i1
	}

	u := cs.Sub(i2, i1)
	l := cs.Mul(u, b)
	res := cs.newInternalVariable()
	if cs.IsConstant(i2) {
		k := frontend.FromInterface(i2)
		_k := cs.CoeffID(&k)
		cs.addPlonkConstraint(l, 0, res, compiled.CoeffIdOne, compiled.CoeffIdZero, compiled.CoeffIdZero, compiled.CoeffIdZero, compiled.CoeffIdZero, _k)
	} else {
		_r := i2.(compiled.Term)
		cs.addPlonkConstraint(l, _r, res, compiled.CoeffIdOne, compiled.CoeffIdMinusOne, compiled.CoeffIdZero, compiled.CoeffIdZero, compiled.CoeffIdOne, compiled.CoeffIdZero)
	}
	return res
}

// Lookup2 performs a 2-bit lookup between i1, i2, i3, i4 based on bits b0
// and b1. Returns i0 if b0=b1=0, i1 if b0=1 and b1=0, i2 if b0=0 and b1=1
// and i3 if b0=b1=1.
func (cs *SparseR1CRefactor) Lookup2(b0, b1 interface{}, i0, i1, i2, i3 interface{}) frontend.Variable {
	return 0
}

// IsZero returns 1 if a is zero, 0 otherwise
func (cs *SparseR1CRefactor) IsZero(i1 interface{}) frontend.Variable {

	if cs.IsConstant(i1) {
		a := frontend.FromInterface(i1)
		var zero big.Int
		if a.Cmp(&zero) != 0 {
			panic("input should be zero")
		}
		return 1
	}

	a := i1.(compiled.Term)
	m := cs.NewHint(hint.IsZero, a)
	cs.AssertIsBoolean(i1)
	cs.addPlonkConstraint(a, m, 0, compiled.CoeffIdZero, compiled.CoeffIdZero, compiled.CoeffIdOne, compiled.CoeffIdOne, compiled.CoeffIdZero, compiled.CoeffIdZero)
	ma := cs.Add(m, a)
	cs.Inverse(ma)
	return m
}

// Println behaves like fmt.Println but accepts frontend.frontend.Variable as parameter
// whose value will be resolved at runtime when computed by the solver
// Println enables circuit debugging and behaves almost like fmt.Println()
//
// the print will be done once the R1CS.Solve() method is executed
//
// if one of the input is a variable, its value will be resolved avec R1CS.Solve() method is called
func (cs *SparseR1CRefactor) Println(a ...interface{}) {
	var sbb strings.Builder

	// prefix log line with file.go:line
	if _, file, line, ok := runtime.Caller(1); ok {
		sbb.WriteString(filepath.Base(file))
		sbb.WriteByte(':')
		sbb.WriteString(strconv.Itoa(line))
		sbb.WriteByte(' ')
	}

	var log compiled.LogEntry

	for i, arg := range a {
		if i > 0 {
			sbb.WriteByte(' ')
		}
		if v, ok := arg.(compiled.Term); ok {

			sbb.WriteString("%s")
			// we set limits to the linear expression, so that the log printer
			// can evaluate it before printing it
			log.ToResolve = append(log.ToResolve, compiled.TermDelimitor)
			log.ToResolve = append(log.ToResolve, v)
			log.ToResolve = append(log.ToResolve, compiled.TermDelimitor)
		} else {
			printArg(&log, &sbb, arg)
		}
	}
	sbb.WriteByte('\n')

	// set format string to be used with fmt.Sprintf, once the variables are solved in the R1CS.Solve() method
	log.Format = sbb.String()

	cs.Logs = append(cs.Logs, log)
}

func printArg(log *compiled.LogEntry, sbb *strings.Builder, a interface{}) {

	count := 0
	counter := func(visibility compiled.Visibility, name string, tValue reflect.Value) error {
		count++
		return nil
	}
	// ignoring error, counter() always return nil
	_ = parser.Visit(a, "", compiled.Unset, counter, tVariable)

	// no variables in nested struct, we use fmt std print function
	if count == 0 {
		sbb.WriteString(fmt.Sprint(a))
		return
	}

	sbb.WriteByte('{')
	printer := func(visibility compiled.Visibility, name string, tValue reflect.Value) error {
		count--
		sbb.WriteString(name)
		sbb.WriteString(": ")
		sbb.WriteString("%s")
		if count != 0 {
			sbb.WriteString(", ")
		}

		v := tValue.Interface().(compiled.Variable)
		// we set limits to the linear expression, so that the log printer
		// can evaluate it before printing it
		log.ToResolve = append(log.ToResolve, compiled.TermDelimitor)
		log.ToResolve = append(log.ToResolve, v.LinExp...)
		log.ToResolve = append(log.ToResolve, compiled.TermDelimitor)
		return nil
	}
	// ignoring error, printer() doesn't return errors
	_ = parser.Visit(a, "", compiled.Unset, printer, tVariable)
	sbb.WriteByte('}')
}

// Tag creates a tag at a given place in a circuit. The state of the tag may contain informations needed to
// measure constraints, variables and coefficients creations through AddCounter
func (cs *SparseR1CRefactor) Tag(name string) frontend.Tag {
	_, file, line, _ := runtime.Caller(1)

	return frontend.Tag{
		Name: fmt.Sprintf("%s[%s:%d]", name, filepath.Base(file), line),
		VID:  cs.NbInternalVariables,
		CID:  len(cs.Constraints),
	}
}

// AddCounter measures the number of constraints, variables and coefficients created between two tags
// note that the PlonK statistics are contextual since there is a post-compile phase where linear expressions
// are factorized. That is, measuring 2 times the "repeating" piece of circuit may give less constraints the second time
func (cs *SparseR1CRefactor) AddCounter(from, to frontend.Tag) {
	cs.Counters = append(cs.Counters, compiled.Counter{
		From:          from.Name,
		To:            to.Name,
		NbVariables:   to.VID - from.VID,
		NbConstraints: to.CID - from.CID,
		CurveID:       cs.CurveID(),
		BackendID:     backend.PLONK,
	})
}

// IsConstant returns true if v is a constant known at compile time
func (cs *SparseR1CRefactor) IsConstant(v frontend.Variable) bool {
	switch t := v.(type) {
	case compiled.Term:
		return false
	default:
		frontend.FromInterface(t)
		return true
	}
}

// ConstantValue returns the big.Int value of v. It
// panics if v.IsConstant() == false
func (cs *SparseR1CRefactor) ConstantValue(v frontend.Variable) *big.Int {
	if !cs.IsConstant(v) {
		panic("v should be a constant")
	}
	res := frontend.FromInterface(v)
	return &res
}

// returns in split into a slice of compiledTerm and the sum of all constants in in as a bigInt
func (cs *SparseR1CRefactor) filterConstantSum(in ...interface{}) ([]compiled.Term, big.Int) {
	res := make([]compiled.Term, 0, len(in))
	var b big.Int
	for i := 0; i < len(in); i++ {
		switch t := in[i].(type) {
		case compiled.Term:
			res = append(res, t)
		default:
			n := frontend.FromInterface(t)
			b.Add(&b, &n)
		}
	}
	return res, b
}

// returns in split into a slice of compiledTerm and the product of all constants in in as a bigInt
func (cs *SparseR1CRefactor) filterConstantProd(in ...interface{}) ([]compiled.Term, big.Int) {
	res := make([]compiled.Term, 0, len(in))
	var b big.Int
	b.SetInt64(1)
	for i := 0; i < len(in); i++ {
		switch t := in[i].(type) {
		case compiled.Term:
			res = append(res, t)
		default:
			n := frontend.FromInterface(t)
			b.Mul(&b, &n)
		}
	}
	return res, b
}

func (cs *SparseR1CRefactor) splitSum(acc compiled.Term, r []compiled.Term) compiled.Term {

	// floor case
	if len(r) == 0 {
		return acc
	}

	cl, _, _ := acc.Unpack()
	cr, _, _ := r[0].Unpack()
	o := cs.newInternalVariable()
	cs.addPlonkConstraint(acc, r[0], o, cl, cr, compiled.CoeffIdZero, compiled.CoeffIdZero, compiled.CoeffIdMinusOne, compiled.CoeffIdZero)
	return cs.splitSum(o, r[1:])
}

func (cs *SparseR1CRefactor) splitProd(acc compiled.Term, r []compiled.Term) compiled.Term {

	// floor case
	if len(r) == 0 {
		return acc
	}

	cl, _, _ := acc.Unpack()
	cr, _, _ := r[0].Unpack()
	o := cs.newInternalVariable()
	cs.addPlonkConstraint(acc, r[0], o, compiled.CoeffIdZero, compiled.CoeffIdZero, cl, cr, compiled.CoeffIdMinusOne, compiled.CoeffIdZero)
	return cs.splitProd(o, r[1:])
}