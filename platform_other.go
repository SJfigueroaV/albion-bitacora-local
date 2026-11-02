//go:build !windows

package main

// Las ventanas solo existen en Windows: en Linux las abre bspwm. Esto queda para probar el servidor.
func setupConsole() {}

func waitEnter() {}

func startVentanas() {}

func runVentanas() {}
