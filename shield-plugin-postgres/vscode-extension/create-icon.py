from PIL import Image, ImageDraw

# Create a new image with blue background
width, height = 128, 128
image = Image.new('RGB', (width, height), '#336791')
draw = ImageDraw.Draw(image)

# Draw the white diamond
diamond_points = [
    (width//2, height//4),      # Top
    (3*width//4, height//2),    # Right
    (width//2, 3*height//4),    # Bottom
    (width//4, height//2)       # Left
]
draw.polygon(diamond_points, fill='white')

# Draw the blue inner diamond
small_diamond_points = [
    (width//2, height//3),      # Top
    (2*width//3, height//2),    # Right
    (width//2, 2*height//3),    # Bottom
    (width//3, height//2)       # Left
]
draw.polygon(small_diamond_points, fill='#336791')

# Save the image
image.save('/Users/f1/Documents/fengyi/shield-plugins/shield-postgresql/images/postgresql.png')
print('PNG icon created successfully!')