//go:build windows

package codefori

import "os"

// Windows has no POSIX uid/mode model. State resolves below the current user's
// application-data directory; symlinks are rejected, but this reader cannot
// independently validate Windows DACL ownership.
func privateTokenEntry(info os.FileInfo) bool { return info.Mode().IsRegular() || info.IsDir() }
