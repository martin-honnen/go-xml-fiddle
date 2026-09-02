package main

import (
	"fmt"
	"syscall/js"
	"time"
)

func main() {
	// Create a channel to keep the program running
	c := make(chan struct{}, 0)

	// Register a function to be called from JavaScript. Keep the Func alive
	// for as long as the WebAssembly program is running.
	greetFunc := js.FuncOf(greet)
	defer greetFunc.Release()
	js.Global().Set("greet", greetFunc)

	fmt.Println("Go WebAssembly initialized")

	// Keep the program alive
	<-c
}

// greet is called through the JavaScript interop boundary. String values are
// converted to and from js.Value at that boundary.
func greet(this js.Value, args []js.Value) interface{} {
	name := ""
	if len(args) > 0 && args[0].Type() == js.TypeString {
		name = args[0].String()
	}
	if name == "" {
		name = "World"
	}
	message := fmt.Sprintf("Hello, %s! From Go WebAssembly at %s", name, time.Now())
	return message
}
