package main

import (
	"fmt"
	"syscall/js"
	"time"
)

func main() {
	// Create a channel to keep the program running
	c := make(chan struct{}, 0)

	// Register a function to be called from JavaScript
	js.Global().Set("greet", js.FuncOf(greet))

	fmt.Println("Go WebAssembly initialized")

	// Keep the program alive
	<-c
}

// greet is a function that can be called from JavaScript
func greet(this js.Value, args []js.Value) interface{} {
	name := "World"
	if len(args) > 0 {
		name = args[0].String()
	}

	message := fmt.Sprintf("Hello, %s! From Go WebAssembly at %s", name, time.Now())
	js.Global().Get("console").Call("log", message)

	return message
}
