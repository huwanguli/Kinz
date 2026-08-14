package kinterceptor

import "kinz/kiface"

// Compile-time interface conformance assertions for the interceptor package.
var (
	_ kiface.IDecoder = (*FrameDecoder)(nil)
	_ kiface.IChain   = (*Chain)(nil)
)
