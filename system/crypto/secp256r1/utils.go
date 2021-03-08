// Copyright Fuzamei Corp. 2018 All Rights Reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecdsa

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/asn1"
	"errors"
	"fmt"
	"math/big"
)

const (
	pubkeyUncompressed byte = 0x4 // x coord + y coord
)

// MarshalECDSASignature marshal ECDSA signature
func MarshalECDSASignature(r, s *big.Int) ([]byte, error) {
	return asn1.Marshal(signatureECDSA{r, s})
}

// UnmarshalECDSASignature unmarshal ECDSA signature
func UnmarshalECDSASignature(raw []byte) (*big.Int, *big.Int, error) {
	sig := new(signatureECDSA)
	_, err := asn1.Unmarshal(raw, sig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed unmashalling signature [%s]", err)
	}

	if sig.R == nil {
		return nil, nil, errors.New("invalid signature, R must be different from nil")
	}
	if sig.S == nil {
		return nil, nil, errors.New("invalid signature, S must be different from nil")
	}

	if sig.R.Sign() != 1 {
		return nil, nil, errors.New("invalid signature, R must be larger than zero")
	}
	if sig.S.Sign() != 1 {
		return nil, nil, errors.New("invalid signature, S must be larger than zero")
	}

	return sig.R, sig.S, nil
}

// ToLowS convert to low int
func ToLowS(k *ecdsa.PublicKey, s *big.Int) *big.Int {
	lowS := IsLowS(s)
	if !lowS {
		s.Sub(k.Params().N, s)

		return s
	}

	return s
}

// IsLowS check is low int
func IsLowS(s *big.Int) bool {
	return s.Cmp(new(big.Int).Rsh(elliptic.P256().Params().N, 1)) != 1
}

func parsePubKey(pubKeyStr []byte) (key *ecdsa.PublicKey, err error) {
	pubkey := ecdsa.PublicKey{}
	pubkey.Curve = elliptic.P256()

	if len(pubKeyStr) == 0 {
		return nil, errors.New("pubkey string is empty")
	}

	pubkey.X = new(big.Int).SetBytes(pubKeyStr[1:33])
	pubkey.Y = new(big.Int).SetBytes(pubKeyStr[33:])
	if pubkey.X.Cmp(pubkey.Curve.Params().P) >= 0 {
		return nil, fmt.Errorf("pubkey X parameter is >= to P")
	}
	if pubkey.Y.Cmp(pubkey.Curve.Params().P) >= 0 {
		return nil, fmt.Errorf("pubkey Y parameter is >= to P")
	}
	if !pubkey.Curve.IsOnCurve(pubkey.X, pubkey.Y) {
		return nil, fmt.Errorf("pubkey isn't on secp256k1 curve")
	}
	return &pubkey, nil
}

// SerializePublicKeyCompressed serialize compressed publicKey
func SerializePublicKeyCompressed(p *ecdsa.PublicKey) []byte {
	byteLen := (elliptic.P256().Params().BitSize + 7) >> 3
	compressed := make([]byte, 1+byteLen)
	compressed[0] = byte(p.Y.Bit(0)) | 2

	xBytes := p.X.Bytes()
	copy(compressed[1:], xBytes)
	return compressed
}

// y² = x³ - 3x + b
func polynomial(B, P, x *big.Int) *big.Int {
	x3 := new(big.Int).Mul(x, x)
	x3.Mul(x3, x)

	threeX := new(big.Int).Lsh(x, 1)
	threeX.Add(threeX, x)

	x3.Sub(x3, threeX)
	x3.Add(x3, B)
	x3.Mod(x3, P)

	return x3
}

func parsePubKeyCompressed(data []byte) (*ecdsa.PublicKey, error) {
	curve := elliptic.P256()
	byteLen := (curve.Params().BitSize + 7) / 8
	if len(data) != 1+byteLen {
		return nil, errors.New("parsePubKeyCompressed byteLen error")
	}
	if data[0] != 2 && data[0] != 3 { // compressed form
		return nil, errors.New("parsePubKeyCompressed compressed form error")
	}
	p := curve.Params().P
	x := new(big.Int).SetBytes(data[1:])
	if x.Cmp(p) >= 0 {
		return nil, errors.New("parsePubKeyCompressed x data error")
	}

	y := polynomial(curve.Params().B, curve.Params().P, x)
	y = y.ModSqrt(y, p)
	if y == nil {
		return nil, errors.New("parsePubKeyCompressed y data error")
	}
	if byte(y.Bit(0)) != data[0]&1 {
		y.Neg(y).Mod(y, p)
	}
	if !curve.IsOnCurve(x, y) {
		return nil, errors.New("parsePubKeyCompressed IsOnCurve error")
	}

	pubkey := ecdsa.PublicKey{}
	pubkey.Curve = curve
	pubkey.X = x
	pubkey.Y = y
	return &pubkey, nil
}

// SerializePublicKey serialize public key
func SerializePublicKey(p *ecdsa.PublicKey) []byte {
	b := make([]byte, 0, publicKeyECDSALength)
	b = append(b, 0x4)
	b = paddedAppend(32, b, p.X.Bytes())
	return paddedAppend(32, b, p.Y.Bytes())
}

// SerializePrivateKey serialize private key
func SerializePrivateKey(p *ecdsa.PrivateKey) []byte {
	b := make([]byte, 0, privateKeyECDSALength)
	return paddedAppend(privateKeyECDSALength, b, p.D.Bytes())
}

func paddedAppend(size uint, dst, src []byte) []byte {
	for i := 0; i < int(size)-len(src); i++ {
		dst = append(dst, 0)
	}
	return append(dst, src...)
}
