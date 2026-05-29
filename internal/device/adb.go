package device

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

var cscListPaths = []string{
	"/prism/etc/sales_code_list.dat",
	"/product/omc/sales_code_list.dat",
	"/optics/configs/carriers/sales_code_list.dat",
	"/system/omc/sales_code_list.dat",
}

func AvailableCSCs(ctx context.Context) ([]string, error) {
	if _, err := exec.LookPath("adb"); err != nil {
		return nil, fmt.Errorf("adb not found in PATH (needed to read the device CSC list)")
	}
	var lastErr error
	for _, path := range cscListPaths {
		out, err := exec.CommandContext(ctx, "adb", "shell", "cat", path).Output()
		if err != nil {
			lastErr = err
			continue
		}
		if codes := parseCSCList(string(out)); len(codes) > 0 {
			return codes, nil
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("read CSC list via adb: %w", lastErr)
	}
	return nil, fmt.Errorf("no CSC list found on device")
}

func parseCSCList(raw string) []string {
	seen := map[string]bool{}
	var codes []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "single/")
		line = strings.TrimPrefix(line, "multi/")
		if ValidCSC(line) && !seen[line] {
			seen[line] = true
			codes = append(codes, line)
		}
	}
	return codes
}
