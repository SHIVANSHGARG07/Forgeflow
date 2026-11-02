package executors

// dependencies
import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// make shell payload struct
type ShellPayload struct {
	Command string `json:"command"`
}

// RunShell function

func RunShell(payload []byte) error {

	var p ShellPayload

	//unmarhsal
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("invalid shell payload: %w", err)
	}

	if p.Command == "" {
		return fmt.Errorf("shell payload missing command")
	}

	cmd := exec.Command("sh", "-c", p.Command)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("shell command failed: %w, output: %s", err, output)

	}

	return nil
}
