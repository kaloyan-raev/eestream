package eestream

type Transformer interface {
	BlockSize() int
	BlockOverhead() int
	Transform(out, in []byte, blockOffset int) []byte
}
