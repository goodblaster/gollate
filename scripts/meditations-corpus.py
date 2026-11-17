#!/usr/bin/env python3
"""Build a real-prose OCR corpus from Public Domain Library's Meditations EPUB.

The EPUB's reading-order XHTML is the only canonical-text source. Rendering
and OCR are separate stages; the PDF text layer is an input control only.
Large generated artifacts live under experiments/ (ignored by git).
"""
import argparse
import concurrent.futures
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess
import tempfile
import time
import urllib.request
import xml.etree.ElementTree as ET
import zipfile

ROOT = Path(__file__).resolve().parents[1]
URL = "https://publicdomainlibrary.org/uploads/attachments/oe5bkjmpegmldniwbcy27p2w-0027-meditations-marcus-aurelius.epub"
EPUB_SHA256 = "0da41e46edab352c9f8b394234a82dc0c0851d0d4a8b7876763e855883769ed7"
BOOK_URL = "https://publicdomainlibrary.org/en/ebooks/meditations#formats"
LAYOUTS = ("single-column", "double-column")
SOURCES = ("apple", "tesseract", "pdftext")
EPUB_TYPE = "{http://www.idpf.org/2007/ops}type"
NS = {"o": "http://www.idpf.org/2007/opf", "dc": "http://purl.org/dc/elements/1.1/"}


def write_bytes(path, data):
    """Publish complete files atomically; leave identical cached files untouched."""
    if path.exists() and path.read_bytes() == data:
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as stream:
        temporary = Path(stream.name)
        stream.write(data)
    try:
        temporary.replace(path)
    finally:
        temporary.unlink(missing_ok=True)


def write_text(path, text):
    write_bytes(path, text.encode("utf-8"))


def write_json(path, value):
    write_text(path, json.dumps(value, ensure_ascii=False, indent=2) + "\n")


def read_json(path):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return None


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def outputs_match(root, outputs):
    return bool(outputs) and all((root / name).is_file() and digest(root / name) == sha
                                 for name, sha in outputs.items())


def cached(root, stage, inputs):
    state = read_json(root / ("." + stage + "-state.json"))
    return bool(state and state.get("inputs") == inputs and outputs_match(root, state.get("outputs")))


def remember(root, stage, inputs, paths):
    write_json(root / ("." + stage + "-state.json"), {
        "inputs": inputs, "outputs": {str(p.relative_to(root)): digest(p) for p in paths}})


def download(source):
    source.mkdir(parents=True, exist_ok=True)
    epub = source / "meditations.epub"
    if epub.exists():
        if digest(epub) != EPUB_SHA256:
            raise ValueError(f"EPUB checksum mismatch: {epub}; use a fresh --out directory or replace the damaged file")
        return epub
    # A failed transfer must not leave a partial EPUB that looks cached.
    with tempfile.TemporaryDirectory(prefix="download-", dir=source) as folder:
        temporary = Path(folder) / "meditations.epub"
        request = urllib.request.Request(URL, headers={"User-Agent": "gollate-corpus/1.0"})
        with urllib.request.urlopen(request, timeout=60) as response, temporary.open("wb") as stream:
            shutil.copyfileobj(response, stream)
        if digest(temporary) != EPUB_SHA256:
            raise ValueError("Downloaded EPUB checksum mismatch; the upstream edition may have changed")
        temporary.replace(epub)
    print(f"Downloaded and verified {epub}", flush=True)
    return epub


def clean(text):
    return (text or "").replace("\u2060", "").replace("\u00ad", "")


def visible_text(node):
    return " ".join("".join(node.itertext()).split())


def validate_rendered_text(document, canonical):
    # Independently walk the rendered HTML, including text outside p tags.
    # Source citations/verse attributions must not silently disappear from
    # the reference while remaining visible in the image.
    from html.parser import HTMLParser
    class BodyText(HTMLParser):
        def __init__(self):
            super().__init__()
            self.body, self.parts = False, []
        def handle_starttag(self, tag, attrs):
            if tag == "body": self.body = True
            if self.body and tag in {"p", "h1", "h2", "h3", "br"}: self.parts.append(" ")
        def handle_endtag(self, tag):
            if self.body and tag in {"p", "h1", "h2", "h3"}: self.parts.append(" ")
            if tag == "body": self.body = False
        def handle_data(self, data):
            if self.body: self.parts.append(data)
    parser = BodyText()
    parser.feed(document)
    rendered = " ".join("".join(parser.parts).split())
    expected = " ".join(canonical.split())
    if rendered != expected:
        at = next((i for i, pair in enumerate(zip(rendered, expected)) if pair[0] != pair[1]), min(len(rendered), len(expected)))
        raise ValueError(f"Rendered body differs from EPUB-derived canonical text at {at}: {rendered[at:at+80]!r} != {expected[at:at+80]!r}")


def prepare(out):
    source = out / "source"
    epub = download(source)
    sections = []
    provenance = []
    with zipfile.ZipFile(epub) as archive:
        package = ET.fromstring(archive.read("epub/content.opf"))
        items = {x.attrib["id"]: x.attrib["href"] for x in package.find("o:manifest", NS)}
        selected = [items[x.attrib["idref"]] for x in package.find("o:spine", NS)
                    if re.fullmatch(r"text/(book-\d+|endnotes)\.xhtml", items[x.attrib["idref"]])]
        metadata = package.find("o:metadata", NS)
        title = metadata.find("dc:title", NS).text
        author = metadata.find("dc:creator", NS).text
        translator = next(x.text for x in metadata.findall("dc:contributor", NS) if x.attrib.get("id") == "translator")
        canonical = [title, author, "Translated by " + translator]
        provenance.extend({"line": i, "source": "epub/content.opf", "kind": "metadata"} for i in range(3))
        for path in selected:
            root = ET.fromstring(archive.read("epub/" + path))
            body = next(x for x in root if x.tag.endswith("}body"))
            body = copy.deepcopy(body)
            for el in body.iter():
                kind = el.attrib.get(EPUB_TYPE, "")
                el.tag = el.tag.rsplit("}", 1)[-1]
                el.text, el.tail = clean(el.text), clean(el.tail)
                el.attrib.clear()
                if kind == "noteref":
                    el.tag = "sup"
                    el.set("class", "noteref")
                    el.text = " " + el.text
                    el.tail = " " + el.tail
                elif kind == "backlink":
                    el.text = ""  # ebook navigation, not prose
                elif el.tag == "a":
                    el.tag = "span"  # preserve visible cross-reference text
            # Make endnote numbers explicit in BOTH source text and rendering,
            # rather than relying on CSS-generated list markers invisible to XML.
            if path.endswith("endnotes.xhtml"):
                for n, li in enumerate(body.iter("li"), 1):
                    first = next(li.iter("p"))
                    first.text = f"{n}. " + (first.text or "")
            for el in body.iter():
                if el.tag == "br":
                    el.tail = " " + (el.tail or "")
            parents = {child: parent for parent in body.iter() for child in parent}
            for el in body.iter():
                selected_block = el.tag in {"h2", "h3", "p", "cite"}
                parent = parents.get(el)
                while selected_block and parent is not None:
                    if parent.tag in {"h2", "h3", "p", "cite"}:
                        selected_block = False
                    parent = parents.get(parent)
                if selected_block:
                    text = visible_text(el)
                    if text:
                        provenance.append({"line": len(canonical), "source": "epub/" + path, "kind": el.tag})
                        canonical.append(text)
            sections.extend(list(body))
        source_info = {
            "page_url": BOOK_URL, "download_url": URL,
            "sha256": hashlib.sha256(epub.read_bytes()).hexdigest(),
            "title": title, "author": author, "translator": translator,
            "rights": metadata.find("dc:rights", NS).text,
            "epub_sources": [x.text for x in metadata.findall("dc:source", NS)],
            "selected_spine_documents": selected,
            "excluded": "cover, publisher title art, public-domain notice, donation and acknowledgments pages",
            "normalization": "Collapse whitespace; remove soft hyphens and word joiners; separate superscript note references; omit backlink arrows; make endnote list numbers explicit.",
            "canonical_lines": len(canonical),
            "whitespace_words": len(" ".join(canonical).split()),
        }
    text = "\n".join(canonical) + "\n"
    write_text(source / "canonical.txt", text)
    write_json(source / "provenance.json", source_info)
    write_json(source / "canonical-map.json", provenance)
    import html
    front = f"<h1>{html.escape(title)}</h1><p class='author'>{html.escape(author)}</p><p class='author'>Translated by {html.escape(translator)}</p>"
    content = "\n".join(ET.tostring(el, encoding="unicode", method="html") for el in sections)
    for name, columns in [("single-column", 1), ("double-column", 2)]:
        dest = out / name
        dest.mkdir(parents=True, exist_ok=True)
        css = f"""
@page {{ size: 7in 9in; margin: .65in .6in .65in;
  @top-center {{ content: 'Meditations'; font: italic 8pt Georgia; }}
  @bottom-center {{ content: counter(page); font: 8pt Georgia; }} }}
html, body {{ padding:0; margin:0; }}
body {{ font: 11pt/1.35 Georgia, 'Times New Roman', serif; column-count:{columns}; column-gap:22pt; }}
h1 {{ column-span:all; text-align:center; font-size:20pt; margin:.2in 0 .1in; }}
p.author {{ text-align:center; text-indent:0; margin:0 0 .1in; }}
h2 {{ font-size:14pt; margin:18pt 0 10pt; break-after:avoid; text-align:center; }}
p {{ margin:0 0 5pt; text-align:justify; text-indent:1em; hyphens:auto; orphans:2; widows:2; }}
blockquote {{ margin:6pt 10pt; font-style:italic; }}
blockquote p {{ text-indent:0; }}
ol {{ list-style:none; margin:0; padding:0; }}
li {{ margin:0 0 6pt; }}
sup.noteref {{ font-size:70%; line-height:0; }}
a,span {{ color:inherit; }}
"""
        document = "<!doctype html><html lang='en-GB'><head><meta charset='UTF-8'><title>Meditations</title><style>" + css + "</style></head><body>" + front + content + "</body></html>"
        validate_rendered_text(document, text)
        write_text(dest / "document.html", document)
        write_text(dest / "canonical.txt", text)
        write_json(dest / "render-settings.json", {"columns": columns, "paper_inches": [7, 9], "font_points": 11, "line_height": 1.35, "dpi": 144, "canonical_sha256": hashlib.sha256(text.encode()).hexdigest()})
    print(f"Extracted {source_info['whitespace_words']:,} words in {len(canonical)} canonical units, all 12 books plus endnotes", flush=True)


def run(command, log, env=None):
    start = time.perf_counter()
    with log.open("w") as stream:
        subprocess.run([str(x) for x in command], stdout=stream, stderr=subprocess.STDOUT, check=True, env=env)
    return time.perf_counter() - start


def render_inputs(dest):
    return {"html_sha256": digest(dest / "document.html"), "dpi": 144, "pipeline": 1}


def page_images(dest):
    info = read_json(dest / "info.json")
    if not info or not info.get("pages"):
        raise ValueError(f"Missing page metadata in {dest}; run --stage render first")
    return [dest / f"page-{n}.png" for n in range(1, len(info["pages"]) + 1)]


def render(out, chrome):
    for name in LAYOUTS:
        dest = out / name
        inputs = render_inputs(dest)
        if cached(dest, "render", inputs):
            print(f"{name}: PDF and page images cached", flush=True)
            continue
        start = time.perf_counter()
        # Generate in a temporary directory, so an interrupted run cannot mix
        # old pages with new pagination. Only info.json determines active pages.
        with tempfile.TemporaryDirectory(prefix="render-", dir=dest) as folder:
            temporary = Path(folder)
            pdf = temporary / "document.pdf"
            run([chrome, "--headless", "--disable-background-networking", "--disable-component-update", "--disable-sync", "--no-first-run", "--no-default-browser-check", "--user-data-dir=" + str(temporary / "profile"), "--no-pdf-header-footer", "--print-to-pdf=" + str(pdf), (dest / "document.html").as_uri()], dest / "render.log")
            run(["pdftoppm", "-r", "144", "-png", pdf, temporary / "page"], dest / "raster.log")
            images = sorted(temporary.glob("page-*.png"), key=lambda p: int(p.stem.split("-")[-1]))
            if not images:
                raise ValueError(f"No pages rendered for {name}")
            sizes = []
            for n, image in enumerate(images, 1):
                width, height = struct.unpack(">II", image.read_bytes()[16:24])
                sizes.append({"width": width, "height": height})
                image.replace(dest / f"page-{n}.png")
            pdf.replace(dest / "document.pdf")
        write_json(dest / "info.json", {"language": "english", "pages": sizes})
        timing = read_json(dest / "timing.json") or {}
        timing["render_and_raster_seconds"] = time.perf_counter() - start
        write_json(dest / "timing.json", timing)
        remember(dest, "render", inputs, [dest / "document.pdf", dest / "info.json", *page_images(dest)])
        print(f"Rendered {name}: {len(sizes)} pages at 144 dpi", flush=True)


def utility(name):
    binary = ROOT / "bin" / name
    if not binary.exists():
        binary.parent.mkdir(exist_ok=True)
        subprocess.run(["go", "build", "-o", str(binary), "./cmd/" + name], cwd=ROOT, check=True)
    return binary


def ocr(out, jobs):
    for name in LAYOUTS:
        dest = out / name
        images = page_images(dest)
        timing = read_json(dest / "timing.json") or {}
        pdf_inputs = {"pdf_sha256": digest(dest / "document.pdf"), "backend": "poppler", "pipeline": 1}
        if cached(dest, "pdftext", pdf_inputs):
            print(f"{name}: PDF text blocks cached", flush=True)
        else:
            start = time.perf_counter()
            run([utility("pdftext-util"), "-backend", "poppler", dest / "document.pdf"], dest / "pdftext.log")
            paths = []
            for n in range(1, len(images) + 1):
                target = dest / f"pdftext-{n}.json"
                produced = dest / ("document-pdftext.json" if len(images) == 1 else f"document-{n}-pdftext.json")
                produced.replace(target)
                paths.append(target)
            timing["pdftext_seconds"] = time.perf_counter() - start
            remember(dest, "pdftext", pdf_inputs, paths)
            write_json(dest / "timing.json", timing)
        # Engines share a temporary output filename, so finish one engine
        # before starting the next. Cache each successful page independently.
        for engine, binary in [("apple", "ocr-util"), ("tesseract", "tesseract-util")]:
            state_path = dest / ("." + engine + "-state.json")
            state = read_json(state_path) or {}
            pending = []
            for image in images:
                n = int(image.stem.split("-")[-1])
                target = dest / f"{engine}-{n}.json"
                inputs = {"image_sha256": digest(image), "slice": False, "pipeline": 1}
                saved = state.get(str(n), {})
                if saved.get("inputs") != inputs or not outputs_match(dest, saved.get("outputs")):
                    pending.append((image, n, target, inputs))
            if not pending:
                print(f"{name}: {engine} all pages cached; preserving OCR timings", flush=True)
                continue
            executable = utility(binary)
            start = time.perf_counter()
            def recognize(item):
                image, n, target, inputs = item
                env = dict(os.environ, OMP_THREAD_LIMIT="1")
                seconds = run([executable, "-slice=false", image], dest / f"{engine}-{n}.log", env)
                temporary = dest / (image.stem + "-ocr.json")
                if read_json(temporary) is None:
                    raise ValueError(f"Invalid OCR JSON: {temporary}")
                temporary.replace(target)
                return n, seconds, {"inputs": inputs, "outputs": {target.name: digest(target)}}
            durations = []
            with concurrent.futures.ThreadPoolExecutor(max_workers=jobs) as pool:
                futures = [pool.submit(recognize, item) for item in pending]
                for done, future in enumerate(concurrent.futures.as_completed(futures), 1):
                    n, seconds, saved = future.result()
                    durations.append(seconds)
                    state[str(n)] = saved
                    write_json(state_path, state)
                    if done % 20 == 0 or done == len(pending):
                        print(f"{name}: {engine} {done}/{len(pending)} uncached pages", flush=True)
            timing[engine + "_ocr_wall_seconds"] = time.perf_counter() - start
            timing[engine + "_ocr_worker_seconds"] = sum(durations)
            timing[engine + "_ocr_pages_processed"] = len(durations)
            timing["workers"] = jobs
            write_json(dest / "timing.json", timing)


def evaluate(out):
    # Always run the accuracy gates against the current matcher. Expensive
    # source preparation, OCR and overlays are cached separately.
    env = dict(os.environ, BOOK_CORPUS=str(out), BOOK_LABEL="current", BOOK_OVERLAYS="0")
    env.pop("UPDATE_BASELINES", None)
    subprocess.run(["go", "test", "./integration", "-run", "^TestBookCorpus$", "-count=1", "-v"], cwd=ROOT, env=env, check=True)


def overlay_inputs(dest, source):
    code = sorted((ROOT / "internal/bookoverlay").glob("*.go")) + [ROOT / "cmd/book-overlays/main.go"]
    return {"sorted_sha256": digest(dest / "current" / (source + "-sorted.json")),
            "canonical_sha256": digest(dest / "canonical.txt"),
            "images": {p.name: digest(p) for p in page_images(dest)},
            "renderer": {str(p.relative_to(ROOT)): digest(p) for p in code if not p.name.endswith("_test.go")},
            "seed": 117, "sentences": 2}


def overlay_files(dest, source):
    base = dest / "current" / (source + "-overlays")
    count = len(page_images(dest))
    return [base / "manifest.json", base / "README.md", *[
        base / mode / f"page-{n:03d}.png" for mode in ("order", "spotcheck") for n in range(1, count + 1)]]


def overlays(out):
    for name in LAYOUTS:
        dest = out / name
        for source in SOURCES:
            inputs = overlay_inputs(dest, source)
            stage = source + "-overlays"
            if cached(dest, stage, inputs):
                print(f"{name}: {source} overlays cached", flush=True)
                continue
            # Go's build cache keeps the renderer current when its source changes.
            subprocess.run(["go", "run", "./cmd/book-overlays", "-corpus", str(out), "-layouts", name, "-sources", source], cwd=ROOT, check=True)
            remember(dest, stage, inputs, overlay_files(dest, source))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", type=Path, default=ROOT / "experiments/meditations")
    parser.add_argument("--stage", choices=["prepare", "render", "ocr", "evaluate", "overlays", "all"], default="all")
    parser.add_argument("--jobs", type=int, default=4)
    parser.add_argument("--chrome", default="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
    parser.add_argument("--skip-overlays", action="store_true", help="Omit large diagnostic PNGs from --stage all")
    args = parser.parse_args()
    if args.jobs < 1:
        parser.error("--jobs must be at least 1")
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=True)
    if args.stage in {"prepare", "all"}: prepare(out)
    if args.stage in {"render", "all"}: render(out, args.chrome)
    if args.stage in {"ocr", "all"}: ocr(out, args.jobs)
    if args.stage in {"evaluate", "all"}: evaluate(out)
    if args.stage == "overlays" or (args.stage == "all" and not args.skip_overlays): overlays(out)

if __name__ == "__main__":
    main()
