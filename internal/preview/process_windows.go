package preview

import "os/exec"

func manageProcess(cmd *exec.Cmd) {}
func stopProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
