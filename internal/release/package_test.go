package release

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionFromTag(t *testing.T) {
	for _, version := range []string{"0.1.0-alpha.1", "1.2.3", "2.0.0-rc.2"} {
		actual, err := VersionFromTag("sdk-v" + version)
		if err != nil || actual != version {
			t.Fatalf("valid version %q: got %q, %v", version, actual, err)
		}
	}
	for _, tag := range []string{
		"main", "v0.1.0", "sdk-v01.0.0", "sdk-v1.0.0-alpha.01",
		"sdk-v1.0.0+build", "sdk-v1.0.0;echo bad", "sdk-v1.0.0-alpha..1",
	} {
		if _, err := VersionFromTag(tag); err == nil {
			t.Fatalf("accepted unsafe or ambiguous release tag %q", tag)
		}
	}
}

func TestVerifyPackageRejectsWrongReleaseMetadata(t *testing.T) {
	commit := strings.Repeat("a", 40)
	tests := []struct {
		name, id, version, authors, source string
		valid                              bool
	}{
		{"release", PackageID, "0.1.0-alpha.1", "fdrstrok", commit, true},
		{"wrong package", "Another.Package", "0.1.0-alpha.1", "fdrstrok", commit, false},
		{"wrong version", PackageID, "0.1.0-alpha.2", "fdrstrok", commit, false},
		{"default assembly author", PackageID, "0.1.0-alpha.1", "Grafana.Foundation", commit, false},
		{"stale source", PackageID, "0.1.0-alpha.1", "fdrstrok", strings.Repeat("b", 40), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := packageFixture(t, test.id, test.version, test.authors, test.source, true)
			err := VerifyPackage(path, "0.1.0-alpha.1", commit)
			if (err == nil) != test.valid {
				t.Fatalf("package verification returned %v", err)
			}
		})
	}
}

func TestVerifyPackageRequiresLicenseAndAssembly(t *testing.T) {
	commit := strings.Repeat("a", 40)
	path := packageFixture(t, PackageID, "0.1.0-alpha.1", "fdrstrok", commit, false)
	if err := VerifyPackage(path, "0.1.0-alpha.1", commit); err == nil {
		t.Fatal("package without distribution files passed release verification")
	}
}

func packageFixture(t *testing.T, id, version, authors, commit string, distributionFiles bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sdk.nupkg")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	manifest, err := archive.Create("sdk.nuspec")
	if err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintf(manifest,
		`<package><metadata><id>%s</id><version>%s</version><authors>%s</authors><repository type="git" url="https://github.com/fdrstrok/cog-sharp" commit="%s"/></metadata></package>`,
		id, version, authors, commit)
	if err != nil {
		t.Fatal(err)
	}
	if distributionFiles {
		for _, name := range []string{"LICENSE", "NOTICE", "README.md", "lib/net10.0/Grafana.Foundation.dll"} {
			entry, err := archive.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write([]byte("fixture")); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
