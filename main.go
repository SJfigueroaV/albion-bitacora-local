// Programa local de la Bitácora de Avalon para Windows: abre las ventanas de búsqueda, navegación y
// farmeo sobre el juego. No toca el juego ni lee su tráfico: el mapa se detecta por pantalla en la web.
// El servidor local solo sirve para que la página le pase el grupo a esas ventanas.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

const addr = "127.0.0.1:47800"

var origins = map[string]bool{
	"https://bitacora-avalon-6d49.web.app":         true,
	"https://bitacora-avalon-6d49.firebaseapp.com": true,
	"http://127.0.0.1:5051":                        true,
}

// Grupo y jugador que abrió el navegador, para que las ventanas entren sin pedir nada.
type session struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Pid  string `json:"pid"`
}

var current struct {
	sync.Mutex
	session
}

func link(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	// Solo para las ventanas del propio programa: ninguna página web manda una petición sin origen.
	if r.URL.Path == "/session" && origin == "" {
		current.Lock()
		defer current.Unlock()
		json.NewEncoder(w).Encode(current.session)
		return
	}
	// Una web ajena no debe poder leer ni cambiar el grupo.
	if !origins[origin] || r.URL.Path != "/events" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	w.Header().Set("Vary", "Origin")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if q := r.URL.Query(); q.Get("g") != "" {
		current.Lock()
		current.session = session{q.Get("g"), q.Get("name"), q.Get("pid")}
		current.Unlock()
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprint(w, "data: {\"linked\":true}\n\n")
}

func main() {
	setupConsole()
	ventanas := flag.Bool("ventanas", false, "uso interno: abre las ventanas")
	flag.Parse()
	if *ventanas {
		runVentanas()
		return
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fatal("El programa ya esta abierto en otra ventana.")
	}
	fmt.Println("Bitacora de Avalon: ventanas sobre el juego")
	fmt.Println("Deja esta ventana abierta mientras juegas.")
	startVentanas()
	srv := &http.Server{Handler: http.HandlerFunc(link), ReadHeaderTimeout: 5 * time.Second}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fatal("Error del servidor local: " + err.Error())
	}
}

var execCommand = exec.Command

func fatal(msg string) {
	fmt.Println(msg)
	waitEnter()
	os.Exit(1)
}
