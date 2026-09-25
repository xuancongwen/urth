package world

import (
	"os"
	"path/filepath"
)

// The sign-in banner is data/banner.txt, with color tokens, so it can be
// changed without a rebuild. This is the fallback when the file is
// missing.
const defaultBanner = "{Y}\n" +
	"    __  __ ____ _____ _   _\n" +
	"   / / / // __ /_  _// / / /\n" +
	"  / / / // /_/ / / / / /_/ /\n" +
	" / /_/ // _, _/ / / / __  /\n" +
	" \\____//_/ |_| /_/ /_/ /_/\n" +
	"{x}\n"

// loadBanner reads the banner file under the data directory.
func loadBanner(dataDir string) string {
	raw, err := os.ReadFile(filepath.Join(dataDir, "banner.txt"))
	if err != nil || len(raw) == 0 {
		return defaultBanner
	}
	s := string(raw)
	if s[len(s)-1] != '\n' {
		s += "\n"
	}
	return s
}
