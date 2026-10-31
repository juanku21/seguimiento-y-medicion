package delivery

import (
	"errors"
	"go/build"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// modulePath es la ruta del módulo de Go del backend.
const modulePath = "github.com/juanku21/seguimiento-y-medicion/backend"

// Caracterización de RN-01 y SC-021: las fórmulas viven en un solo lugar. Ningún paquete fuera de
// internal/metrics, internal/reports y cmd/app importa internal/metrics/..., e internal/metrics/delivery no importa la
// capa delivery de otro módulo.
func TestMetrics_SingleSourceOfFormulas(t *testing.T) {
	root, err := filepath.Abs("../../..")
	require.NoError(t, err)
	metricsPrefix := modulePath + "/internal/metrics"

	// Recorre todos los directorios del backend y lee los imports de cada paquete con go/build.
	require.NoError(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if name := entry.Name(); path != root && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor") {
			return filepath.SkipDir
		}
		pkg, err := build.ImportDir(path, 0)
		if err != nil {
			var none *build.NoGoError
			if errors.As(err, &none) {
				return nil
			}
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		// Regla 1: solo internal/metrics, internal/reports y cmd/app importan internal/metrics/...
		// Los reportes (spec 010) consumen las métricas ya calculadas y no repiten sus fórmulas.
		allowed := rel == "internal/metrics" || strings.HasPrefix(rel, "internal/metrics/") ||
			rel == "internal/reports" || strings.HasPrefix(rel, "internal/reports/") || rel == "cmd/app"
		if !allowed {
			all := append(append(append([]string{}, pkg.Imports...), pkg.TestImports...), pkg.XTestImports...)
			for _, imp := range all {
				assert.False(t, imp == metricsPrefix || strings.HasPrefix(imp, metricsPrefix+"/"),
					"el paquete %s no puede importar %s", rel, imp)
			}
		}

		// Regla 2: delivery de métricas no importa el delivery de otro módulo (código de producción).
		// Única excepción, prevista en tasks.md: ErrorResponse de la 001, compartido por todos los módulos.
		if rel == "internal/metrics/delivery" {
			for _, imp := range pkg.Imports {
				other := strings.HasPrefix(imp, modulePath+"/internal/") && strings.HasSuffix(imp, "/delivery") &&
					imp != metricsPrefix+"/delivery" && imp != modulePath+"/internal/auth/delivery"
				assert.False(t, other, "internal/metrics/delivery no puede importar %s", imp)
			}
		}
		return nil
	}))
}
