#!/usr/bin/env python3
"""Process the hound image into optimized app icon"""

from PIL import Image, ImageEnhance, ImageFilter
import os

def process_icon_for_size(img, size):
    """Process and optimize icon for specific size"""
    # Resize with high-quality resampling
    resized = img.resize((size, size), Image.Resampling.LANCZOS)

    # For smaller sizes, enhance sharpness slightly
    if size <= 64:
        enhancer = ImageEnhance.Sharpness(resized)
        resized = enhancer.enhance(1.3)

    # For very small sizes, increase contrast slightly to make red eyes pop
    if size <= 32:
        enhancer = ImageEnhance.Contrast(resized)
        resized = enhancer.enhance(1.15)

    return resized

def create_preview_grid(base_img):
    """Create a preview showing icon at multiple sizes"""
    # Create white background
    preview_width = 800
    preview_height = 600
    preview = Image.new('RGB', (preview_width, preview_height), (240, 240, 240))

    # Show icon at different sizes
    sizes_to_show = [
        (512, 50, 50, "512x512\n(DMG/Finder)"),
        (256, 600, 50, "256x256\n(Finder)"),
        (128, 50, 300, "128x128\n(Dock)"),
        (64, 220, 300, "64x64\n(Sidebar)"),
        (32, 350, 300, "32x32\n(List)"),
        (16, 450, 300, "16x16\n(Small)"),
    ]

    from PIL import ImageDraw, ImageFont
    draw = ImageDraw.Draw(preview)

    try:
        # Try to use a nice system font
        font = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 14)
        small_font = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 11)
    except:
        font = ImageFont.load_default()
        small_font = font

    for size, x, y, label in sizes_to_show:
        # Process icon at this size
        icon = process_icon_for_size(base_img, size)

        # Paste onto preview
        preview.paste(icon, (x, y), icon if icon.mode == 'RGBA' else None)

        # Draw border around icon
        draw.rectangle([x-1, y-1, x+size, y+size], outline=(100, 100, 100), width=1)

        # Draw label
        label_y = y + size + 5
        draw.text((x, label_y), label, fill=(60, 60, 60), font=small_font)

    # Add title
    title = "Hound App Icon Preview"
    draw.text((20, 15), title, fill=(40, 40, 40), font=font)

    return preview

def main():
    # Paths
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)
    source_img = os.path.expanduser("~/Downloads/generated-image.png")

    print("🎨 Processing Hound icon from source image...")

    # Load source image
    img = Image.open(source_img)
    print(f"   Source: {img.size[0]}x{img.size[1]}px")

    # Ensure it's square and has the right size
    if img.size != (1024, 1024):
        print(f"   Resizing to 1024x1024...")
        img = img.resize((1024, 1024), Image.Resampling.LANCZOS)

    # Create preview grid
    print("   Creating preview grid...")
    preview = create_preview_grid(img)
    preview_path = os.path.join(project_root, "resources", "icon_preview.png")
    preview.save(preview_path, "PNG")
    print(f"   ✓ Preview saved: {preview_path}")

    # Create iconset directory
    iconset_path = os.path.join(project_root, "AppIcon.iconset")
    os.makedirs(iconset_path, exist_ok=True)

    # Standard macOS icon sizes
    sizes = [
        (16, "16x16"),
        (32, "16x16@2x"),
        (32, "32x32"),
        (64, "32x32@2x"),
        (128, "128x128"),
        (256, "128x128@2x"),
        (256, "256x256"),
        (512, "256x256@2x"),
        (512, "512x512"),
        (1024, "512x512@2x"),
    ]

    print("\n   Generating icon sizes:")
    for size, name in sizes:
        processed = process_icon_for_size(img, size)
        filename = f"icon_{name}.png"
        filepath = os.path.join(iconset_path, filename)
        processed.save(filepath, "PNG")
        print(f"     ✓ {name}")

    print(f"\n✅ Iconset ready at: {iconset_path}")
    print(f"📷 Preview: {preview_path}")
    print("\n   To convert to .icns: iconutil -c icns AppIcon.iconset")
    print(f"   To view preview: open {preview_path}")

if __name__ == "__main__":
    main()
