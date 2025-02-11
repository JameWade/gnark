package test

import (
	"crypto/rand"
	"math/big"
	mrand "math/rand"
	"reflect"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/internal/backend/compiled"
	"github.com/consensys/gnark/internal/parser"
)

var seedCorpus []*big.Int

func init() {
	seedCorpus = make([]*big.Int, 0, 300)

	// small values, including bits
	for i := -5; i <= 5; i++ {
		seedCorpus = append(seedCorpus, big.NewInt(int64(i)))
	}

	// moduli
	for _, curve := range ecc.Implemented() {
		fp := curve.Info().Fp.Modulus()
		fr := curve.Info().Fr.Modulus()
		seedCorpus = append(seedCorpus, fp)
		seedCorpus = append(seedCorpus, fr)

		var bi big.Int
		for i := -3; i <= 3; i++ {
			bi.SetInt64(int64(i))
			var fp1, fr1 big.Int
			fp1.Add(fp, &bi)
			fr1.Add(fr, &bi)

			seedCorpus = append(seedCorpus, &fp1)
			seedCorpus = append(seedCorpus, &fr1)
		}
	}

	// powers of 2
	bi := big.NewInt(1)
	bi.Lsh(bi, 32)
	seedCorpus = append(seedCorpus, bi)

	bi = big.NewInt(1)
	bi.Lsh(bi, 64)
	seedCorpus = append(seedCorpus, bi)

	bi = big.NewInt(1)
	bi.Lsh(bi, 254)
	seedCorpus = append(seedCorpus, bi)

	bi = big.NewInt(1)
	bi.Lsh(bi, 255)
	seedCorpus = append(seedCorpus, bi)

	bi = big.NewInt(1)
	bi.Lsh(bi, 256)
	seedCorpus = append(seedCorpus, bi)

}

type filler func(frontend.Circuit, ecc.ID)

func zeroFiller(w frontend.Circuit, curve ecc.ID) {
	fill(w, func() interface{} {
		return 0
	})
}

func binaryFiller(w frontend.Circuit, curve ecc.ID) {
	mrand.Seed(time.Now().Unix())

	fill(w, func() interface{} {
		return int(mrand.Uint32() % 2)
	})
}

func seedFiller(w frontend.Circuit, curve ecc.ID) {

	mrand.Seed(time.Now().Unix())

	m := curve.Info().Fr.Modulus()

	fill(w, func() interface{} {
		i := int(mrand.Uint32() % uint32(len(seedCorpus)))
		r := new(big.Int).Set(seedCorpus[i])
		return r.Mod(r, m)
	})
}

func randomFiller(w frontend.Circuit, curve ecc.ID) {

	mrand.Seed(time.Now().Unix())

	r := mrand.New(mrand.NewSource(time.Now().Unix()))
	m := curve.Info().Fr.Modulus()

	fill(w, func() interface{} {
		i := int(mrand.Uint32() % uint32(len(seedCorpus)*2))
		if i >= len(seedCorpus) {
			b1, _ := rand.Int(r, m)
			return b1
		}
		r := new(big.Int).Set(seedCorpus[i])
		return r.Mod(r, m)
	})
}

func fill(w frontend.Circuit, nextValue func() interface{}) {
	var setHandler parser.LeafHandler = func(visibility compiled.Visibility, name string, tInput reflect.Value) error {
		if visibility == compiled.Secret || visibility == compiled.Public {
			v := nextValue()
			tInput.Set(reflect.ValueOf(frontend.Value(v)))
		}
		return nil
	}
	// this can't error.
	_ = parser.Visit(w, "", compiled.Unset, setHandler, reflect.TypeOf(frontend.Variable{}))
}
