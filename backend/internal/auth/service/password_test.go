package service

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// Pruebas unitarias del hash de contraseña del registro (T026). Los nombres llevan el prefijo
// "Hash" para no chocar con otras pruebas de US1 del paquete service.

// validPassword es la contraseña del caso normal de la spec.
const validPassword = "Proyecto2026"

// Escenario: Alta exitosa (001/US1)
// Cubre RN-05 y FR-010: la contraseña del alta se conserva solo como hash bcrypt, que no permite
// recuperarla, mide 60 caracteres, cambia en cada cálculo por la sal y nunca contiene ni expone el
// texto original.
func TestUS1_HashAltaExitosa(t *testing.T) {
	// El hash es bcrypt con el coste por defecto, distinto del texto original y solo verificable
	// recalculando: acepta la contraseña original y rechaza cualquier otra.
	t.Run("el hash no es reversible", func(t *testing.T) {
		hash, err := HashPassword(validPassword)
		if err != nil {
			t.Fatalf("HashPassword devolvió un error inesperado: %v", err)
		}
		if hash == validPassword {
			t.Fatalf("el hash es igual a la contraseña en claro")
		}
		cost, err := bcrypt.Cost([]byte(hash))
		if err != nil {
			t.Fatalf("el resultado no es un hash bcrypt válido: %v", err)
		}
		if cost != bcrypt.DefaultCost {
			t.Errorf("coste bcrypt = %d, se esperaba %d", cost, bcrypt.DefaultCost)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(validPassword)); err != nil {
			t.Errorf("el hash no verifica la contraseña original: %v", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("Proyecto2027")); err == nil {
			t.Errorf("el hash verifica una contraseña distinta de la original")
		}
	})

	// La salida de bcrypt tiene largo fijo, que es el ancho de la columna password_hash.
	t.Run("el hash mide 60 caracteres", func(t *testing.T) {
		hash, err := HashPassword(validPassword)
		if err != nil {
			t.Fatalf("HashPassword devolvió un error inesperado: %v", err)
		}
		if got := utf8.RuneCountInString(hash); got != 60 {
			t.Errorf("largo del hash = %d caracteres, se esperaban 60", got)
		}
	})

	// La sal aleatoria hace que dos cálculos sobre la misma contraseña produzcan hashes distintos,
	// ambos válidos para ella.
	t.Run("dos hashes de la misma contraseña difieren", func(t *testing.T) {
		first, err := HashPassword(validPassword)
		if err != nil {
			t.Fatalf("primer HashPassword devolvió un error inesperado: %v", err)
		}
		second, err := HashPassword(validPassword)
		if err != nil {
			t.Fatalf("segundo HashPassword devolvió un error inesperado: %v", err)
		}
		if first == "" || first == second {
			t.Fatalf("se esperaban dos hashes no vacíos y distintos, se obtuvo %q y %q", first, second)
		}
		for _, hash := range []string{first, second} {
			if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(validPassword)); err != nil {
				t.Errorf("el hash %q no verifica la contraseña original: %v", hash, err)
			}
		}
	})

	// Ni el hash ni el registro de actividad del proceso contienen la contraseña en claro. Se
	// redirige la salida del paquete log, que es también el destino por defecto de slog.
	t.Run("la contraseña en claro no aparece en ninguna salida", func(t *testing.T) {
		var logs bytes.Buffer
		previous := log.Writer()
		log.SetOutput(&logs)
		t.Cleanup(func() { log.SetOutput(previous) })

		hash, err := HashPassword(validPassword)
		if err != nil {
			t.Fatalf("HashPassword devolvió un error inesperado: %v", err)
		}
		if hash == "" {
			t.Fatalf("HashPassword devolvió un hash vacío")
		}
		if strings.Contains(hash, validPassword) {
			t.Errorf("el hash contiene la contraseña en claro")
		}
		if strings.Contains(logs.String(), validPassword) {
			t.Errorf("el registro de actividad contiene la contraseña en claro")
		}
	})

	// Si bcrypt rechaza la contraseña (más de 72 bytes, que delivery ya filtra antes), el error que
	// se propaga no la repite y no se devuelve ningún hash.
	t.Run("el error de bcrypt no expone la contraseña", func(t *testing.T) {
		tooLong := validPassword + strings.Repeat("a", 61)
		hash, err := HashPassword(tooLong)
		if err == nil {
			t.Fatalf("se esperaba un error para una contraseña de %d bytes", len(tooLong))
		}
		if hash != "" {
			t.Errorf("se devolvió un hash junto con el error: %q", hash)
		}
		if strings.Contains(err.Error(), tooLong) || strings.Contains(err.Error(), validPassword) {
			t.Errorf("el mensaje de error contiene la contraseña en claro: %q", err.Error())
		}
	})
}
