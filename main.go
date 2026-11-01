// Puente local de la Bitácora de Avalon para quien juega Albion instalado en su PC.
//
// Arranca el Albion Data Client (solo escucha el tráfico, no lo toca ni sube datos), traduce el
// código de mapa a su nombre y lo entrega a la web por http://127.0.0.1. Nada sale de este PC.
package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const addr = "127.0.0.1:47800"

//go:embed clusters.json
var clustersRaw []byte

var (
	origins = map[string]bool{
		"https://bitacora-avalon-6d49.web.app":         true,
		"https://bitacora-avalon-6d49.firebaseapp.com": true,
		"http://127.0.0.1:5051":                        true,
	}
	ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	// Los caminos llegan como TNL-xxx y el cliente los descarta por raros: esa línea solo sale con -debug.
	location = regexp.MustCompile(`Updating player location to\s+(\S+?)\.?\s*$|Ignoring implausible join location value:\s+\\?"([^"\\]+)`)
	road     = regexp.MustCompile(`TNL-\d+`)
	clusters map[string]string
)

type state struct {
	Map    *string `json:"map"`
	Raw    *string `json:"raw"`
	Client string  `json:"client"`
}

// Grupo y jugador que abrió el navegador, para que las ventanas de Windows entren sin pedir nada.
type session struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Pid  string `json:"pid"`
}

var current struct {
	sync.Mutex
	session
}

var hub = struct {
	sync.Mutex
	cur       state
	lastRaw   string
	warned    bool
	listeners map[chan string]bool
}{cur: state{Client: "starting"}, listeners: map[chan string]bool{}}

func publish(change func(*state)) {
	hub.Lock()
	defer hub.Unlock()
	change(&hub.cur)
	b, _ := json.Marshal(hub.cur)
	for q := range hub.listeners {
		select {
		case q <- string(b):
		default:
		}
	}
}

// El mismo aviso puede llegar por la salida del cliente y por su registro, de ahí la comparación.
func onLine(line string) {
	explainError(line)
	m := location.FindStringSubmatch(ansi.ReplaceAllString(strings.TrimSpace(line), ""))
	if m == nil {
		return
	}
	raw := m[1] + m[2]
	hub.Lock()
	repeated := raw == hub.lastRaw
	hub.lastRaw = raw
	hub.Unlock()
	if repeated {
		return
	}
	var name *string
	if n, ok := clusters[road.FindString(raw)]; ok {
		name = &n
	}
	if name != nil {
		fmt.Printf("[bitacora] mapa %s (%s)\n", *name, raw)
	} else {
		fmt.Printf("[bitacora] fuera de los caminos (%s)\n", raw)
	}
	publish(func(s *state) { s.Map, s.Raw = name, &raw })
}

// Traduce los fallos típicos de captura, porque el cliente se cierra sin que nadie vea su registro.
func explainError(line string) {
	if !strings.Contains(line, "Could not open") {
		return
	}
	hub.Lock()
	seen := hub.warned
	hub.warned = true
	hub.Unlock()
	if !seen {
		fmt.Println("[bitacora] el cliente no tiene permiso para leer la red.")
		fmt.Println(captureHint)
	}
}

func tail(path string, off int64, stop <-chan struct{}) {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	partial := ""
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		if fi.Size() < off { // el cliente rotó el registro al arrancar
			off, partial = 0, ""
		}
		if fi.Size() == off {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		f.Seek(off, io.SeekStart)
		b, _ := io.ReadAll(f)
		f.Close()
		off += int64(len(b))
		lines := strings.Split(partial+string(b), "\n")
		partial = lines[len(lines)-1]
		for _, l := range lines[:len(lines)-1] {
			onLine(l)
		}
	}
}

func runClient(path string, once, debug bool) {
	dir := filepath.Dir(path)
	logPath := filepath.Join(dir, "albiondata-client.log")
	args := append([]string{"-d", "-debug"}, clientArgs...)
	var trace *os.File
	if debug {
		trace, _ = os.OpenFile(filepath.Join(dir, "bitacora-captura.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	}
	for {
		var off int64
		if fi, err := os.Stat(logPath); err == nil {
			off = fi.Size()
		}
		stop := make(chan struct{})
		go tail(logPath, off, stop)
		pr, pw := io.Pipe()
		cmd := exec.Command(path, args...)
		cmd.Dir, cmd.Stdout, cmd.Stderr = dir, pw, pw
		prepareCmd(cmd)
		done := make(chan struct{})
		go func() {
			defer close(done)
			r := bufio.NewReaderSize(pr, 1<<20)
			for {
				l, err := r.ReadString('\n')
				if trace != nil {
					trace.WriteString(l)
				}
				onLine(l)
				if err != nil {
					return
				}
			}
		}()
		if err := cmd.Start(); err != nil {
			fmt.Println("[bitacora] no pude abrir el cliente de datos:", err)
		} else {
			publish(func(s *state) { s.Client = "running" })
			fmt.Println("[bitacora] cliente de datos en marcha")
			err = cmd.Wait()
			fmt.Println("[bitacora] el cliente se cerro:", err)
		}
		pw.Close()
		<-done
		close(stop)
		publish(func(s *state) { s.Client = "stopped" })
		if once {
			return
		}
		fmt.Println("[bitacora] reinicio en 5 s")
		time.Sleep(5 * time.Second)
	}
}

func cors(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	w.Header().Set("Vary", "Origin")
}

func events(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	// Solo para las ventanas del propio programa: ninguna página web manda una petición sin origen.
	if r.URL.Path == "/session" && origin == "" {
		current.Lock()
		defer current.Unlock()
		json.NewEncoder(w).Encode(current.session)
		return
	}
	// Una web ajena no debe poder leer en qué mapa estás.
	if !origins[origin] || r.URL.Path != "/events" {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	cors(w, origin)
	if q := r.URL.Query(); q.Get("g") != "" {
		current.Lock()
		current.session = session{q.Get("g"), q.Get("name"), q.Get("pid")}
		current.Unlock()
	}
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	fl := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	q := make(chan string, 16)
	hub.Lock()
	hub.listeners[q] = true
	first, _ := json.Marshal(hub.cur)
	hub.Unlock()
	defer func() { hub.Lock(); delete(hub.listeners, q); hub.Unlock() }()
	fmt.Fprintf(w, "data: %s\n\n", first)
	fl.Flush()
	beat := time.NewTicker(15 * time.Second)
	defer beat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case m := <-q:
			fmt.Fprintf(w, "data: %s\n\n", m)
		case <-beat.C:
			fmt.Fprint(w, ": sigo aqui\n\n")
		}
		fl.Flush()
	}
}

func main() {
	setupConsole()
	client := flag.String("client", "", "ruta del Albion Data Client")
	once := flag.Bool("once", false, "no reiniciar el cliente si se cierra")
	debug := flag.Bool("debug", false, "guarda la salida del cliente en bitacora-captura.log")
	ventanas := flag.Bool("ventanas", false, "uso interno: abre las ventanas de búsqueda y ruta")
	flag.Parse()
	if *ventanas {
		runVentanas()
		return
	}
	if err := json.Unmarshal(clustersRaw, &clusters); err != nil {
		fatal("lista de mapas danada: " + err.Error())
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fatal("El programa ya esta abierto en otra ventana.")
	}
	fmt.Println("Bitacora de Avalon: programa local")
	fmt.Println("Deja esta ventana abierta mientras juegas.")
	go func() {
		srv := &http.Server{Handler: http.HandlerFunc(events), ReadHeaderTimeout: 5 * time.Second}
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Println("[bitacora] error del servidor local:", err)
		}
	}()
	startVentanas()
	path := *client
	if path == "" {
		if path, err = findClient(); err != nil {
			fatal(err.Error())
		}
	}
	runClient(path, *once, *debug)
}

var execCommand = exec.Command

func fatal(msg string) {
	fmt.Println(msg)
	waitEnter()
	os.Exit(1)
}
