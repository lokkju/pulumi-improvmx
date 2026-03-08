package main

import (
	"context"
	"fmt"
	"os"

	improvmx "github.com/lokkju/pulumi-improvmx/provider"
)

func main() {
	err := improvmx.Provider().Run(context.Background(), improvmx.Name, improvmx.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(1)
	}
}
