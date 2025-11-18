#!/usr/bin/env python3
"""Generate Hound app icon - dog silhouette with red eyes"""

from PIL import Image, ImageDraw
import os

def create_dog_icon(size):
    """Create a dog silhouette icon with red eyes at given size"""
    # Create image with transparent background
    img = Image.new('RGBA', (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)

    # Scale factors based on size
    s = size / 512.0

    # Dark gray/black for dog silhouette
    dog_color = (40, 40, 40, 255)
    eye_color = (220, 20, 20, 255)  # Red eyes

    # Draw dog head (simplified sitting hound profile)

    # Main head circle
    head_x = int(256 * s)
    head_y = int(200 * s)
    head_r = int(140 * s)
    draw.ellipse([head_x - head_r, head_y - head_r,
                  head_x + head_r, head_y + head_r], fill=dog_color)

    # Snout/muzzle (elongated oval)
    snout_x = int(380 * s)
    snout_y = int(240 * s)
    snout_w = int(100 * s)
    snout_h = int(60 * s)
    draw.ellipse([snout_x - snout_w, snout_y - snout_h,
                  snout_x + snout_w, snout_y + snout_h], fill=dog_color)

    # Ears (long droopy hound ears)
    # Left ear
    ear_l_points = [
        (int(180 * s), int(150 * s)),  # Top
        (int(140 * s), int(200 * s)),  # Middle out
        (int(160 * s), int(320 * s)),  # Bottom
        (int(200 * s), int(280 * s)),  # Bottom in
    ]
    draw.polygon(ear_l_points, fill=dog_color)

    # Right ear (partially behind head)
    ear_r_points = [
        (int(260 * s), int(140 * s)),  # Top
        (int(280 * s), int(180 * s)),  # Middle
        (int(270 * s), int(260 * s)),  # Bottom
        (int(240 * s), int(240 * s)),  # Bottom in
    ]
    draw.polygon(ear_r_points, fill=dog_color)

    # Body (sitting position)
    body_x = int(240 * s)
    body_y = int(360 * s)
    body_w = int(120 * s)
    body_h = int(100 * s)
    draw.ellipse([body_x - body_w, body_y - body_h,
                  body_x + body_w, body_y + body_h], fill=dog_color)

    # Neck connection
    neck_points = [
        (int(220 * s), int(300 * s)),
        (int(280 * s), int(300 * s)),
        (int(300 * s), int(360 * s)),
        (int(180 * s), int(360 * s)),
    ]
    draw.polygon(neck_points, fill=dog_color)

    # Front legs
    leg_w = int(30 * s)
    # Left leg
    draw.rectangle([int(200 * s), int(400 * s),
                   int(200 * s) + leg_w, int(480 * s)], fill=dog_color)
    # Right leg
    draw.rectangle([int(280 * s), int(400 * s),
                   int(280 * s) + leg_w, int(480 * s)], fill=dog_color)

    # RED EYES (the key feature!)
    eye_size = int(20 * s)
    eye_glow = int(8 * s)

    # Left eye with glow effect
    left_eye_x = int(300 * s)
    left_eye_y = int(200 * s)

    # Glow (outer red, more transparent)
    glow_color = (220, 20, 20, 120)
    draw.ellipse([left_eye_x - eye_glow, left_eye_y - eye_glow,
                  left_eye_x + eye_glow, left_eye_y + eye_glow], fill=glow_color)

    # Bright eye core
    draw.ellipse([left_eye_x - eye_size//2, left_eye_y - eye_size//2,
                  left_eye_x + eye_size//2, left_eye_y + eye_size//2], fill=eye_color)

    # Highlight (makes it look more alive/glowing)
    highlight_size = int(6 * s)
    highlight_x = left_eye_x - int(4 * s)
    highlight_y = left_eye_y - int(4 * s)
    draw.ellipse([highlight_x, highlight_y,
                  highlight_x + highlight_size, highlight_y + highlight_size],
                 fill=(255, 100, 100, 255))

    # Right eye (similar)
    right_eye_x = int(360 * s)
    right_eye_y = int(200 * s)

    # Glow
    draw.ellipse([right_eye_x - eye_glow, right_eye_y - eye_glow,
                  right_eye_x + eye_glow, right_eye_y + eye_glow], fill=glow_color)

    # Bright eye core
    draw.ellipse([right_eye_x - eye_size//2, right_eye_y - eye_size//2,
                  right_eye_x + eye_size//2, right_eye_y + eye_size//2], fill=eye_color)

    # Highlight
    highlight_x = right_eye_x - int(4 * s)
    highlight_y = right_eye_y - int(4 * s)
    draw.ellipse([highlight_x, highlight_y,
                  highlight_x + highlight_size, highlight_y + highlight_size],
                 fill=(255, 100, 100, 255))

    return img

def main():
    script_dir = os.path.dirname(os.path.abspath(__file__))
    project_root = os.path.dirname(script_dir)

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

    print("🎨 Generating Hound icon at multiple resolutions...")

    for size, name in sizes:
        img = create_dog_icon(size)
        filename = f"icon_{name}.png"
        filepath = os.path.join(iconset_path, filename)
        img.save(filepath, "PNG")
        print(f"  ✓ Created {name} ({size}x{size}px)")

    print(f"\n✅ Iconset created at: {iconset_path}")
    print("   Run: iconutil -c icns AppIcon.iconset")

    # Also create a preview PNG
    preview_img = create_dog_icon(512)
    preview_path = os.path.join(project_root, "resources", "icon_preview.png")
    preview_img.save(preview_path, "PNG")
    print(f"   Preview: {preview_path}")

if __name__ == "__main__":
    main()
