// Package apporigin valida el origin del frontend que un request dice tener,
// para construir links que se envian por mail. Es la unica barrera entre un
// header controlable por el llamador y una URL que termina en la casilla de
// un usuario, asi que el matching es exacto por diseño.
package apporigin

import (
	"net/url"
	"strings"
)

// AllowList es el conjunto de origins en los que el backend confia.
type AllowList struct {
	exact    []string
	wildcard []wildcardEntry
}

type wildcardEntry struct {
	scheme string
	suffix string // incluye el punto inicial, p.ej. ".vercel.app"
}

// Parse construye una AllowList a partir de una lista separada por comas.
// Una entrada con la forma "https://*.example.com" acepta cualquier subdominio
// de example.com bajo ese esquema, pero nunca example.com a secas. Las entradas
// que no parsean se descartan en silencio: una config con una entrada rota no
// debe voltear el arranque del servicio.
func Parse(raw string) AllowList {
	var a AllowList
	for _, item := range strings.Split(raw, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		if scheme, rest, found := strings.Cut(item, "://"); found && strings.HasPrefix(rest, "*.") {
			if scheme != "http" && scheme != "https" {
				continue
			}
			a.wildcard = append(a.wildcard, wildcardEntry{scheme: scheme, suffix: rest[1:]})
			continue
		}
		if origin, ok := Normalize(item); ok {
			a.exact = append(a.exact, origin)
		}
	}
	return a
}

// Normalize reduce una URL cruda a su origin: esquema://host[:puerto], en
// minusculas y sin path, query, fragment, userinfo ni barra final. Devuelve
// false si el valor no es una URL absoluta http(s).
func Normalize(raw string) (string, bool) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	if u.Host == "" {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}

// Allows indica si el origin ya normalizado esta permitido.
func (a AllowList) Allows(origin string) bool {
	for _, e := range a.exact {
		if e == origin {
			return true
		}
	}
	for _, w := range a.wildcard {
		prefix := w.scheme + "://"
		if !strings.HasPrefix(origin, prefix) {
			continue
		}
		host := origin[len(prefix):]
		// Exigir al menos una etiqueta propia antes del sufijo: con
		// "https://*.vercel.app", tanto "vercel.app" como ".vercel.app"
		// tienen que quedar afuera.
		if len(host) > len(w.suffix) && strings.HasSuffix(host, w.suffix) {
			return true
		}
	}
	return false
}

// Resolve devuelve el base URL a usar para links salientes. Cuando candidate
// esta vacio o no esta permitido devuelve fallback y false, para que el
// llamador pueda loguear el rechazo sin hacer fallar el request.
func (a AllowList) Resolve(candidate, fallback string) (string, bool) {
	origin, ok := Normalize(candidate)
	if !ok || !a.Allows(origin) {
		return fallback, false
	}
	return origin, true
}
