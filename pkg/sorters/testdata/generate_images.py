#!/usr/bin/env python3
"""
Generate JPEG images from canonical text files using PIL with proper Unicode fonts.
This works better than PDF for CJK and Indic scripts.
"""

import sys
from pathlib import Path

try:
    from PIL import Image, ImageDraw, ImageFont
except ImportError:
    print("ERROR: Pillow not installed. Install with:")
    print("  pip install Pillow")
    sys.exit(1)

# Image dimensions (letter size at 150 DPI)
WIDTH = 1275
HEIGHT = 1650

# Language configurations
LANGUAGE_CONFIGS = {
    'english': {
        'font_name': 'Helvetica',
        'font_size': 16,
        'title_size': 22,
        'line_spacing': 1.3,
    },
    'spanish': {
        'font_name': 'Helvetica',
        'font_size': 16,
        'title_size': 22,
        'line_spacing': 1.3,
    },
    'french': {
        'font_name': 'Helvetica',
        'font_size': 16,
        'title_size': 22,
        'line_spacing': 1.3,
    },
    'chinese': {
        'font_name': 'Arial Unicode MS',  # Fallback to default
        'font_size': 16,
        'title_size': 22,
        'line_spacing': 1.5,
    },
    'japanese': {
        'font_name': 'Arial Unicode MS',
        'font_size': 16,
        'title_size': 22,
        'line_spacing': 1.5,
    },
    'arabic': {
        'font_name': 'Arial Unicode MS',
        'font_size': 18,
        'title_size': 24,
        'line_spacing': 1.6,
    },
    'hindi': {
        'font_name': 'Arial Unicode MS',
        'font_size': 16,
        'title_size': 22,
        'line_spacing': 1.5,
    },
}

def get_system_font(preferred_name, size):
    """Try to get a system font, falling back to PIL default if needed."""
    # Common font paths on macOS
    font_paths = [
        f"/System/Library/Fonts/{preferred_name}.ttc",
        f"/System/Library/Fonts/{preferred_name}.ttf",
        f"/Library/Fonts/{preferred_name}.ttf",
        "/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
        "/System/Library/Fonts/Helvetica.ttc",
    ]

    for font_path in font_paths:
        try:
            return ImageFont.truetype(font_path, size)
        except:
            continue

    # Fallback to PIL default font
    print(f"Warning: Could not load {preferred_name}, using default font")
    return ImageFont.load_default()

def is_cjk_char(char):
    """Check if a character is CJK (Chinese, Japanese, Korean)."""
    code = ord(char)
    return (0x4E00 <= code <= 0x9FFF or    # CJK Unified Ideographs
            0x3400 <= code <= 0x4DBF or    # CJK Extension A
            0x20000 <= code <= 0x2A6DF or  # CJK Extension B
            0x3040 <= code <= 0x309F or    # Hiragana
            0x30A0 <= code <= 0x30FF)      # Katakana

def wrap_text(text, max_width, font, draw):
    """Wrap text to fit within max_width."""
    # Check if text contains CJK characters
    has_cjk = any(is_cjk_char(c) for c in text[:100])  # Check first 100 chars

    if has_cjk:
        # For CJK text, wrap by characters
        lines = []
        current_line = ""

        for char in text:
            test_line = current_line + char
            bbox = draw.textbbox((0, 0), test_line, font=font)
            width = bbox[2] - bbox[0]

            if width <= max_width:
                current_line += char
            else:
                if current_line:
                    lines.append(current_line)
                current_line = char

        if current_line:
            lines.append(current_line)

        return lines
    else:
        # For space-separated text, wrap by words
        words = text.split()
        lines = []
        current_line = []

        for word in words:
            test_line = ' '.join(current_line + [word])
            bbox = draw.textbbox((0, 0), test_line, font=font)
            width = bbox[2] - bbox[0]

            if width <= max_width:
                current_line.append(word)
            else:
                if current_line:
                    lines.append(' '.join(current_line))
                current_line = [word]

        if current_line:
            lines.append(' '.join(current_line))

        return lines

def generate_image(input_file, output_file, lang_config):
    """Generate a JPEG image from a text file."""

    # Read the text file
    with open(input_file, 'r', encoding='utf-8') as f:
        content = f.read()

    # Split into paragraphs
    paragraphs = [p.strip() for p in content.split('\n\n') if p.strip()]

    # Create image
    img = Image.new('RGB', (WIDTH, HEIGHT), color='white')
    draw = ImageDraw.Draw(img)

    # Set up margins
    margin_left = 100
    margin_right = 100
    margin_top = 100
    margin_bottom = 100

    usable_width = WIDTH - margin_left - margin_right

    # Load fonts
    font_size = lang_config['font_size']
    title_size = lang_config['title_size']
    line_spacing = lang_config['line_spacing']

    regular_font = get_system_font(lang_config['font_name'], font_size)
    title_font = get_system_font(lang_config['font_name'], title_size)

    # Starting position
    y = margin_top
    x = margin_left

    is_first = True

    for para in paragraphs:
        # Check if this is a title (first paragraph or short paragraph)
        is_title = is_first or len(para) < 100
        is_first = False

        font = title_font if is_title else regular_font
        size = title_size if is_title else font_size

        # Wrap text
        lines = wrap_text(para, usable_width, font, draw)

        for line in lines:
            # Check if we've run out of space
            if y > HEIGHT - margin_bottom:
                break

            # Draw the line
            draw.text((x, y), line, fill='black', font=font)

            # Get actual line height
            bbox = draw.textbbox((0, 0), line, font=font)
            line_height = bbox[3] - bbox[1]

            y += int(line_height * line_spacing)

        # Add extra space after paragraph
        y += int(size * 0.5)

        if y > HEIGHT - margin_bottom:
            break

    # Save image
    img.save(output_file, 'JPEG', quality=95)
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

        output_path = script_dir / filename.replace('-canonical.txt', '.jpg')
        config = LANGUAGE_CONFIGS.get(lang, LANGUAGE_CONFIGS['english'])

        try:
            generate_image(str(input_path), str(output_path), config)
        except Exception as e:
            print(f"Error generating image for {filename}: {e}")
            import traceback
            traceback.print_exc()

    print("\nImage generation complete!")

if __name__ == '__main__':
    main()
