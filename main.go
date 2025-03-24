package main

import (
	"awsctl/cmd/root"
	"fmt"
	"os"
)

func main() {
	if err := root.RootCmd.Execute(); err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
