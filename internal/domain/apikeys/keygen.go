package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

const (
	// Prefix marca visualmente el secreto en logs, dumps y variables de entorno,
	// para que sea obvio que es una credencial y no un identificador cualquiera.
	Prefix = "emb_"

	// KeyIDLen es el largo del key_id en caracteres hexadecimales (6 bytes).
	// Hexadecimal —y no base64url— porque el alfabeto base64url incluye "_",
	// que es justamente el separador del formato: un key_id con "_" adentro
	// haria ambiguo el parseo.
	KeyIDLen = 12

	// SecretBytes es la entropia del secreto. 32 bytes es lo que justifica usar
	// SHA-256 y no bcrypt (D-4): no hay diccionario que atacar.
	SecretBytes = 32
)

// ErrMalformedKey indica que el string recibido no tiene la forma
// emb_<key_id>_<secreto>. Se devuelve sin detalle de que parte fallo: el
// llamador lo traduce a 403 y no debe filtrar en que se equivoco el cliente.
var ErrMalformedKey = errors.New("apikeys: formato de key invalido")

// Generate produce una API key nueva. Devuelve el texto en claro —que se le
// muestra al usuario UNA sola vez y no se persiste jamas—, el key_id publico
// que va indexado en Postgres, y el sha256 del secreto que si se guarda.
func Generate() (plaintext string, keyID string, hash []byte, err error) {
	idBytes := make([]byte, KeyIDLen/2)
	if _, err = rand.Read(idBytes); err != nil {
		return "", "", nil, err
	}
	keyID = hex.EncodeToString(idBytes)

	secretBytes := make([]byte, SecretBytes)
	if _, err = rand.Read(secretBytes); err != nil {
		return "", "", nil, err
	}
	secret := base64.RawURLEncoding.EncodeToString(secretBytes)

	return Prefix + keyID + "_" + secret, keyID, HashSecret(secret), nil
}

// Parse separa una key en claro en su key_id y su secreto.
func Parse(plaintext string) (keyID string, secret string, err error) {
	rest, ok := strings.CutPrefix(plaintext, Prefix)
	if !ok {
		return "", "", ErrMalformedKey
	}

	// SplitN con limite 2: el secreto es base64url y puede contener "_",
	// asi que solo el PRIMER separador cuenta.
	parts := strings.SplitN(rest, "_", 2)
	if len(parts) != 2 {
		return "", "", ErrMalformedKey
	}
	keyID, secret = parts[0], parts[1]

	if len(keyID) != KeyIDLen || secret == "" {
		return "", "", ErrMalformedKey
	}
	if _, err := hex.DecodeString(keyID); err != nil {
		return "", "", ErrMalformedKey
	}
	return keyID, secret, nil
}

// HashSecret devuelve sha256(secreto). Es lo unico que se persiste.
func HashSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// Matches compara el secreto contra un hash almacenado en tiempo constante.
// El tiempo constante importa aunque el hash no sea secreto: sin el, la
// latencia de la comparacion filtra cuantos bytes iniciales acerto un atacante.
func Matches(secret string, hash []byte) bool {
	return subtle.ConstantTimeCompare(HashSecret(secret), hash) == 1
}
