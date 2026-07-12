#!/usr/bin/env bash
# Uniform PDF -> PNG rasterization for OCR input (macOS, no dependencies).
#
# OCR engines (Apple Vision, Tesseract) consume raster images, not PDFs, so
# every PDF must be rasterized first. This script is the one supported way
# to do that: CoreGraphics via a small Swift program, so results are
# consistent across machines without ghostscript/pdftoppm.
#
# Usage:
#   scripts/pdf-to-png.sh <input.pdf> [output.png] [scale]
#
#   output.png  default: <input-basename>.png next to the PDF
#   scale       default: 2.0 (the suite/case-study convention: 2x rasters,
#               e.g. Apple.pdf 1924x8788pt -> 3848x17576px)
#
# Multi-page PDFs produce <output>-<page>.png per page.
# Prints the pixel dimensions of each page written.
set -euo pipefail

if [ $# -lt 1 ] || [ $# -gt 3 ]; then
  echo "usage: $0 <input.pdf> [output.png] [scale]" >&2
  exit 2
fi

in="$1"
out="${2:-${in%.pdf}.png}"
scale="${3:-2.0}"

if [ ! -f "$in" ]; then
  echo "error: no such file: $in" >&2
  exit 1
fi

swift_src="$(mktemp -t pdf-rasterize).swift"
trap 'rm -f "$swift_src"' EXIT
cat > "$swift_src" <<'EOF'
import CoreGraphics
import ImageIO
import Foundation
import UniformTypeIdentifiers

let args = CommandLine.arguments
guard args.count == 4, let scale = Double(args[3]) else {
    FileHandle.standardError.write("usage: rasterize <in.pdf> <out.png> <scale>\n".data(using: .utf8)!)
    exit(2)
}
guard let doc = CGPDFDocument(URL(fileURLWithPath: args[1]) as CFURL), doc.numberOfPages > 0 else {
    FileHandle.standardError.write("error: cannot open PDF\n".data(using: .utf8)!)
    exit(1)
}

let outPath = args[2]
for pageNum in 1...doc.numberOfPages {
    guard let page = doc.page(at: pageNum) else { continue }
    let box = page.getBoxRect(.mediaBox)
    let w = Int(box.width * scale), h = Int(box.height * scale)
    guard let ctx = CGContext(data: nil, width: w, height: h, bitsPerComponent: 8,
                              bytesPerRow: 0, space: CGColorSpaceCreateDeviceRGB(),
                              bitmapInfo: CGImageAlphaInfo.noneSkipLast.rawValue) else {
        FileHandle.standardError.write("error: cannot create \(w)x\(h) context (page too large?)\n".data(using: .utf8)!)
        exit(1)
    }
    ctx.setFillColor(CGColor(red: 1, green: 1, blue: 1, alpha: 1))
    ctx.fill(CGRect(x: 0, y: 0, width: w, height: h))
    ctx.scaleBy(x: scale, y: scale)
    ctx.drawPDFPage(page)

    var path = outPath
    if doc.numberOfPages > 1 {
        let base = outPath.hasSuffix(".png") ? String(outPath.dropLast(4)) : outPath
        path = "\(base)-\(pageNum).png"
    }
    let img = ctx.makeImage()!
    guard let dest = CGImageDestinationCreateWithURL(URL(fileURLWithPath: path) as CFURL,
                                                     UTType.png.identifier as CFString, 1, nil) else {
        FileHandle.standardError.write("error: cannot write \(path)\n".data(using: .utf8)!)
        exit(1)
    }
    CGImageDestinationAddImage(dest, img, nil)
    CGImageDestinationFinalize(dest)
    print("\(path): \(w)x\(h)")
}
EOF

exec swift "$swift_src" "$in" "$out" "$scale"
