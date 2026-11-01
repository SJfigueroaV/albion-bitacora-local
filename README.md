# Bitácora de Avalon: programa local

Programa opcional para [Bitácora de Avalon](https://bitacora-avalon-6d49.web.app), una web para registrar en grupo el recorrido por los Caminos de Avalon de Albion Online. Si juegas con Albion instalado en tu PC, detecta el mapa en el que estás y lo pasa a la web.

Proyecto de la comunidad, sin relación con Sandbox Interactive ni con Albion Online.

## Cómo usarlo en Windows

1. Descarga `bitacora-local.exe` desde [Releases](https://github.com/SJfigueroaV/albion-bitacora-local/releases/latest).
2. Ábrelo. Si Windows lo bloquea, pulsa **Más información** y **Ejecutar de todas formas**: el programa no tiene firma digital.
3. La primera vez descarga el [Albion Data Client](https://github.com/ao-data/albiondata-client) oficial. Si falta [Npcap](https://npcap.com/#download), abre su página: instálalo con **WinPcap API-compatible Mode** marcado.
4. Deja la ventana abierta y, en la web, pulsa **Juego instalado: detectar por red**.

## Qué hace

- Arranca el Albion Data Client con la subida de datos desactivada (`-d`).
- Lee el código de mapa que el cliente observa en el tráfico del juego y lo traduce con `clusters.json`.
- Entrega el nombre del mapa solo a la web de la Bitácora, por `127.0.0.1:47800`.

No modifica el juego ni su tráfico, y no envía nada desde tu PC. Aun así, escucha el tráfico de Albion: decide si quieres usarlo. Si juegas por GeForce Now no sirve, porque el juego no corre en tu PC; usa la detección por pantalla de la web.

## Compilar

```
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags '-s -w' -o bitacora-local.exe .
```

SHA-256 de `bitacora-local.exe` v1.0.0: `447603964a80beb845e15337d570d629189ccf4ee676e4aeaac835ac8267c56f`

## Ventanas sobre el juego (Windows)

Desde la versión 1.1 el programa abre dos ventanas siempre encima del juego:

- **Búsqueda:** `Ctrl+Shift+E` la muestra y la oculta. También se oculta al hacer clic fuera.
- **Navegación:** indica el siguiente mapa, y solo se ve mientras sigues una ruta.

Toman el grupo de la página: entra a un grupo y pulsa «Juego instalado: detectar por red».
Necesitan Microsoft Edge WebView2 Runtime, que ya trae Windows 11. Si falta, la detección del mapa sigue funcionando igual.
Albion debe estar en ventana sin bordes: en pantalla completa exclusiva ninguna ventana puede ir encima.
