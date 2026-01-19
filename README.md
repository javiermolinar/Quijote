# Quijote
![Descubriendo-el-Quijote-Urda-1](https://github.com/user-attachments/assets/1a63d6f4-b6ef-49fc-ae7c-b84b58c1b13d)


Lee "Don Quijote de la Mancha" en tu terminal, con interfaz interactiva, busqueda de capitulos y lectura paginada.

Fuente del HTML:
https://www.gutenberg.org/files/2000/2000-h/2000-h.htm


<img width="1013" height="1158" alt="Screenshot 2026-01-16 at 19 10 21" src="https://github.com/user-attachments/assets/84cffeb1-e97e-48b7-bac1-c504ba316904" />



## Requisitos
- Terminal con soporte ANSI

## Uso rapido

```bash
./quijote
```

## Compilacion local

```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -o quijote .
```

## Instalacion (binarios)

1) Descarga el binario desde Releases:
   https://github.com/javiermolinar/Quijote/releases
2) Dale permisos de ejecucion (macOS/Linux):

```bash
chmod +x quijote-<tu-sistema>
```

3) Ejecuta:

```bash
./quijote-<tu-sistema>
```

## Comandos

```bash
quijote             (interfaz interactiva)
quijote ui
quijote list
quijote read [-n paginas]
quijote status
quijote goto <numero-capitulo>
quijote reset
```

## Controles en la interfaz

- Enter/Espacio: siguiente pagina
- b/pgup: pagina anterior
- l: lista de capitulos
- q: salir

## Progreso

El progreso se guarda en el archivo `.quijote_state.json` en el directorio del proyecto.

## Notas

- La lista de capitulos es interactiva y permite buscar escribiendo.
- El texto se adapta al tamano del terminal con minimos de ancho/alto.

## Releases (GitHub)

Este repo incluye GoReleaser y un workflow de GitHub Actions para generar binarios:

1) Etiqueta una version y subela:

```bash
git tag v0.1.0
git push origin v0.1.0
```

2) El workflow creara un Release con binarios para:
   - darwin/arm64
   - darwin/amd64
   - linux/amd64
   - windows/amd64
