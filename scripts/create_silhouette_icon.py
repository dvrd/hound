#!/usr/bin/env python3
"""Create silhouette version of hound icon - black dog with red eyes"""

from PIL import Image, ImageDraw, ImageEnhance, ImageFilter
import os

def create_silhouette_icon(source_img, size=1024):
    """Convert image to silhouette with red eyes on transparent background"""

    # Resize to working size
    img = source_img.resize((size, size), Image.Resampling.LANCZOS)

    # Convert to RGBA if not already
    if img.mode != 'RGBA':
        img = img.convert('RGBA')

    # Create new image with transparent background
    silhouette = Image.new('RGBA', (size, size), (0, 0, 0, 0))
    pixels = img.load()
    sil_pixels = silhouette.load()

    # First pass: identify the dog area (non-background)
    # The background is the blue/purple gradient, dog is darker
    for y in range(size):
        for x in range(size):
            r, g, b, a = pixels[x, y]

            # Calculate brightness
            brightness = (r + g + b) / 3

            # If pixel is part of the dog (darker areas or has alpha)
            # Background is bright blue/purple (high brightness)
            # Dog is black/dark (low brightness)
            if brightness < 180 or a < 255:  # Dark areas or edges
                # Check if this is a red eye area (very high red, very low green/blue)
                # Be VERY strict - only the brightest red pixels should be red
                is_red_eye = r > 180 and r > g * 2.5 and r > b * 2.5 and g < 80 and b < 80

                if is_red_eye:
                    # Keep bright red for eyes only
                    intensity = min(255, int(r * 1.3))
                    sil_pixels[x, y] = (intensity, 0, 0, 255)  # Pure red, no green/blue
                else:
                    # Everything else is pure black silhouette
                    sil_pixels[x, y] = (0, 0, 0, 255)

    # Apply edge detection to clean up the silhouette
    # Convert to grayscale for edge detection
    gray = silhouette.convert('L')

    # Create mask from non-transparent areas
    mask = Image.new('L', (size, size), 0)
    for y in range(size):
        for x in range(size):
            if sil_pixels[x, y][3] > 0:
                mask.putpixel((x, y), 255)

    # Clean up and smooth edges
    mask = mask.filter(ImageFilter.GaussianBlur(radius=2))

    # Apply threshold to make crisp edges
    mask_pixels = mask.load()
    for y in range(size):
        for x in range(size):
            val = mask_pixels[x, y]
            mask_pixels[x, y] = 255 if val > 128 else 0

    # Apply mask to silhouette
    silhouette.putalpha(mask)

    # Enhance the red eyes with glow effect
    enhanced = Image.new('RGBA', (size, size), (0, 0, 0, 0))
    enhanced_pixels = enhanced.load()
    sil_pixels = silhouette.load()

    for y in range(size):
        for x in range(size):
            r, g, b, a = sil_pixels[x, y]

            if a > 0:
                # If it's a red eye pixel (ONLY pure red pixels)
                if r > 150 and g < 10 and b < 10:
                    # Add glow around eyes
                    glow_radius = max(2, size // 200)
                    for dy in range(-glow_radius, glow_radius + 1):
                        for dx in range(-glow_radius, glow_radius + 1):
                            nx, ny = x + dx, y + dy
                            if 0 <= nx < size and 0 <= ny < size:
                                dist = (dx*dx + dy*dy) ** 0.5
                                if dist <= glow_radius:
                                    glow_intensity = int(200 * (1 - dist / glow_radius))
                                    er, eg, eb, ea = enhanced_pixels[nx, ny]
                                    # Pure red glow, no green or blue
                                    enhanced_pixels[nx, ny] = (
                                        max(er, glow_intensity),
                                        max(eg, 0),  # No green in glow
                                        max(eb, 0),  # No blue in glow
                                        max(ea, glow_intensity)
                                    )

                # Set the main pixel
                enhanced_pixels[x, y] = (
                    max(enhanced_pixels[x, y][0], r),
                    max(enhanced_pixels[x, y][1], g),
                    max(enhanced_pixels[x, y][2], b),
                    max(enhanced_pixels[x, y][3], a)
                )

    return enhanced

def process_icon_for_size(img, size):
    """Process and optimize icon for specific size"""
    resized = img.resize((size, size), Image.Resampling.LANCZOS)

    # For smaller sizes, enhance the red eyes more
    if size <= 64:
        pixels = resized.load()
        for y in range(size):
            for x in range(size):
                r, g, b, a = pixels[x, y]
                # Only enhance pure red pixels (eyes)
                if r > 100 and g < 10 and b < 10 and a > 0:
                    # Make eyes more prominent at small sizes
                    pixels[x, y] = (min(255, int(r * 1.3)), 0, 0, a)

    return resized

def create_preview_grid(base_img):
    """Create a preview showing icon at multiple sizes"""
    preview_width = 800
    preview_height = 600

    # Create checkered background to show transparency
    preview = Image.new('RGB', (preview_width, preview_height), (255, 255, 255))
    draw = ImageDraw.Draw(preview)

    # Draw checkerboard pattern
    checker_size = 10
    for y in range(0, preview_height, checker_size):
        for x in range(0, preview_width, checker_size):
            if (x // checker_size + y // checker_size) % 2:
                draw.rectangle([x, y, x + checker_size, y + checker_size], fill=(230, 230, 230))

    sizes_to_show = [
        (512, 50, 50, "512x512\n(DMG/Finder)"),
        (256, 600, 50, "256x256\n(Finder)"),
        (128, 50, 300, "128x128\n(Dock)"),
        (64, 220, 300, "64x64\n(Sidebar)"),
        (32, 350, 300, "32x32\n(List)"),
        (16, 450, 300, "16x16\n(Small)"),
    ]

    try:
        from PIL import ImageFont
        font = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 14)
        small_font = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 11)
    except:
        font = ImageFont.load_default()
        small_font = font

    for size, x, y, label in sizes_to_show:
        icon = process_icon_for_size(base_img, size)
        preview.paste(icon, (x, y), icon)

        # Draw border
        draw.rectangle([x-1, y-1, x+size, y+size], outline=(100, 100, 100), width=1)

        # Label
        label_y = y + size + 5
        draw.text((x, label_y), label, fill=(60, 60, 60), font=small_font)

    # Title
    title = "Hound Silhouette Icon Preview"
    draw.text((20, 15), title, fill=(40, 40, 40), font=font)

    return preview

def main():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)
    source_path = os.path.expanduser("~/Downloads/generated-image.png")

    print("🎨 Creating silhouette icon from source image...")

    # Load source
    source = Image.open(source_path)
    print(f"   Source: {source.size[0]}x{source.size[1]}px")

    # Create silhouette version
    print("   Converting to silhouette (this may take a moment)...")
    silhouette = create_silhouette_icon(source, size=1024)

    # Save high-res version for reference
    silhouette_path = os.path.join(project_root, "resources", "icon_silhouette.png")
    silhouette.save(silhouette_path, "PNG")
    print(f"   ✓ Silhouette saved: {silhouette_path}")

    # Create preview
    print("   Creating preview grid...")
    preview = create_preview_grid(silhouette)
    preview_path = os.path.join(project_root, "resources", "icon_preview_silhouette.png")
    preview.save(preview_path, "PNG")
    print(f"   ✓ Preview saved: {preview_path}")

    # Create iconset
    iconset_path = os.path.join(project_root, "AppIcon.iconset")
    os.makedirs(iconset_path, exist_ok=True)

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
        processed = process_icon_for_size(silhouette, size)
        filename = f"icon_{name}.png"
        filepath = os.path.join(iconset_path, filename)
        processed.save(filepath, "PNG")
        print(f"     ✓ {name}")

    print(f"\n✅ Silhouette iconset ready!")
    print(f"📷 Preview: {preview_path}")
    print(f"\n   To view: open {preview_path}")

if __name__ == "__main__":
    main()
