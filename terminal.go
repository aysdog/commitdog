package main

import "fmt"

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}
