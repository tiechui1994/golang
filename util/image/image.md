#### 图片格式转换

go当中生成Image的流程:

```
创建文件 -> 创建Image并进行绘图操作 -> 将绘图后的Image保存到创建的文件当中
```

下面是一个绘图的例子:

```cgo
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
```

图片格式转换:

```cgo
func Png2Jpeg() {
  	// 构建源文件和目标文件
	srcFile, _ := os.Open("image.png")
	defer srcFile.Close()

	destFile, _ := os.Create("image.jpg")
	defer destFile.Close()
	
    // 绘图
	srcImage, _ := png.Decode(srcFile) // 获取源文件的Image
	destImage := image.NewRGBA(srcImage.Bounds()) //根据源文件的大小构建目标文件的Image
	draw.Draw(destImage, destImage.Bounds(), srcImage, srcImage.Bounds().Min, draw.Src) //绘图
  
	// 保存
	jpeg.Encode(destFile, destImage, nil) 
}
```

#### 图片绘图

图片的创建:

```cgo
image包结构:
type Image interface {
    // ColorModel方法返回图像的色彩模型
    ColorModel() color.Model
    // Bounds方法返回图像的范围，范围不一定包括点(0, 0)
    Bounds() Rectangle
    // At方法返回(x, y)位置的色彩
    At(x, y int) color.Color
}

Image -> Gray, Gray16, NGRBA, NGRBA16, RGBA, RGBA64, Alpha, Alpha16, Rectangle, PalettedImage, Uniform

PalettedImage接口: 代表一幅图像, 它的像素可能来自一个有限的调色板.
Rectangle: 代表一个矩形. 该矩形包含满足Min.x <= x < Max.X 且 Min.y <= y < Max.y的点.
	React(x0,y0,x1,y1 int) Rectangle

Uniform: 代表一块面积无限大的具有同一色彩的图像. 实现了color.Color, color.Model和Image接口
	NewUniform(c color.Color) *Uniform

Alpha: 代表一幅内存中的图像, 其At方法返回color.Alpha类型的值.
	NewAlpha(r Rectangle) *Alpha

Gray: 代表一幅内存中的图像, 其At方法返回color.Gray类型的值
	NewGray(r Rectangle) *Gray
	
RGBA: 代表一幅内存中的图像, 其At方法返回color.RGAB类型的值
 	NewRGBA(r Rectangle) *RGBA
 
NRGBA: 代表一幅内存中的图像，其At方法返回color.NRGBA类型的值
	NewNRGBA(r Rectangle) *NRGBA

Paletted: 一幅采用uint8类型索引调色板的内存中的图像.
	NewPaletted(r Rectangle, p color.Palette) *Paletted
```

