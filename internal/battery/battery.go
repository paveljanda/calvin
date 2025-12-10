package battery

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// GetBatteryPercentage returns the battery percentage from PiSugar 2
// Returns 100 if noBattery is true or if battery reading fails
func GetBatteryPercentage(ctx context.Context) (string, error) {
	output, err := exec.CommandContext(ctx, "pisugar-cli", "--get-battery-level").CombinedOutput()
	if err != nil {
		// If pisugar-cli is not available or fails, return 100%
		return "", fmt.Errorf("failed to exec pisugar-cli --get-battery-level: %w", err)
	}

	// Parse output - expected format: "battery_level: 85.5"
	outputStr := strings.TrimSpace(string(output))
	parts := strings.Split(outputStr, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("failed to parse output of pisugar-cli --get-battery-level: %w", err)
	}

	percentageStr := strings.TrimSpace(parts[1])
	percentage, err := strconv.ParseFloat(percentageStr, 64)
	if err != nil {
		return "", fmt.Errorf("failed to parse float output of pisugar-cli --get-battery-level: %w", err)
	}

	return fmt.Sprintf("%d%%", int(percentage)), nil
}
