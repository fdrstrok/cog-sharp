package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/fdrstrok/cog-sharp/internal/release"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 2 && args[0] == "version" {
		version, err := release.VersionFromTag(args[1])
		if err != nil {
			return err
		}
		fmt.Println(version)
		return nil
	}
	if len(args) != 0 && args[0] == "verify" {
		flags := flag.NewFlagSet("verify", flag.ContinueOnError)
		path := flags.String("package", "", "NuGet package file")
		version := flags.String("version", "", "Expected release version")
		commit := flags.String("commit", "", "Expected full source commit")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return fmt.Errorf("unexpected package verification arguments")
		}
		if err := release.VerifyPackage(*path, *version, *commit); err != nil {
			return err
		}
		fmt.Println("Package identity, source commit, author, license and SDK assembly verified.")
		return nil
	}
	return fmt.Errorf("usage: sdk-release version <sdk-v...> | verify --package file --version version --commit sha")
}
