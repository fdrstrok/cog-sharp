package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/fdrstrok/cog-sharp/internal/sdkgen"
)

func main() {
	root := flag.String("root", ".", "cog-sharp repository directory")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: sdk-validate [-root directory] dashboard.json")
		os.Exit(2)
	}
	if err := sdkgen.ValidateDashboard(*root, flag.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Dashboard JSON matches the pinned upstream Dashboardv2 schema.")
}
