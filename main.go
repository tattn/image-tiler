package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/draw"
	"image/jpeg"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

func main() {
	fmt.Println("Launching...")
	mux := http.NewServeMux()
	files := http.FileServer(http.Dir("public"))
	mux.Handle("/static/", http.StripPrefix("/static/", files))

	mux.HandleFunc("/", index)
	mux.HandleFunc("/upload", upload)
 
	server := &http.Server{
		Addr: "127.0.0.1:8080",
		Handler: mux,
	}

	TILES = loadTiles()

	fmt.Println("Started!")
	server.ListenAndServe()
}

func index(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("index.html")
	t.Execute(w, nil)
}

func upload(w http.ResponseWriter, r *http.Request) {
	t0 := time.Now()

	r.ParseMultipartForm(10485760)
	file, _, err := r.FormFile("image")
	if err != nil {
		fmt.Println("failed to read an uploaded image", err)
		w.Write([]byte("failed to read en uploaded image" + err.Error()))
		return
	}
	defer file.Close()

	tileSize, _ := strconv.Atoi(r.FormValue("tile_size"))

	origImg, _, err := image.Decode(file)
	if err != nil {
		fmt.Println("failed to decode an uploaded image", err)
		w.Write([]byte("failed to decode an uploaded image" + err.Error()))
		return
	}
	bounds := origImg.Bounds()
	db := cloneTileDB()

	// fan-out
	c1 := cut(origImg, &db, tileSize, bounds.Min.X, bounds.Min.Y, bounds.Max.X/2, bounds.Max.Y/2)
	c2 := cut(origImg, &db, tileSize, bounds.Max.X/2, bounds.Min.Y, bounds.Max.X, bounds.Max.Y/2)
	c3 := cut(origImg, &db, tileSize, bounds.Min.X, bounds.Max.Y/2, bounds.Max.X/2, bounds.Max.Y)
	c4 := cut(origImg, &db, tileSize, bounds.Max.X/2, bounds.Max.Y/2, bounds.Max.X, bounds.Max.Y)

	// fan-in
	c := merge(bounds, c1, c2, c3, c4)
	buf := new(bytes.Buffer)
	jpeg.Encode(buf, origImg, nil)
	imgBase64Str := base64.StdEncoding.EncodeToString(buf.Bytes())

	t1 := time.Now()
	w.Header().Set("Content-Type", "application/json")
	
	json.NewEncoder(w).Encode(map[string]string{
		"original": imgBase64Str,
		"tiled": <-c,
		"duration": fmt.Sprintf("%v ", t1.Sub(t0)),
	})
}

func cut(origImg image.Image, db *TileDB, tileSize, x1, y1, x2, y2 int) <- chan image.Image {
	c := make(chan image.Image)
	sp := image.Point{0, 0}
	go func() {
		newImg := image.NewNRGBA(image.Rect(x1, y1, x2, y2))
		for y := y1; y < y2; y = y + tileSize {
			for x := x1; x < x2; x = x + tileSize {
				r, g, b, _ := origImg.At(x, y).RGBA()
				color := [3]float64{float64(r), float64(g), float64(b)}
				nearest := db.nearest(color)
				file, err := os.Open(nearest)
				if err == nil {
					img, _, err := image.Decode(file)
					if err == nil {
						t := resize(img, tileSize)
						tile := t.SubImage(t.Bounds())
						tileBounds := image.Rect(x, y, x+tileSize, y+tileSize)
						draw.Draw(newImg, tileBounds, tile, sp, draw.Src)
					} else {
						fmt.Println("failed to decode a tile file", nearest, err)
					}
				} else {
					fmt.Println("failed to open a tile file", err)
				}
				file.Close()
			}
		}
		c <- newImg.SubImage(newImg.Rect)
	}()
	return c
}

func merge(r image.Rectangle, c1, c2, c3, c4 <- chan image.Image) <- chan string {
	c := make(chan string)
	go func() {
		var wg sync.WaitGroup
		newImg := image.NewNRGBA(r)
		copy := func(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point) {
			draw.Draw(dst, r, src, sp, draw.Src)
			wg.Done()
		}
		wg.Add(4)
		var s1, s2, s3, s4 image.Image
		var ok1, ok2, ok3, ok4 bool
		for {
			select {
			case s1, ok1 = <-c1:
				go copy(newImg, s1.Bounds(), s1, image.Point{r.Min.X, r.Min.Y})
			case s2, ok2 = <-c2:
				go copy(newImg, s2.Bounds(), s2, image.Point{r.Max.X / 2, r.Min.Y})
			case s3, ok3 = <-c3:
				go copy(newImg, s3.Bounds(), s3, image.Point{r.Min.X, r.Max.Y / 2})
			case s4, ok4 = <-c4:
				go copy(newImg, s4.Bounds(), s4, image.Point{r.Max.X / 2, r.Max.Y / 2})
			}
			if ok1 && ok2 && ok3 && ok4 { break }
		}
		wg.Wait()
		buf := new(bytes.Buffer)
		jpeg.Encode(buf, newImg, nil)
		c <- base64.StdEncoding.EncodeToString(buf.Bytes())
	}()
	return c
}

