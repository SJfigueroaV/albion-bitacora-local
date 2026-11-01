//go:build !windows

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var clientArgs []string

const captureHint = "Revisa el permiso de captura con getcap sobre albiondata-client."

func setupConsole() {}

func prepareCmd(*exec.Cmd) {}

func waitEnter() {}

func startVentanas() {}

func runVentanas() {}

func findClient() (string, error) {
	path, err := exec.LookPath("albiondata-client")
	if err != nil {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, "albion-flipper", "albiondata-client")
	}
	if _, err := os.Stat(path); err != nil {
		return "", errors.New("No encuentro albiondata-client. Indica su ruta con --client.")
	}
	if out, _ := exec.Command("getcap", path).Output(); !strings.Contains(string(out), "cap_net_raw") {
		fmt.Printf("El cliente no tiene permiso de captura. Ejecuta:\n  sudo setcap cap_net_raw,cap_net_admin=eip %s\n", path)
	}
	return path, nil
}
