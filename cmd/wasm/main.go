//go:build js && wasm
// +build js,wasm

package main

import (
	"fmt"
	"github.com/DjarinDin/TuringSNN/internal/bridge"
	"runtime/debug"
	"syscall/js"
)

func main() {
	// Panic recovery to display error in browser
	defer func() {
		if r := recover(); r != nil {
			errStr := fmt.Sprintf("PANIC: %v\nStack: %s", r, debug.Stack())
			js.Global().Get("console").Call("error", errStr)

			doc := js.Global().Get("document")
			if !doc.IsNull() {
				div := doc.Call("getElementById", "error-display")
				if !div.IsNull() {
					div.Get("style").Set("display", "block")
					div.Set("innerText", errStr)
				}
			}
		}
	}()

	// Keep WASM alive
	done := make(chan struct{})

	// Export TuringCortex API to JavaScript. This is the open-source core's
	// own reference build: no external data sources, just the synthetic Sim.
	// A deployment wiring in a live feed passes WorldSourceFactory values here.
	js.Global().Set("TuringCortex", bridge.NewWASMBridge())

	// Block forever
	<-done
}
