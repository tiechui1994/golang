package images

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
)

func DrawPNG(srcPath string) {
	const (
		width  = 300
		height = 500
	)

	// 文件
	pngFile, _ := os.Create(srcPath)
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


func ConvertPNG2JPEG(srcPath, dstPath string) (err error) {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return
	}
	defer dstFile.Close()

	srcImage, _ := png.Decode(srcFile)
	dstImage := image.NewRGBA(srcImage.Bounds())
	draw.Draw(dstImage, dstImage.Bounds(), srcImage, srcImage.Bounds().Min, draw.Src)

	jpeg.Encode(dstFile, dstImage, nil)
	return
}
