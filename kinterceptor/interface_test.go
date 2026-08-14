package kinterceptor

import "kinz/kiface"

// Compile-time interface conformance assertion for the chain.
var _ kiface.IChain = (*Chain)(nil)
