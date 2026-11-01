package main

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const clientURL = "https://github.com/ao-data/albiondata-client/releases/latest/download/update-windows-amd64.exe.gz"

var clientArgs = []string{"-minimize"}

const captureHint = "Reinstala Npcap sin marcar \"Restrict Npcap driver's access to Administrators only\", o abre este programa como administrador."

// Para que Windows muestre bien los mensajes aunque la consola no use UTF-8.
func setupConsole() {
	syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleOutputCP").Call(65001)
}

func prepareCmd(*exec.Cmd) {}

func waitEnter() {
	fmt.Println("Pulsa Enter para cerrar.")
	bufio.NewReader(os.Stdin).ReadString('\n')
}

func openURL(u string) {
	exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
}

func download(dest string) error {
	fmt.Println("Descargando el Albion Data Client oficial (solo la primera vez)...")
	resp, err := http.Get(clientURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub respondio %s", resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err = io.Copy(f, gz); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	f.Close()
	return os.Rename(tmp, dest)
}

// El cliente usa wpcap.dll, que Npcap solo deja en System32 con el modo compatible activado.
func npcapReady() bool {
	_, err := os.Stat(filepath.Join(os.Getenv("SystemRoot"), "System32", "wpcap.dll"))
	return err == nil
}

func findClient() (string, error) {
	base, err := os.UserCacheDir() // %LOCALAPPDATA%
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "BitacoraAvalon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "albiondata-client.exe")
	if _, err := os.Stat(path); err != nil {
		if err := download(path); err != nil {
			return "", fmt.Errorf("No pude descargar el Albion Data Client: %v\nRevisa tu conexion y vuelve a abrir este programa.", err)
		}
	}
	if !npcapReady() {
		fmt.Println()
		fmt.Println("Falta Npcap, el componente que permite leer el mapa del juego.")
		fmt.Println("Te abro su pagina: descarga el instalador y marca la casilla")
		fmt.Println(`"Install Npcap in WinPcap API-compatible Mode".`)
		fmt.Println("Este programa seguira solo cuando termine la instalacion.")
		openURL("https://npcap.com/#download")
		for !npcapReady() {
			time.Sleep(2 * time.Second)
		}
		fmt.Println("Npcap listo.")
	}
	return path, nil
}
