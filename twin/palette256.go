package twin

func color256ToRGB(color256 uint8) (r, g, b float64) {
	if color256 < 16 {
		// Standard ANSI colors
		r := float64(standardAnsiColors[color256][0]) / 255.0
		g := float64(standardAnsiColors[color256][1]) / 255.0
		b := float64(standardAnsiColors[color256][2]) / 255.0
		return r, g, b
	}

	if color256 >= 232 {
		// Grayscale. Colors 232-255 map to components 0x08 to 0xee
		gray := float64((color256-232)*0x0a+0x08) / 255.0
		return gray, gray, gray
	}

	// 6x6 color cube
	color0_to_215 := color256 - 16

	components := []uint8{0x00, 0x5f, 0x87, 0xaf, 0xd7, 0xff}
	r = float64(components[(color0_to_215/36)%6]) / 255.0
	g = float64(components[(color0_to_215/6)%6]) / 255.0
	b = float64(components[(color0_to_215/1)%6]) / 255.0
	return r, g, b
}

// Source, the 256 colors table from here:
// https://en.wikipedia.org/wiki/ANSI_escape_code#3-bit_and_4-bit
var standardAnsiColors = [16][3]uint8{
	{0x00, 0x00, 0x00}, // Black
	{0x80, 0x00, 0x00}, // Red
	{0x00, 0x80, 0x00}, // Green
	{0x80, 0x80, 0x00}, // Yellow
	{0x00, 0x00, 0x80}, // Blue
	{0x80, 0x00, 0x80}, // Magenta
	{0x00, 0x80, 0x80}, // Cyan
	{0x80, 0x80, 0x80}, // White

	{0x80, 0x80, 0x80}, // Bright Black
	{0xff, 0x00, 0x00}, // Bright Red
	{0x00, 0xff, 0x00}, // Bright Green
	{0xff, 0xff, 0x00}, // Bright Yellow
	{0x00, 0x00, 0xff}, // Bright Blue
	{0xff, 0x00, 0xff}, // Bright Magenta
	{0x00, 0xff, 0xff}, // Bright Cyan
	{0xff, 0xff, 0xff}, // Bright White
}
