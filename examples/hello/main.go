package main

import (
	"fmt"
)

// Simple hello example to keep the repo tidy.
func main() {
	s := "gopher"
	fmt.Printf("Hello and welcome, %s!\n", s)
	for i := 1; i <= 5; i++ {
		fmt.Println("i =", 100/i)
	}
}
