package delivery

import "github.com/juanku21/seguimiento-y-medicion/backend/internal/auth/service"

// Handler agrupa los manejadores HTTP de la feature de autenticación sobre el servicio. Firma
// mínima creada por T027 para que compilen las pruebas de integración; los manejadores los agrega
// T034 (registerUser) y las historias siguientes.
type Handler struct {
	auth *service.AuthService
}

// NewHandler crea el conjunto de manejadores con el servicio de autenticación recibido.
func NewHandler(auth *service.AuthService) *Handler {
	return &Handler{auth: auth}
}
