// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jtolds/eestream"
	"github.com/jtolds/eestream/ranger"
	"github.com/vivint/infectious"
)

var (
	addr           = flag.String("addr", "localhost:8080", "address to serve from")
	pieceBlockSize = flag.Int("piece_block_size", 4*1024, "block size of pieces")
	key            = flag.String("key", "a key", "the secret key")
	rsk            = flag.Int("required", 20, "rs required")
	rsn            = flag.Int("total", 40, "rs total")
)

func main() {
	flag.Parse()
	if flag.Arg(0) == "" {
		fmt.Printf("usage: %s <targetdir>\n", os.Args[0])
		os.Exit(1)
	}
	err := Main()
	if err != nil {
		panic(err)
	}
}

func Main() error {
	encKey := sha256.Sum256([]byte(*key))
	fc, err := infectious.NewFEC(*rsk, *rsn)
	if err != nil {
		return err
	}
	es := eestream.NewRSScheme(fc, *pieceBlockSize)
	decrypter, err := eestream.NewSecretboxDecrypter(encKey[:],
		es.DecodedBlockSize())
	if err != nil {
		return err
	}
	pieces, err := ioutil.ReadDir(flag.Arg(0))
	if err != nil {
		return err
	}
	rrs := map[int]ranger.Ranger{}
	for _, piece := range pieces {
		piecenum, err := strconv.Atoi(strings.TrimSuffix(piece.Name(), ".piece"))
		if err != nil {
			return err
		}
		fh, err := os.Open(filepath.Join(flag.Arg(0), piece.Name()))
		if err != nil {
			return err
		}
		defer fh.Close()
		fs, err := fh.Stat()
		if err != nil {
			return err
		}
		rrs[piecenum] = ranger.ReaderAtRanger(fh, fs.Size())
	}
	rr, err := eestream.Decode(rrs, es)
	if err != nil {
		return err
	}
	rr, err = eestream.Transform(rr, decrypter)
	if err != nil {
		return err
	}
	rr, err = eestream.UnpadSlow(rr)
	if err != nil {
		return err
	}

	return http.ListenAndServe(*addr, http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			ranger.ServeContent(w, r, flag.Arg(0), time.Time{}, rr)
		}))
}
