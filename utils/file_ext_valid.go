package utils

import (
	"os"
	"fmt"
	"bytes"
	"encoding/hex"
	"path"
	"strings"
)

var (
	exts = map[string][]byte{
		"avi": {0x52, 0x49, 0x46, 0x46},
		"mp4": {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
		"webm": {0x1a, 0x45, 0xdf, 0xa3, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x1f, 0x42, 0x86,
			0x81, 0x01, 0x42, 0xf7, 0x81, 0x01, 0x42, 0xf2, 0x81, 0x04, 0x42, 0xf3, 0x81, 0x08, 0x42,
			0x82, 0x84, 0x77, 0x65, 0x62, 0x6d, 0x42, 0x87, 0x81, 0x04, 0x42, 0x85, 0x81, 0x02, 0x18,
			0x53, 0x80, 0x67, 0x01, 0x00, 0x00, 0x00, 0x00},

		"bmp":  {0x42, 0x4d},
		"gif":  {0x47, 0x49, 0x46, 0x38, 0x39, 0x61},
		"jpeg": {0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01},
		"jpg":  {0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01, 0x01},
		"webp": {0x52, 0x49, 0x46, 0x46, 0x92, 0xa2, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50, 0x56, 0x50,
			0x38, 0x20},
	}
)

func ExtValid(filePath string) {
	ext := strings.ToLower(path.Ext(filePath)[1:])
	fd, _ := os.Open(filePath)
	if val, ok := exts[ext]; ok {
		header := make([]byte, len(val))
		fd.Read(header)
		fmt.Println(hex.EncodeToString(header))
		fmt.Println(bytes.Compare(header, exts[ext]))
	} else {
		header := make([]byte, 128)
		fd.Read(header)
		fmt.Println(hex.EncodeToString(header))
	}

}
