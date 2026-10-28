package domain

import "errors"

// ErrSprintNotFound indica que el sprint no existe o pertenece a otro proyecto. Delivery lo traduce
// a 404 con el mismo cuerpo que la spec 004, de modo que ambos casos sean indistinguibles.
var ErrSprintNotFound = errors.New("el sprint no existe o es de otro proyecto")
