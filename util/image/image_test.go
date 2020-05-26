package image

import "testing"

func TestDrawPNG(t *testing.T) {
	DrawPNG("rand.png")
}

func TestPNG2JPEG(t *testing.T) {
	PNG2JPEG("rand.png", "rand.jpeg")
}

func TestJPEG2PNG(t *testing.T) {
	JPEG2PNG("rand.jpeg", "random.png")
}
