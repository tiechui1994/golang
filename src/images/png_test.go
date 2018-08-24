package images

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
)

func GerneratePng() {
	const (
		width  = 300
		height = 500
	)

	// 文件
	pngFile, _ := os.Create("image.png")
	defer pngFile.Close()

	// Image, 进行绘图操作
	pngImage := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pngImage.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), uint8((x ^ y) % 256), uint8((x ^ y) % 256)})
		}
	}

	// 以png的格式写入文件
	png.Encode(pngFile, pngImage)
}

func Png2Jpeg() {
	srcFile, _ := os.Open("image.png")
	defer srcFile.Close()

	destFile, _ := os.Create("image.jpg")
	defer destFile.Close()

	srcImage, _ := png.Decode(srcFile)
	destImage := image.NewRGBA(srcImage.Bounds())
	draw.Draw(destImage, destImage.Bounds(), srcImage, srcImage.Bounds().Min, draw.Src)

	jpeg.Encode(destFile, destImage, nil)
}

func TestPng(t *testing.T) {
	GerneratePng()
	Png2Jpeg()
}
