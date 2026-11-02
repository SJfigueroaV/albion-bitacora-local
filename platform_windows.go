package main

import (
	"bufio"
	"fmt"
	"os"
	"syscall"
)

// Para que Windows muestre bien los mensajes aunque la consola no use UTF-8.
func setupConsole() {
	syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleOutputCP").Call(65001)
}

func waitEnter() {
	fmt.Println("Pulsa Enter para cerrar.")
	bufio.NewReader(os.Stdin).ReadString('\n')
}
