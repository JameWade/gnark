package frontend

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend"
	"github.com/consensys/gnark/debug"
	"github.com/consensys/gnark/internal/backend/compiled"
)

// ConstraintSystem contains the parts common to plonk and Groth16
type ConstraintSystem struct {
	compiled.CS

	// input wires
	Public, Secret []string

	curveID   ecc.ID
	backendID backend.ID

	// Coefficients in the constraints
	Coeffs         []big.Int      // list of unique coefficients.
	CoeffsIDsLarge map[string]int // map to check existence of a coefficient (key = coeff.Bytes())
	CoeffsIDsInt64 map[int64]int  // map to check existence of a coefficient (key = int64 value)
}

func (cs *ConstraintSystem) CoeffID64(v int64) int {
	if resID, ok := cs.CoeffsIDsInt64[v]; ok {
		return resID
	} else {
		var bCopy big.Int
		bCopy.SetInt64(v)
		resID := len(cs.Coeffs)
		cs.Coeffs = append(cs.Coeffs, bCopy)
		cs.CoeffsIDsInt64[v] = resID
		return resID
	}
}

// CoeffID tries to fetch the entry where b is if it exits, otherwise appends b to
// the list of Coeffs and returns the corresponding entry
func (cs *ConstraintSystem) CoeffID(b *big.Int) int {

	// if the coeff is a int64 we have a fast path.
	if b.IsInt64() {
		return cs.CoeffID64(b.Int64())
	}

	// GobEncode is 3x faster than b.Text(16). Slightly slower than Bytes, but Bytes return the same
	// thing for -x and x .
	bKey, _ := b.GobEncode()
	key := string(bKey)

	// if the coeff is already stored, fetch its ID from the cs.CoeffsIDs map
	if idx, ok := cs.CoeffsIDsLarge[key]; ok {
		return idx
	}

	// else add it in the cs.Coeffs map and update the cs.CoeffsIDs map
	var bCopy big.Int
	bCopy.Set(b)
	resID := len(cs.Coeffs)
	cs.Coeffs = append(cs.Coeffs, bCopy)
	cs.CoeffsIDsLarge[key] = resID
	return resID
}

func (cs *ConstraintSystem) AddDebugInfo(errName string, i ...interface{}) int {
	var l compiled.LogEntry

	const minLogSize = 500
	var sbb strings.Builder
	sbb.Grow(minLogSize)
	sbb.WriteString("[")
	sbb.WriteString(errName)
	sbb.WriteString("] ")

	for _, _i := range i {
		switch v := _i.(type) {
		case string:
			sbb.WriteString(v)
		case int:
			sbb.WriteString(strconv.Itoa(v))
		case compiled.Term:
			l.WriteTerm(v, &sbb)
		default:
			panic("unsupported log type")
		}
	}
	sbb.WriteByte('\n')
	debug.WriteStack(&sbb)
	l.Format = sbb.String()

	cs.DebugInfo = append(cs.DebugInfo, l)
	return len(cs.DebugInfo) - 1
}

// bitLen returns the number of bits needed to represent a fr.Element
func (cs *ConstraintSystem) BitLen() int {
	return cs.curveID.Info().Fr.Bits
}

// CurveID returns the ecc.ID injected by the compiler
func (cs *ConstraintSystem) CurveID() ecc.ID {
	return cs.curveID
}

// Backend returns the backend.ID injected by the compiler
func (cs *ConstraintSystem) BackendID() backend.ID {
	return cs.backendID
}