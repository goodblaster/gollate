#!/usr/bin/env python3
"""
Generate PDF documents from canonical text files for OCR testing.
Uses simple single-column layouts with appropriate fonts for each language.
"""

import os
import sys
from pathlib import Path

try:
    from reportlab.lib.pagesizes import letter, A4
    from reportlab.lib.units import inch
    from reportlab.pdfgen import canvas
    from reportlab.pdfbase import pdfmetrics
    from reportlab.pdfbase.ttfonts import TTFont
    from reportlab.lib.colors import black
    from reportlab.pdfbase.cidfonts import UnicodeCIDFont
except ImportError:
    print("ERROR: reportlab not installed. Install with:")
    print("  pip install reportlab")
    sys.exit(1)

# Language configurations - using single column for simplicity
LANGUAGE_CONFIGS = {
    'english': {
        'font': 'Helvetica',
        'font_size': 11,
        'title_size': 14,
        'line_spacing': 1.3,
    },
    'spanish': {
        'font': 'Helvetica',
        'font_size': 11,
        'title_size': 14,
        'line_spacing': 1.3,
    },
    'french': {
        'font': 'Helvetica',
        'font_size': 11,
        'title_size': 14,
        'line_spacing': 1.3,
    },
    'chinese': {
        'font': 'Helvetica',  # Will try to register CID font
        'font_size': 11,
        'title_size': 14,
        'line_spacing': 1.4,
        'use_cid': True,
        'cid_font': 'STSong-Light',  # Standard CJK font
    },
    'japanese': {
        'font': 'Helvetica',
        'font_size': 11,
        'title_size': 14,
        'line_spacing': 1.4,
        'use_cid': True,
        'cid_font': 'HeiseiMin-W3',  # Standard Japanese font
    },
    'arabic': {
        'font': 'Helvetica',
        'font_size': 12,
        'title_size': 16,
        'line_spacing': 1.5,
        'use_cid': True,
        'cid_font': 'STSong-Light',  # Fallback
    },
    'hindi': {
        'font': 'Helvetica',
        'font_size': 11,
        'title_size': 14,
        'line_spacing': 1.4,
        'use_cid': True,
        'cid_font': 'STSong-Light',  # Fallback
    },
}

def wrap_text(text, max_width, canvas_obj, font_name, font_size):
    """Wrap text to fit within max_width."""
    words = text.split()
    lines = []
    current_line = []

    for word in words:
        test_line = ' '.join(current_line + [word])
        try:
            width = canvas_obj.stringWidth(test_line, font_name, font_size)
        except:
            # If stringWidth fails, estimate
            width = len(test_line) * font_size * 0.5

        if width <= max_width:
            current_line.append(word)
        else:
            if current_line:
                lines.append(' '.join(current_line))
            current_line = [word]

    if current_line:
        lines.append(' '.join(current_line))

    return lines

def generate_pdf(input_file, output_file, lang_config):
    """Generate a PDF from a text file."""

    # Read the text file
    with open(input_file, 'r', encoding='utf-8') as f:
        content = f.read()

    # Split into paragraphs
    paragraphs = [p.strip() for p in content.split('\n\n') if p.strip()]

    # Create PDF
    c = canvas.Canvas(output_file, pagesize=letter)
    width, height = letter

    # Try to register CID font if needed
    font_name = lang_config['font']
    if lang_config.get('use_cid', False):
        try:
            # Try to register the CID font
            cid_font = lang_config['cid_font']
            pdfmetrics.registerFont(UnicodeCIDFont(cid_font))
            font_name = cid_font
        except Exception as e:
            print(f"Warning: Could not register {lang_config['cid_font']}: {e}")
            print(f"Falling back to Helvetica (may show boxes for non-Latin characters)")

    # Set up margins - simple single column
    margin_left = 0.75 * inch
    margin_right = 0.75 * inch
    margin_top = 0.75 * inch
    margin_bottom = 0.75 * inch

    usable_width = width - margin_left - margin_right

    # Starting position
    y = height - margin_top
    x = margin_left

    font_size = lang_config['font_size']
    title_size = lang_config['title_size']
    line_spacing = lang_config['line_spacing']

    is_first = True

    for para in paragraphs:
        # Check if this is a title (first paragraph or short paragraph)
        is_title = is_first or len(para) < 100
        is_first = False

        if is_title:
            try:
                c.setFont(font_name + '-Bold', title_size)
            except:
                c.setFont(font_name, title_size)
            size = title_size
        else:
            c.setFont(font_name, font_size)
            size = font_size

        # Wrap text
        try:
            lines = wrap_text(para, usable_width, c, font_name, size)
        except:
            # Simple fallback - split by character count
            lines = [para[i:i+80] for i in range(0, len(para), 80)]

        for line in lines:
            # Check if we need a new page
            if y < margin_bottom + size:
                c.showPage()
                y = height - margin_top
                # Re-set font after page break
                if is_title:
                    try:
                        c.setFont(font_name + '-Bold', size)
                    except:
                        c.setFont(font_name, size)
                else:
                    c.setFont(font_name, size)

            # Draw the line
            try:
                c.drawString(x, y, line)
            except Exception as e:
                print(f"Warning: Could not draw line '{line[:50]}...': {e}")
            y -= size * line_spacing

        # Add extra space after paragraph
        y -= size * 0.5

    c.save()
    print(f"Generated: {output_file}")

def main():
    script_dir = Path(__file__).parent

    # Find all canonical text files
    files = [
        ('english-newspaper-canonical.txt', 'english'),
        ('spanish-newspaper-canonical.txt', 'spanish'),
        ('french-article-canonical.txt', 'french'),
        ('chinese-article-canonical.txt', 'chinese'),
        ('japanese-article-canonical.txt', 'japanese'),
        ('arabic-newspaper-canonical.txt', 'arabic'),
        ('hindi-article-canonical.txt', 'hindi'),
    ]

    for filename, lang in files:
        input_path = script_dir / filename
        if not input_path.exists():
            print(f"Skipping {filename} - file not found")
            continue

        output_path = script_dir / filename.replace('-canonical.txt', '.pdf')
        config = LANGUAGE_CONFIGS.get(lang, LANGUAGE_CONFIGS['english'])

        try:
            generate_pdf(str(input_path), str(output_path), config)
        except Exception as e:
            print(f"Error generating PDF for {filename}: {e}")
            import traceback
            traceback.print_exc()

    print("\nPDF generation complete!")
    print("\nNote: CJK and Indic scripts may not render correctly due to font limitations.")
    print("For best results with these languages, consider using a dedicated layout tool")
    print("or install system fonts and modify the script to use TTF fonts.")

if __name__ == '__main__':
    main()
