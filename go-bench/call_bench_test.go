//go:build cgo

package main

import "testing"

// Sink is a global to prevent compiler optimizations removing the work.
var Sink int32

// ------------------------
// 1. Native Go call
// ------------------------

func addGo(a, b int32) int32 {
	return a + b
}

func BenchmarkNativeCall(b *testing.B) {
	var acc int32
	a, c := int32(1), int32(2)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		acc += addGo(a, c)
	}
	Sink = acc
}

// ------------------------
// 2. cgo call (via exported API)
// ------------------------

func BenchmarkCgoCall(b *testing.B) {
	var acc int32
	a, c := int32(1), int32(2)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		acc += AddCgo(a, c)
	}
	Sink = acc
}

// ------------------------
// 3. Call-through-channels
// ------------------------

type Request struct {
	A, B int32
	Resp chan int32
}

func worker(reqCh <-chan Request) {
	for req := range reqCh {
		req.Resp <- (req.A + req.B)
	}
}

func BenchmarkChannelCall(b *testing.B) {
	reqCh := make(chan Request)
	respCh := make(chan int32)

	go worker(reqCh)

	a, c := int32(1), int32(2)
	var acc int32

	// Warm up once
	reqCh <- Request{A: a, B: c, Resp: respCh}
	<-respCh

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reqCh <- Request{A: a, B: c, Resp: respCh}
		acc += <-respCh
	}

	b.StopTimer()
	Sink = acc
	close(reqCh)
}
