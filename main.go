package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	"net/http"
	"strconv"
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
		Addr:    "127.0.0.1:8080",
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

	c := tile(origImg, tileSize)

	buf := new(bytes.Buffer)
	jpeg.Encode(buf, origImg, nil)
	imgBase64Str := base64.StdEncoding.EncodeToString(buf.Bytes())

	t1 := time.Now()
	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"original": imgBase64Str,
		"tiled":    <-c,
		"duration": fmt.Sprintf("%v ", t1.Sub(t0)),
	})
}
