package files

import (
	"log/slog"
	"net/http"

	"openscum/internal/apperr"
)

var (
	errFileTooLarge       = apperr.New(http.StatusRequestEntityTooLarge, "file_too_large", "El archivo supera 10 MB")
	errFileTypeNotAllowed = apperr.New(http.StatusUnsupportedMediaType, "file_type_not_allowed", "Tipo de archivo no admitido")
	errStorageUnavailable = apperr.New(http.StatusServiceUnavailable, "storage_unavailable", "El almacenamiento de archivos no está disponible")
)

// unavailable logs why S3 failed and answers the client without the details.
func unavailable(err error) error {
	slog.Warn("almacenamiento de archivos no disponible", slog.String("error", err.Error()))
	return errStorageUnavailable
}
