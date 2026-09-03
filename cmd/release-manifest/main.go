package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"bac-nexus/internal/release"
)

var (
	errInvalidReleaseIdentity     = errors.New("release identity is invalid")
	errReleaseBinaryUnavailable   = errors.New("release binary is unavailable")
	errReleaseManifestUnavailable = errors.New("release manifest is unavailable")
	errReleaseManifestInvalid     = errors.New("release manifest is invalid")
	semverVersion                 = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-((0|[1-9][0-9]*)|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(\.((0|[1-9][0-9]*)|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*)?$`)
	approvedReleaseTargets        = map[string]map[string]bool{"linux": {"amd64": true, "arm64": true}, "darwin": {"amd64": true, "arm64": true}, "windows": {"amd64": true, "arm64": true}}
	readFile                      = os.ReadFile
	writeFile                     = os.WriteFile
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "release-manifest:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("release-manifest", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	binaryPath := flags.String("binary", "", "built Nexus binary")
	manifestPath := flags.String("manifest", "", "manifest sidecar path")
	version := flags.String("version", "", "release version")
	revision := flags.String("revision", "", "VCS revision")
	goos := flags.String("goos", "", "target operating system")
	goarch := flags.String("goarch", "", "target architecture")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *binaryPath == "" || *manifestPath == "" || *version == "" || *goos == "" || *goarch == "" {
		return errors.New("binary, manifest, version, revision, goos, and goarch are required")
	}
	identity := release.Identity{Version: *version, Revision: *revision}
	if !approvedIdentity(identity, *goos, *goarch) {
		return errInvalidReleaseIdentity
	}
	if filepath.ToSlash(*binaryPath) != approvedBinaryPath(identity, *goos, *goarch) || filepath.ToSlash(*manifestPath) != release.ManifestFilename(identity, *goos, *goarch) {
		return errInvalidReleaseIdentity
	}

	binary, err := readFile(*binaryPath)
	if err != nil {
		return errReleaseBinaryUnavailable
	}
	manifest := release.NewManifest(identity, *goos, *goarch, binary)
	data, err := manifest.JSON()
	if err != nil {
		return errReleaseManifestInvalid
	}
	if err := writeFile(*manifestPath, append(data, '\n'), 0o644); err != nil {
		return errReleaseManifestUnavailable
	}

	data, err = readFile(*manifestPath)
	if err != nil {
		return errReleaseManifestUnavailable
	}
	var parsed release.Manifest
	if err := json.Unmarshal(data, &parsed); err != nil {
		return errReleaseManifestInvalid
	}
	if err := release.VerifyManifest(parsed, *binaryPath, binary, identity); err != nil {
		return errReleaseManifestInvalid
	}
	return nil
}

func approvedIdentity(identity release.Identity, goos, goarch string) bool {
	return semverVersion.MatchString(identity.Version) && approvedRevision(identity.Revision) && approvedReleaseTargets[goos][goarch]
}

func approvedRevision(revision string) bool {
	if revision == "" || strings.Contains(revision, "..") {
		return false
	}
	for _, character := range revision {
		if unicode.IsControl(character) || unicode.IsSpace(character) || character == '/' || character == '\\' {
			return false
		}
	}
	return true
}

func approvedBinaryPath(identity release.Identity, goos, goarch string) string {
	binary := "nexus"
	if goos == "windows" {
		binary += ".exe"
	}
	return filepath.ToSlash(filepath.Join("build", "v1-mcp-foundation", identity.Version, goos+"-"+goarch, binary))
}
