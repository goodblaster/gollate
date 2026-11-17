"""Hermetic pipeline checks: no downloaded book, browser, OCR, or network needed."""
import contextlib
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import struct
import tempfile
import unittest
from unittest.mock import patch
import zipfile

spec = importlib.util.spec_from_file_location("meditations", Path(__file__).with_name("meditations-corpus.py"))
corpus = importlib.util.module_from_spec(spec)
spec.loader.exec_module(corpus)


def tiny_epub(path):
    """Exercise all spine selections using a tiny, authored book."""
    paths = [f"text/book-{n}.xhtml" for n in range(1, 13)] + ["text/endnotes.xhtml"]
    manifest = "".join(f'<item id="s{n}" href="{p}" />' for n, p in enumerate(paths))
    spine = "".join(f'<itemref idref="s{n}" />' for n in range(len(paths)))
    package = f'''<package xmlns="http://www.idpf.org/2007/opf" xmlns:dc="http://purl.org/dc/elements/1.1/">
      <metadata><dc:title>Tiny Book</dc:title><dc:creator>A. Writer</dc:creator>
      <dc:contributor id="translator">T. Reader</dc:contributor><dc:rights>Authored test data.</dc:rights>
      <dc:source>test fixture</dc:source></metadata>
      <manifest>{manifest}</manifest><spine>{spine}</spine></package>'''
    with zipfile.ZipFile(path, "w") as archive:
        archive.writestr("epub/content.opf", package)
        for n, name in enumerate(paths, 1):
            if name.endswith("endnotes.xhtml"):
                body = '<h2>Notes</h2><ol><li><p>A note.<a epub:type="backlink">↩</a></p></li></ol>'
            else:
                body = f'<h2>Book {n}</h2><p>Café co\u00adoperate <a epub:type="noteref">1</a> today.</p>'
                if n <= 5:
                    body += '<blockquote><p>A quotation.</p><cite>Its author.</cite></blockquote>'
            archive.writestr("epub/" + name, '<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body>' + body + '</body></html>')
    return corpus.digest(path)


class CorpusTest(unittest.TestCase):
    def setUp(self):
        self.folder = tempfile.TemporaryDirectory()
        self.addCleanup(self.folder.cleanup)
        self.root = Path(self.folder.name)
        self.quiet = contextlib.redirect_stdout(io.StringIO())
        self.quiet.__enter__()
        self.addCleanup(self.quiet.__exit__, None, None, None)

    def prepare_fixture(self):
        source = self.root / "source"
        source.mkdir()
        sha = tiny_epub(source / "meditations.epub")
        with patch.object(corpus, "EPUB_SHA256", sha):
            corpus.prepare(self.root)

    def fake_render(self, command, log, env=None):
        command = [str(x) for x in command]
        pdf_arg = next((x for x in command if x.startswith("--print-to-pdf=")), None)
        if pdf_arg:
            Path(pdf_arg.split("=", 1)[1]).write_bytes(b"test PDF")
        elif command[0] == "pdftoppm":
            for n in (1, 2):
                Path(command[-1] + f"-{n}.png").write_bytes(b"\x89PNG\r\n\x1a\n" + b"\0" * 8 + struct.pack(">II", 1008, 1296))
        else:
            raise AssertionError(command)
        return 1.0

    def rendered_fixture(self):
        self.prepare_fixture()
        with patch.object(corpus, "run", side_effect=self.fake_render):
            corpus.render(self.root, "fake-chrome")

    def test_download_verified_atomic_and_cached(self):
        payload = b"test source"
        sha = hashlib.sha256(payload).hexdigest()
        source = self.root / "source"
        with patch.object(corpus, "EPUB_SHA256", sha), patch.object(corpus.urllib.request, "urlopen", return_value=io.BytesIO(payload)) as fetch:
            path = corpus.download(source)
            self.assertEqual(path.read_bytes(), payload)
            corpus.download(source)
            self.assertEqual(fetch.call_count, 1)
            path.write_bytes(b"damaged")
            with self.assertRaisesRegex(ValueError, "checksum"):
                corpus.download(source)
            self.assertEqual(fetch.call_count, 1)
        with patch.object(corpus.urllib.request, "urlopen", return_value=io.BytesIO(b"wrong edition")):
            with self.assertRaisesRegex(ValueError, "checksum"):
                corpus.download(self.root / "failed")
        self.assertFalse((self.root / "failed/meditations.epub").exists())

    def test_complete_spine_and_visible_text(self):
        self.prepare_fixture()
        source = self.root / "source"
        provenance = json.loads((source / "provenance.json").read_text())
        self.assertEqual(len(provenance["selected_spine_documents"]), 13)
        mapping = json.loads((source / "canonical-map.json").read_text())
        self.assertEqual(sum(x["kind"] == "cite" for x in mapping), 5)
        canonical = (source / "canonical.txt").read_text()
        self.assertIn("Café cooperate 1 today.", canonical)
        self.assertIn("1. A note.", canonical)
        self.assertNotIn("↩", canonical)
        for layout in corpus.LAYOUTS:
            dest = self.root / layout
            self.assertEqual((dest / "canonical.txt").read_text(), canonical)
            corpus.validate_rendered_text((dest / "document.html").read_text(), canonical)
        before = (source / "canonical.txt").stat().st_mtime_ns
        with patch.object(corpus, "EPUB_SHA256", corpus.digest(source / "meditations.epub")):
            corpus.prepare(self.root)
        self.assertEqual((source / "canonical.txt").stat().st_mtime_ns, before)

    def test_render_cache_and_missing_or_changed_inputs(self):
        self.prepare_fixture()
        with patch.object(corpus, "run", side_effect=self.fake_render) as run:
            corpus.render(self.root, "fake-chrome")
            self.assertEqual(run.call_count, 4)
            run.reset_mock()
            corpus.render(self.root, "fake-chrome")
            run.assert_not_called()
            (self.root / "single-column/page-1.png").unlink()
            corpus.render(self.root, "fake-chrome")
            self.assertEqual(run.call_count, 2)
            run.reset_mock()
            html = self.root / "double-column/document.html"
            html.write_text(html.read_text() + "<!-- changed layout -->")
            corpus.render(self.root, "fake-chrome")
            self.assertEqual(run.call_count, 2)

    def test_ocr_cache_resumes_individual_pages(self):
        self.rendered_fixture()
        def fake_ocr(command, log, env=None):
            binary = Path(command[0]).name
            source = Path(command[-1])
            if binary == "pdftext-util":
                for n in (1, 2):
                    (source.parent / f"document-{n}-pdftext.json").write_text("[]")
            else:
                source.with_name(source.stem + "-ocr.json").write_text("[]")
            return 1.0
        with patch.object(corpus, "utility", side_effect=lambda name: Path(name)), patch.object(corpus, "run", side_effect=fake_ocr) as run:
            corpus.ocr(self.root, 2)
            self.assertEqual(run.call_count, 10)
            timing = (self.root / "single-column/timing.json").read_bytes()
            run.reset_mock()
            corpus.ocr(self.root, 2)
            run.assert_not_called()
            self.assertEqual((self.root / "single-column/timing.json").read_bytes(), timing)
            (self.root / "single-column/apple-2.json").unlink()
            corpus.ocr(self.root, 2)
            self.assertEqual(run.call_count, 1)
            self.assertEqual(json.loads((self.root / "single-column/timing.json").read_text())["apple_ocr_pages_processed"], 1)
            run.reset_mock()
            (self.root / "single-column/page-1.png").write_bytes(b"changed image")
            corpus.ocr(self.root, 2)
            self.assertEqual(run.call_count, 2)

    def test_overlay_cache_tracks_results_and_page_images(self):
        self.rendered_fixture()
        for layout in corpus.LAYOUTS:
            dest = self.root / layout / "current"
            dest.mkdir()
            for source in corpus.SOURCES:
                (dest / (source + "-sorted.json")).write_text("{}")
        def fake_overlay(command, **kwargs):
            dest = self.root / command[command.index("-layouts") + 1]
            source = command[command.index("-sources") + 1]
            for path in corpus.overlay_files(dest, source):
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(b"generated overlay")
        with patch.object(corpus.subprocess, "run", side_effect=fake_overlay) as run:
            corpus.overlays(self.root)
            self.assertEqual(run.call_count, 6)
            run.reset_mock()
            corpus.overlays(self.root)
            run.assert_not_called()
            (self.root / "single-column/current/apple-sorted.json").write_text('{"changed":true}')
            corpus.overlays(self.root)
            self.assertEqual(run.call_count, 1)
            run.reset_mock()
            (self.root / "double-column/page-1.png").write_bytes(b"new image")
            corpus.overlays(self.root)
            self.assertEqual(run.call_count, 3)

    def test_orphan_attribution_cannot_disappear(self):
        document = "<body><p>A quotation.</p><cite>Its author.</cite></body>"
        with self.assertRaises(ValueError):
            corpus.validate_rendered_text(document, "A quotation.")
        corpus.validate_rendered_text(document, "A quotation.\nIts author.")

    def test_evaluation_checks_without_updating_baselines(self):
        with patch.dict(corpus.os.environ, {"UPDATE_BASELINES": "1", "BOOK_OVERLAYS": "1"}), patch.object(corpus.subprocess, "run") as run:
            corpus.evaluate(self.root)
        env = run.call_args.kwargs["env"]
        self.assertNotIn("UPDATE_BASELINES", env)
        self.assertEqual(env["BOOK_OVERLAYS"], "0")
        self.assertEqual(env["BOOK_CORPUS"], str(self.root))

    def test_inline_markup_does_not_invent_spaces(self):
        corpus.validate_rendered_text("<body><p>A<cite>B</cite>, C.</p></body>", "AB, C.")


if __name__ == "__main__":
    unittest.main()
