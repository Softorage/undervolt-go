//go:build !gui

/*
 4. Build CLI-only version

go build -o undervolt-go .
go build -ldflags="-X main.version=$(git describe --tags)" -o undervolt-go .
✅ This will exclude GUI code and dependencies like fyne.
*/

package main

import "fmt"

// Used in main.go by rootCmd
const rootCmdUseString := "undervolt-go"

func runGUI() {
    fmt.Println("Run 'undervolt-go --help' for information about CLI flags. To get the GUI, get the GUI binary from https://softorage.github.io/undervolt-go/")
}