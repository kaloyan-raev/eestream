# eestream

erasure encoded streaming. some initial go code for how to stream data through
encryption and erasure encoding on demand.

https://godoc.org/github.com/jtolds/eestream

TODO: all erasure encoding and decoding steps currently expect to operate in
lockstep, which is bad when some of the sources and sinks are going much slower
or fail. loosen this restriction.

### License

All files are copyright Storj Labs, Inc., 2018 unless otherwise noted. See the
LICENSE file for more details.

One specific exception is ranger/content.go, which is code that came from the
Go standard library and is copyright the Go authors. Please see the Go project's
LICENSE file.


