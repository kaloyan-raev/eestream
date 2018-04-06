package eestream

import (
	"encoding/binary"

	"golang.org/x/crypto/nacl/secretbox"
)

type SecretboxEncrypter struct {
	blockSize int
	key       [32]byte
}

func setKey(dst *[32]byte, key []byte) error {
	if len((*dst)[:]) != len(key) {
		return Error.New("invalid key length, expected %d", len((*dst)[:]))
	}
	copy((*dst)[:], key)
	return nil
}

func NewSecretboxEncrypter(key []byte, encryptedBlockSize int) (
	*SecretboxEncrypter, error) {
	if encryptedBlockSize <= secretbox.Overhead {
		return nil, Error.New("block size too small")
	}
	rv := &SecretboxEncrypter{blockSize: encryptedBlockSize - secretbox.Overhead}
	return rv, setKey(&rv.key, key)
}

func (s *SecretboxEncrypter) InBlockSize() int {
	return s.blockSize
}

func (s *SecretboxEncrypter) OutBlockSize() int {
	return s.blockSize + secretbox.Overhead
}

func calcNonce(blockNum int64) *[24]byte {
	var buf [uint32Size]byte
	binary.BigEndian.PutUint32(buf[:], uint32(blockNum))
	var nonce [24]byte
	copy(nonce[:], buf[1:])
	return &nonce
}

func (s *SecretboxEncrypter) Transform(out, in []byte, blockNum int64) (
	[]byte, error) {
	return secretbox.Seal(out, in, calcNonce(blockNum), &s.key), nil
}

type SecretboxDecrypter struct {
	blockSize int
	key       [32]byte
}

func NewSecretboxDecrypter(key []byte, encryptedBlockSize int) (
	*SecretboxDecrypter, error) {
	if encryptedBlockSize <= secretbox.Overhead {
		return nil, Error.New("block size too small")
	}
	rv := &SecretboxDecrypter{blockSize: encryptedBlockSize - secretbox.Overhead}
	return rv, setKey(&rv.key, key)
}

func (s *SecretboxDecrypter) InBlockSize() int {
	return s.blockSize + secretbox.Overhead
}

func (s *SecretboxDecrypter) OutBlockSize() int {
	return s.blockSize
}

func (s *SecretboxDecrypter) Transform(out, in []byte, blockNum int64) (
	[]byte, error) {
	rv, success := secretbox.Open(out, in, calcNonce(blockNum), &s.key)
	if !success {
		return nil, Error.New("failed decrypting")
	}
	return rv, nil
}
