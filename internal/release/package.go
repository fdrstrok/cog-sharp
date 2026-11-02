package release

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

const PackageID = "CogSharp.Grafana.Foundation"

var releaseTag = regexp.MustCompile(`^sdk-v((0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?)$`)

func VersionFromTag(tag string) (string, error) {
	match := releaseTag.FindStringSubmatch(tag)
	if match == nil {
		return "", fmt.Errorf("release ref must be an sdk-v<major>.<minor>.<patch>[-prerelease] tag")
	}
	version := match[1]
	if _, prerelease, ok := strings.Cut(version, "-"); ok {
		for _, identifier := range strings.Split(prerelease, ".") {
			if len(identifier) > 1 && identifier[0] == '0' && strings.Trim(identifier, "0123456789") == "" {
				return "", fmt.Errorf("numeric prerelease identifiers cannot have leading zeroes")
			}
		}
	}
	return version, nil
}

type packageDocument struct {
	Metadata struct {
		ID         string `xml:"id"`
		Version    string `xml:"version"`
		Authors    string `xml:"authors"`
		Repository struct {
			Type   string `xml:"type,attr"`
			URL    string `xml:"url,attr"`
			Commit string `xml:"commit,attr"`
		} `xml:"repository"`
	} `xml:"metadata"`
}

func VerifyPackage(path, version, commit string) error {
	if _, err := VersionFromTag("sdk-v" + version); err != nil {
		return err
	}
	if matched, _ := regexp.MatchString(`^[0-9a-f]{40}$`, commit); !matched {
		return fmt.Errorf("expected a full source commit SHA")
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer archive.Close()

	required := map[string]bool{
		"LICENSE": false, "NOTICE": false, "README.md": false,
		"lib/net10.0/Grafana.Foundation.dll": false,
	}
	var document packageDocument
	manifests := 0
	for _, file := range archive.File {
		if _, ok := required[file.Name]; ok && file.UncompressedSize64 > 0 {
			required[file.Name] = true
		}
		if strings.HasSuffix(file.Name, ".nuspec") {
			manifests++
			reader, err := file.Open()
			if err != nil {
				return err
			}
			err = xml.NewDecoder(reader).Decode(&document)
			reader.Close()
			if err != nil {
				return err
			}
		}
	}
	if manifests != 1 {
		return fmt.Errorf("expected exactly one package manifest, got %d", manifests)
	}
	for path, found := range required {
		if !found {
			return fmt.Errorf("package is missing %s", path)
		}
	}
	metadata := document.Metadata
	if metadata.ID != PackageID || metadata.Version != version {
		return fmt.Errorf("package identity mismatch: got %s %s, expected %s %s",
			metadata.ID, metadata.Version, PackageID, version)
	}
	if metadata.Authors != "fdrstrok" {
		return fmt.Errorf("unexpected package authors %q", metadata.Authors)
	}
	if metadata.Repository.Type != "git" ||
		metadata.Repository.URL != "https://github.com/fdrstrok/cog-sharp" ||
		metadata.Repository.Commit != commit {
		return fmt.Errorf("package repository metadata does not match the release source")
	}
	return nil
}
