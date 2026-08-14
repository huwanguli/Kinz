// Package kpool provides size-classed byte buffers backed by sync.Pool.
// It reduces allocation pressure on hot paths (per-connection read buffers,
// frame payloads). Only buffers created by Get should be returned with Put;
// buffers whose capacity does not match a size class are dropped silently.
package kpool

import "sync"

// size classes in bytes.
var classes = []int{4096, 16384, 65536}

var pools = []sync.Pool{
	{New: func() any { return make([]byte, classes[0]) }},
	{New: func() any { return make([]byte, classes[1]) }},
	{New: func() any { return make([]byte, classes[2]) }},
}

// putPoison is written to the first byte of a pooled buffer on Put, so a
// use-after-put that reads stale data becomes detectable.
const putPoison byte = 0xAA

// Get returns a zeroed-free buffer with capacity >= size, from the smallest
// matching size class. Requests larger than the largest class allocate
// directly (not pooled).
func Get(size int) []byte {
	for i, c := range classes {
		if size <= c {
			return pools[i].Get().([]byte)
		}
	}
	return make([]byte, size)
}

// Put returns buf to the pool when its capacity matches a size class exactly;
// otherwise it is dropped. Buffers must not be used after Put.
func Put(buf []byte) {
	for i, c := range classes {
		if cap(buf) == c {
			buf[0] = putPoison
			buf[len(buf)-1] = putPoison
			pools[i].Put(buf[:c])
			return
		}
	}
}
