//go:build cgo

package main

/*
#include <stdint.h>

static inline int32_t add_c(int32_t a, int32_t b) {
    return a + b;
}
*/
// #cgo nocallback add_c
// #cgo noescape add_c
import "C"

// AddCgo is a small wrapper exposing the C implementation as a normal Go function.
func AddCgo(a, b int32) int32 {
	return int32(C.add_c(C.int32_t(a), C.int32_t(b)))
}
