const fs = require('fs');
const path = require('path');

// Create a simple PNG file with blue background and white diamond
// This is a minimal PNG file structure
const pngHeader = Buffer.from([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]);
const ihdrChunk = Buffer.from([
    0x00, 0x00, 0x00, 0x0D, // Length
    0x49, 0x48, 0x44, 0x52, // Chunk type: IHDR
    0x00, 0x00, 0x00, 0x80, // Width: 128
    0x00, 0x00, 0x00, 0x80, // Height: 128
    0x08, // Bit depth: 8
    0x06, // Color type: RGB with alpha
    0x00, // Compression method: deflate
    0x00, // Filter method: adaptive
    0x00, // Interlace method: none
    0x00, 0x00, 0x00, 0x00 // CRC (placeholder)
]);

// Create image data (128x128 pixels)
const imageData = Buffer.alloc(128 * 128 * 4); // 4 bytes per pixel (RGBA)

for (let y = 0; y < 128; y++) {
    for (let x = 0; x < 128; x++) {
        const index = (y * 128 + x) * 4;
        
        // Check if pixel is inside the diamond
        const centerX = 64;
        const centerY = 64;
        const distanceX = Math.abs(x - centerX);
        const distanceY = Math.abs(y - centerY);
        
        if (distanceX + distanceY <= 64) {
            // Inside the outer diamond
            if (distanceX + distanceY <= 42) {
                // Inside the inner diamond - blue
                imageData[index] = 51;   // R
                imageData[index + 1] = 103; // G
                imageData[index + 2] = 145; // B
                imageData[index + 3] = 255; // A
            } else {
                // Outside the inner diamond - white
                imageData[index] = 255;   // R
                imageData[index + 1] = 255; // G
                imageData[index + 2] = 255; // B
                imageData[index + 3] = 255; // A
            }
        } else {
            // Outside the outer diamond - blue
            imageData[index] = 51;   // R
            imageData[index + 1] = 103; // G
            imageData[index + 2] = 145; // B
            imageData[index + 3] = 255; // A
        }
    }
}

// Create IDAT chunk (simplified, no compression)
const idatChunk = Buffer.concat([
    Buffer.from([0x00, 0x00, 0x08, 0x00]), // Length
    Buffer.from([0x49, 0x44, 0x41, 0x54]), // Chunk type: IDAT
    imageData,
    Buffer.from([0x00, 0x00, 0x00, 0x00]) // CRC (placeholder)
]);

// Create IEND chunk
const iendChunk = Buffer.from([
    0x00, 0x00, 0x00, 0x00, // Length
    0x49, 0x45, 0x4E, 0x44, // Chunk type: IEND
    0xAE, 0x42, 0x60, 0x82  // CRC
]);

// Combine all chunks
const pngData = Buffer.concat([pngHeader, ihdrChunk, idatChunk, iendChunk]);

// Write to file
const outputPath = path.join(__dirname, 'images', 'postgresql.png');
fs.writeFileSync(outputPath, pngData);
console.log('PNG icon created successfully at:', outputPath);