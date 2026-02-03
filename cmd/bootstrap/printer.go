package bootstrap

import (
	"fmt"
	"os"
	"strings"
)

// PrintBannerFromFile Read file and print, auto-generate if file doesn't exist
func PrintBannerFromFile(filename string, defaultText string) error {
	// Ensure banner file exists, generate if it doesn't
	if err := EnsureBannerFile(filename, defaultText); err != nil {
		return fmt.Errorf("failed to ensure banner file: %w", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")

	colors := []string{
		"\x1b[38;5;165m",
		"\x1b[38;5;189m",
		"\x1b[38;5;207m",
		"\x1b[38;5;219m",
		"\x1b[38;5;225m",
		"\x1b[38;5;231m",
	}

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		color := colors[i%len(colors)]
		fmt.Println(color + line + "\x1b[0m")
	}
	return nil
}
