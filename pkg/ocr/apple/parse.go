//go:build darwin

package apple

/*
#cgo CFLAGS: -x objective-c -fmodules
#cgo LDFLAGS: -framework Foundation -framework Vision
#include <stdlib.h>
const char *performAppleVisionOCR(const void *imageBytes, size_t length, const char **langs, size_t langsCount, int recognitionLevel);
*/
import "C"

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"unsafe"

	"github.com/goodblaster/errors"
	"github.com/goodblaster/gollate/pkg/language"
)

// LanguageSettingsFromHandler converts language handler OCR settings to parameters
// suitable for Apple Vision OCR. This demonstrates how handler settings map to
// actual OCR configuration.
//
// Note: This is primarily for documentation/examples. In practice, OCR runs before
// language detection, so language codes are typically provided directly via CLI flags
// or API parameters.
func LanguageSettingsFromHandler(handler language.Handler) (langs []string, recognitionLevel int) {
	settings := handler.OCRSettings()

	// Use language codes from handler
	langs = settings.LanguageCodes
	if len(langs) == 0 {
		langs = []string{"en-US"}
	}

	// Convert recognition level string to int
	// "fast" -> 0, "accurate" -> 1
	if strings.ToLower(settings.RecognitionLevel) == "accurate" {
		recognitionLevel = 1
	} else {
		recognitionLevel = 0
	}

	return langs, recognitionLevel
}

func (engine *Engine) ParseBytes(b []byte, langs []string) ([]Line, error) {
	// If langs is nil, use default "en-US"
	if langs == nil {
		langs = []string{"en-US"}
	}

	// Always use accurate recognition mode for best quality
	// 0 = fast, 1 = accurate
	recognitionLevel := 1

	// Convert Go slice of strings to a slice of *C.char
	cLangArray := make([]*C.char, len(langs))
	for i, lang := range langs {
		cLangArray[i] = C.CString(lang)
	}
	// Ensure that allocated C strings are freed later.
	defer func() {
		for _, cStr := range cLangArray {
			C.free(unsafe.Pointer(cStr))
		}
	}()

	// Perform OCR using the converted array.
	result := C.performAppleVisionOCR(
		unsafe.Pointer(&b[0]),
		C.size_t(len(b)),
		(**C.char)(unsafe.Pointer(&cLangArray[0])),
		C.size_t(len(cLangArray)),
		C.int(recognitionLevel),
	)
	defer C.free(unsafe.Pointer(result))

	// Parse JSON result
	var lines []Line
	if err := json.Unmarshal([]byte(C.GoString(result)), &lines); err != nil {
		return nil, errors.Wrap(err, "failed to parse JSON")
	}

	return lines, nil
}

func (engine *Engine) ParseReader(r io.Reader, langs []string) ([]Line, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read image reader")
	}

	return engine.ParseBytes(b, langs)
}

func (engine *Engine) ParseFile(imagePath string, langs []string) ([]Line, error) {
	b, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read image file")
	}

	return engine.ParseBytes(b, langs)
}
