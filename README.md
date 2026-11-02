# Bitácora de Avalon: ventanas sobre el juego

Programa opcional para Windows de la [Bitácora de Avalon](https://bitacora-avalon-6d49.web.app).
Abre ventanas siempre encima de Albion Online.

Proyecto de la comunidad, sin relación con Sandbox Interactive ni con Albion Online.

Estas son las ventanas:

- **Búsqueda:** `Alt+E` la muestra y la oculta. También se oculta al hacer clic fuera. Al agregar un mapa muestra su imagen, sus zonas de farmeo y sus aspectos. Con la detección por pantalla, también se abre sola a la derecha al pasar el mouse sobre un portal, sin quitarle el teclado al juego.
- **Navegación:** indica el siguiente mapa, y solo se ve mientras sigues una ruta.
- **Zonas de farmeo:** `Alt+R` muestra los mapas con zonas registradas, pendientes o todos, y los mapas por explorar.

## Cómo se usa

1. Descarga `bitacora-local.exe` y ábrelo. Si Windows lo bloquea: Más información, Ejecutar de todas formas.
2. Deja su ventana abierta mientras juegas.
3. En la web, entra a tu grupo y pulsa **Ventanas sobre el juego**. Solo hace falta la primera vez.
4. Pulsa **Detectar mi mapa solo** para que la web siga tu mapa por pantalla.

Funciona con GeForce Now y con el juego instalado.

## Qué hace y qué no

- No toca el juego ni lee su tráfico: desde la versión 1.2 ya no incluye detección por red, Albion Data Client ni Npcap.
- Solo abre las páginas de la Bitácora con WebView2, que ya trae Windows 11.
- El servidor local en `127.0.0.1:47800` solo sirve para que la web le pase el grupo a las ventanas, y rechaza a cualquier otra web.
- Al abrirse borra las descargas anteriores de sí mismo que estén en su misma carpeta, como `bitacora-local (1).exe`. Solo toca archivos con ese nombre, más antiguos y que contengan este programa.

Albion debe estar en ventana sin bordes: en pantalla completa exclusiva ninguna ventana puede ir encima.

## Compilar

```
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags '-s -w' -o bitacora-local.exe .
```
