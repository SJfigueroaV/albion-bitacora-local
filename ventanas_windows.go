package main

// Ventanas de escritorio para Windows, las mismas /buscar y /ruta que usa bspwm en Linux:
// la búsqueda aparece con Alt+E y la navegación solo durante una ruta.
// Corren en un proceso aparte porque la biblioteca de WebView2 cierra el programa si algo falla,
// y eso no debe tumbar la detección del mapa.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
)

const web = "https://bitacora-avalon-6d49.web.app"

const (
	anchoBusqueda, altoBusqueda = 560, 96
	altoVista                   = 400
	anchoRuta, altoRuta         = 400, 60
	margenBusqueda, margenRuta  = 240, 12
	fuera                       = -3000 // apartada de la pantalla: conserva el foco del campo de texto
	opacidadRuta                = 148   // de 255, el 58 % que usa picom en Linux

	gwlStyle, gwlExStyle        = -16, -20
	wsPopup                     = 0x80000000
	wsVisible                   = 0x10000000
	wsExToolWindow              = 0x00000080
	wsExTopmost                 = 0x00000008
	wsExLayered                 = 0x00080000
	lwaAlpha                    = 2
	swpNoSize, swpNoActivate    = 0x0001, 0x0010
	swpFrameChanged             = 0x0020
	hwndTopmost                 = ^uintptr(0)
	smCxScreen                  = 0
	modAlt                      = 0x0001
	modNoRepeat                 = 0x4000
	vkE                         = 0x45
	wmHotkey, wmTimer           = 0x0312, 0x0113
	dwmwaWindowCornerPreference = 33
	dwmwcpRound                 = 2
	accentEnableBlurBehind      = 3
	wcaAccentPolicy             = 19
)

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	dwmapi                  = syscall.NewLazyDLL("dwmapi.dll")
	setWindowLongPtr        = user32.NewProc("SetWindowLongPtrW")
	getWindowLongPtr        = user32.NewProc("GetWindowLongPtrW")
	setWindowPos            = user32.NewProc("SetWindowPos")
	setLayeredAttributes    = user32.NewProc("SetLayeredWindowAttributes")
	getSystemMetrics        = user32.NewProc("GetSystemMetrics")
	getForegroundWindow     = user32.NewProc("GetForegroundWindow")
	setForegroundWindow     = user32.NewProc("SetForegroundWindow")
	registerHotKey          = user32.NewProc("RegisterHotKey")
	setTimer                = user32.NewProc("SetTimer")
	getMessage              = user32.NewProc("GetMessageW")
	setProcessDPIAware      = user32.NewProc("SetProcessDPIAware")
	getDpiForSystem         = user32.NewProc("GetDpiForSystem")
	setCompositionAttribute = user32.NewProc("SetWindowCompositionAttribute")
	dwmSetWindowAttribute   = dwmapi.NewProc("DwmSetWindowAttribute")
)

type ventana struct {
	hwnd    uintptr
	w       webview2.WebView
	ancho   int
	alto    int
	margen  int
	visible bool
}

func escala(n int) int {
	dpi, _, _ := getDpiForSystem.Call()
	if dpi == 0 {
		dpi = 96
	}
	return n * int(dpi) / 96
}

func (v *ventana) mover(visible bool) {
	x, y := 0, fuera
	if visible {
		pantalla, _, _ := getSystemMetrics.Call(smCxScreen)
		x, y = (int(pantalla)-escala(v.ancho))/2, escala(v.margen)
	}
	setWindowPos.Call(v.hwnd, hwndTopmost, uintptr(x), uintptr(y), uintptr(escala(v.ancho)), uintptr(escala(v.alto)), swpNoActivate)
	v.visible = visible
}

// Ventana sin marco, siempre encima y fuera de la barra de tareas.
func (v *ventana) preparar(vidrio bool) {
	ex, _, _ := getWindowLongPtr.Call(v.hwnd, uintptr(gwlExStyleU()))
	estiloEx := ex | wsExToolWindow | wsExTopmost
	if vidrio {
		estiloEx |= wsExLayered
	}
	setWindowLongPtr.Call(v.hwnd, gwlStyleU(), wsPopup|wsVisible)
	setWindowLongPtr.Call(v.hwnd, uintptr(gwlExStyleU()), estiloEx)
	redondeo := uint32(dwmwcpRound)
	dwmSetWindowAttribute.Call(v.hwnd, dwmwaWindowCornerPreference, uintptr(unsafe.Pointer(&redondeo)), 4)
	if vidrio {
		setLayeredAttributes.Call(v.hwnd, 0, opacidadRuta, lwaAlpha)
		desenfocar(v.hwnd)
	}
	setWindowPos.Call(v.hwnd, hwndTopmost, 0, uintptr(fuera&0xffffffff), uintptr(escala(v.ancho)), uintptr(escala(v.alto)), swpNoActivate|swpFrameChanged)
}

func gwlStyleU() uintptr   { n := gwlStyle; return uintptr(n) }
func gwlExStyleU() uintptr { n := gwlExStyle; return uintptr(n) }

// Difuminado de lo que queda detrás. Si esta versión de Windows no lo admite, solo se pierde el efecto.
func desenfocar(hwnd uintptr) {
	type accent struct{ estado, flags, color, animacion int32 }
	type datos struct {
		atributo int32
		_        int32
		valor    unsafe.Pointer
		tamano   uintptr
	}
	a := accent{estado: accentEnableBlurBehind}
	d := datos{atributo: wcaAccentPolicy, valor: unsafe.Pointer(&a), tamano: unsafe.Sizeof(a)}
	setCompositionAttribute.Call(hwnd, uintptr(unsafe.Pointer(&d)))
}

func nuevaVentana(v *ventana, ruta, datos string, vidrio bool, antes func(webview2.WebView), lista *sync.WaitGroup) {
	go func() {
		runtime.LockOSThread()
		w := webview2.NewWithOptions(webview2.WebViewOptions{
			DataPath:      datos,
			AutoFocus:     true,
			WindowOptions: webview2.WindowOptions{Title: "Bitácora de Avalon", Width: uint(v.ancho), Height: uint(v.alto)},
		})
		if w == nil {
			fmt.Println("No pude abrir WebView2.")
			os.Exit(2)
		}
		v.w, v.hwnd = w, uintptr(w.Window())
		v.preparar(vidrio)
		if antes != nil {
			antes(w)
		}
		w.Navigate(web + ruta)
		lista.Done()
		w.Run()
		os.Exit(0)
	}()
}

// Lee del puente el grupo que abrió el navegador. Si el puente se cierra, las ventanas también.
func seguirSesion(busqueda, ruta *ventana) {
	var actual string
	fallos := 0
	cliente := http.Client{Timeout: 2 * time.Second}
	for ; ; time.Sleep(1500 * time.Millisecond) {
		r, err := cliente.Get("http://" + addr + "/session")
		if err != nil {
			if fallos++; fallos >= 4 {
				os.Exit(0)
			}
			continue
		}
		fallos = 0
		var s session
		json.NewDecoder(r.Body).Decode(&s)
		r.Body.Close()
		if s.Code == "" || s.Code+s.Pid == actual {
			continue
		}
		actual = s.Code + s.Pid
		q := url.Values{"g": {s.Code}, "name": {s.Name}, "pid": {s.Pid}}.Encode()
		busqueda.w.Dispatch(func() { busqueda.w.Navigate(web + "/buscar?" + q) })
		ruta.w.Dispatch(func() { ruta.w.Navigate(web + "/ruta?" + q) })
	}
}

// El atajo y el clic fuera se atienden en un hilo propio: no dependen de ninguna ventana.
func atenderAtajo(busqueda *ventana) {
	runtime.LockOSThread()
	if ok, _, _ := registerHotKey.Call(0, 1, modAlt|modNoRepeat, vkE); ok == 0 {
		fmt.Println("Otro programa ya usa Alt+E.")
	}
	setTimer.Call(0, 0, 150, 0)
	var msg struct {
		hwnd    uintptr
		message uint32
		wparam  uintptr
		lparam  uintptr
		time    uint32
		x, y    int32
		_       uint32
	}
	var anterior uintptr
	var abierta time.Time
	for {
		if r, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0); int32(r) <= 0 {
			return
		}
		switch msg.message {
		case wmHotkey:
			if busqueda.visible {
				busqueda.mover(false)
				setForegroundWindow.Call(anterior)
			} else {
				anterior, _, _ = getForegroundWindow.Call()
				busqueda.mover(true)
				setForegroundWindow.Call(busqueda.hwnd)
				abierta = time.Now()
			}
		case wmTimer:
			if frente, _, _ := getForegroundWindow.Call(); busqueda.visible && frente != busqueda.hwnd && frente != 0 && time.Since(abierta) > 400*time.Millisecond {
				busqueda.mover(false) // clic fuera: el foco ya está donde el jugador lo puso
			}
		}
	}
}

func runVentanas() {
	setProcessDPIAware.Call()
	base, _ := os.UserCacheDir()
	datos := filepath.Join(base, "BitacoraAvalon", "ventanas")
	busqueda := &ventana{ancho: anchoBusqueda, alto: altoBusqueda, margen: margenBusqueda}
	ruta := &ventana{ancho: anchoRuta, alto: altoRuta, margen: margenRuta}
	var listas sync.WaitGroup
	listas.Add(1)
	nuevaVentana(busqueda, "/buscar", datos, false, func(w webview2.WebView) {
		w.Bind("bitacoraVista", func(activa bool) { // la imagen del mapa necesita más alto
			busqueda.alto = altoBusqueda
			if activa {
				busqueda.alto = altoVista
			}
			busqueda.mover(busqueda.visible)
		})
	}, &listas)
	listas.Wait() // WebView2 no admite crear su entorno dos veces a la vez sobre la misma carpeta
	listas.Add(1)
	nuevaVentana(ruta, "/ruta", datos, true, func(w webview2.WebView) {
		w.Bind("bitacoraRuta", func(activa bool) { ruta.mover(activa) })
	}, &listas)
	listas.Wait()
	go seguirSesion(busqueda, ruta)
	atenderAtajo(busqueda)
}

// Quita las descargas anteriores de este mismo programa, como "bitacora-local (1).exe".
// Solo en su carpeta, solo con ese nombre, solo si son más antiguas y solo si de verdad son este programa.
func limpiarAnteriores(exe string) {
	propio, err := os.Stat(exe)
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	archivos, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, a := range archivos {
		nombre := strings.ToLower(a.Name())
		if a.IsDir() || !strings.HasPrefix(nombre, "bitacora-local") || !strings.HasSuffix(nombre, ".exe") {
			continue
		}
		ruta := filepath.Join(dir, a.Name())
		info, err := os.Stat(ruta)
		if err != nil || os.SameFile(info, propio) || !info.ModTime().Before(propio.ModTime()) {
			continue
		}
		contenido, err := os.ReadFile(ruta)
		if err != nil || !bytes.Contains(contenido, []byte(web)) {
			continue
		}
		if os.Remove(ruta) == nil { // si esa copia sigue abierta, Windows lo impide y queda para otra vez
			fmt.Println("[bitacora] Quité la versión anterior:", a.Name())
		}
	}
}

// Lanza las ventanas sin arriesgar la detección: si fallan, este proceso sigue igual.
func startVentanas() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	limpiarAnteriores(exe)
	registro, _ := os.Create(filepath.Join(filepath.Dir(exe), "bitacora-ventanas.log"))
	cmd := execCommand(exe, "--ventanas")
	cmd.Stdout, cmd.Stderr = registro, registro
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000} // sin consola propia
	go func() {
		inicio := time.Now()
		if err := cmd.Run(); err != nil && time.Since(inicio) < time.Minute {
			fmt.Println("[bitacora] No pude abrir las ventanas de búsqueda y ruta. La detección del mapa sigue funcionando.")
			fmt.Println("[bitacora] Instala Microsoft Edge WebView2 Runtime si quieres usarlas.")
		}
	}()
	fmt.Println("Alt+E: abrir o cerrar la búsqueda de mapas.")
}
