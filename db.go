package main

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"sync"
)

type TileDB struct {
	mutex *sync.Mutex
	store map[string][3]float64
}

func (db *TileDB) nearest(target [3]float64) string {
	var filename string
	db.mutex.Lock()
	smallest := 1000000.0
	for k, v := range db.store {
		dist := distance(target, v)
		if dist < smallest {
			filename, smallest = k, dist
		}
	}
	delete(db.store, filename)
	db.mutex.Unlock()
	return filename
}

var TILES map[string][3]float64

func loadTiles() map[string][3]float64 {
	fmt.Println("Loading tiles...")
	tiles := make(map[string][3]float64)
	files, _ := ioutil.ReadDir("tiles")
	for _, f := range files {
		name := filepath.Join("tiles", f.Name())
		file, err := os.Open(name)
		if err == nil {
			img, _, err := image.Decode(file)
			if err == nil {
				tiles[name] = averageColor(img)
			} else {
				fmt.Println("failed to decode a tile file:", name, err)
			}
		} else {
			fmt.Println("failed to open a tile file:", name, err)
		}
		file.Close()
	}
	fmt.Println("Loaded tiles!")
	return tiles
}

func averageColor(img image.Image) [3]float64 {
	bounds := img.Bounds()
	r, g, b := 0.0, 0.0, 0.0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			ir, ig, ib, _ := img.At(x, y).RGBA()
			r, g, b = r+float64(ir), g+float64(ig), b+float64(ib)
		}
	}
	pixelCount := float64(bounds.Max.X * bounds.Max.Y)
	return [3]float64{r / pixelCount, g / pixelCount, b / pixelCount}
}

func cloneTileDB() TileDB {
	tiles := make(map[string][3]float64)
	for k, v := range TILES {
		tiles[k] = v
	}
	return TileDB{
		mutex: &sync.Mutex{},
		store: tiles,
	}
}

func resize(in image.Image, newWidth int) image.NRGBA {
	bounds := in.Bounds()
	width := bounds.Dx()
	raito := width / newWidth
	out := image.NewNRGBA(image.Rect(bounds.Min.X/raito, bounds.Min.Y/raito, bounds.Max.X/raito, bounds.Max.Y/raito))
	for y, j := bounds.Min.Y, bounds.Min.Y; y < bounds.Max.Y; y, j = y+raito, j+1 {
		for x, i := bounds.Min.X, bounds.Min.X; x < bounds.Max.X; x, i = x+raito, i+1 {
			r, g, b, a := in.At(x, y).RGBA()
			out.SetNRGBA(i, j, color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)})
		}
	}
	return *out
}

func distance(p1 [3]float64, p2 [3]float64) float64 {
	return math.Sqrt(sq(p2[0]-p1[0]) + sq(p2[1]-p1[1]) + sq(p2[2]-p1[2]))
}

func sq(n float64) float64 {
	return n * n
}

