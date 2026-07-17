//go:build darwin

package pdftext

/*
#cgo CFLAGS: -x objective-c -fmodules
#cgo LDFLAGS: -framework Foundation -framework PDFKit
#include <stdlib.h>
const char *extractPDFText(const char *path);
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	"github.com/goodblaster/errors"
)

// pdfkitBackend extracts via Apple PDFKit (pdftext.m). Codepoint-perfect
// for Latin, CJK (including vertical), and Arabic; known to drop and
// reorder Devanagari matras — prefer poppler for Indic scripts.
type pdfkitBackend struct{}

func init() { backends = append(backends, pdfkitBackend{}) }

func (pdfkitBackend) Name() string    { return "pdfkit" }
func (pdfkitBackend) Available() bool { return true }

func (pdfkitBackend) Extract(path string) ([]Page, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	result := C.extractPDFText(cPath)
	defer C.free(unsafe.Pointer(result))

	raw := []byte(C.GoString(result))

	var failure struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &failure); err == nil && failure.Error != "" {
		return nil, errors.New(failure.Error)
	}

	var pages []Page
	if err := json.Unmarshal(raw, &pages); err != nil {
		return nil, errors.Wrap(err, "failed to parse PDFKit JSON")
	}
	foldText(pages)
	return pages, nil
}
