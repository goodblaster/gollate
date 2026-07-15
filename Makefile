GO_BIN=$(shell which go)

# Variables
VERSION=$(shell git describe --tags --always)
COMMIT=$(shell git rev-parse --short HEAD)
BUILT_AT=$(shell date +%FT%T%z)
BUILT_BY=$(USER)
BUILT_ON=$(shell hostname)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)

# Targets
.PHONY: all build install clean test utils integration baselines sort

all: build

build:
	@echo "Building gollate..."
	@mkdir -p bin
	$(GO_BIN) build -o bin/gollate ./cmd/gollate

utils:
	@echo "Building utilities..."
	@mkdir -p bin
	$(GO_BIN) build -o bin/ocr-util ./cmd/ocr-util
	$(GO_BIN) build -o bin/ocr-highlight ./cmd/ocr-highlight
	$(GO_BIN) build -o bin/tesseract-util ./cmd/tesseract-util
	$(GO_BIN) build -o bin/pdftext-util ./cmd/pdftext-util
	$(GO_BIN) build -o bin/testdoc ./cmd/testdoc

# Run the OCR integration suite (sorts committed OCR fixtures, checks
# accuracy against testdata/ocr-tests/baselines.json).
integration:
	$(GO_BIN) test ./integration -run TestOCRSorting -v

# Re-run the suite and ratchet baselines up where accuracy improved.
baselines:
	UPDATE_BASELINES=1 $(GO_BIN) test ./integration -run TestOCRSorting -v

# make sort — full pipeline for one document: rasterize (if PDF), OCR, sort.
# Sorted output goes to stdout (redirect it to a file); all progress and
# build noise go to stderr, so `make sort ... > out.json` yields clean output.
# macOS only (Apple Vision, sips, CoreGraphics rasterization).
#
# Required: IMG (image or PDF), TEXT (canonical text file).
# Optional: ENGINE (apple|tesseract; default apple on macOS, tesseract else),
#           LANG (sorter language, default english), OCR_LANG (engine lang
#           codes, e.g. ja-JP or jpn+jpn_vert+eng), FORMAT (json|text,
#           default json).
#
#   make sort IMG=doc.pdf TEXT=canonical.txt LANG=english > sorted.json
ENGINE ?=
LANG ?= english
FORMAT ?= json
OCR_LANG ?=

sort:
	@$(MAKE) --no-print-directory build utils >&2
	@scripts/sort.sh -i "$(IMG)" -t "$(TEXT)" -e "$(ENGINE)" -l "$(LANG)" -L "$(OCR_LANG)" -f "$(FORMAT)"

install:
	@echo "Installing gollate to $(GOPATH)/bin..."
	$(GO_BIN) install ./cmd/gollate

clean:
	@echo "Cleaning up..."
	rm -rf bin/

test:
	@echo "Running tests..."
	$(GO_BIN) test ./...
