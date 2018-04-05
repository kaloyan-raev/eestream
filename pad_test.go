package eestream

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestPad(t *testing.T) {
	for _, example := range []struct {
		data      string
		blockSize int64
		padding   int
	}{
		{"abcdef", 24, 24 - 6},
		{"abcdef", 6, 6},
		{"abcdef", 7, 1},
		{"abcde", 7, 2},
		{"abcdefg", 7, 7},
	} {
		padded, padding, err := Pad(ByteRangeReader([]byte(example.data)),
			example.blockSize)
		if err != nil {
			t.Fatalf("unexpected error")
		}
		if padding != example.padding {
			t.Fatalf("invalid padding")
		}
		if int64(padding+len(example.data)) != padded.Size() {
			t.Fatalf("invalid padding")
		}
		unpadded, err := Unpad(padded, padding)
		data, err := ioutil.ReadAll(unpadded.Range(0, unpadded.Size()))
		if err != nil {
			t.Fatalf("unexpected error")
		}
		if !bytes.Equal(data, []byte(example.data)) {
			t.Fatalf("mismatch")
		}
		unpadded, err = UnpadSlow(padded)
		data, err = ioutil.ReadAll(unpadded.Range(0, unpadded.Size()))
		if err != nil {
			t.Fatalf("unexpected error")
		}
		if !bytes.Equal(data, []byte(example.data)) {
			t.Fatalf("mismatch")
		}
	}
}
