package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/fdrstrok/cog-sharp/internal/sdkgen"
)

func main() {
	root := flag.String("root", ".", "cog-sharp repository directory")
	foundation := flag.String("foundation-sdk-dir", "", "Existing checkout at the pinned Foundation SDK revision")
	registry := flag.String("kind-registry-dir", "", "Existing checkout at the pinned kind-registry revision")
	flag.Parse()
	if err := sdkgen.Generate(context.Background(), *root, *foundation, *registry); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
