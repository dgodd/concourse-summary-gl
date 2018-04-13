package main

import (
	"fmt"
	"image/color"
	"io/ioutil"
	"math"
	"os"
	"sync/atomic"
	"time"

	"github.com/faiface/pixel"
	"github.com/faiface/pixel/imdraw"
	"github.com/faiface/pixel/pixelgl"
	"github.com/faiface/pixel/text"
	"github.com/golang/freetype/truetype"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/image/colornames"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

var hostName string
var statuses = []string{"aborted", "errored", "failed", "succeeded"}
var colors = map[string]color.RGBA{
	"bg":        color.RGBA{39, 55, 71, 255},
	"paused":    color.RGBA{0x34, 0x98, 0xDB, 0xff},
	"running":   color.RGBA{0xF2, 0xC5, 0x00, 0xff},
	"aborted":   color.RGBA{0x8F, 0x4B, 0x2D, 0xff},
	"errored":   color.RGBA{0xE6, 0x7E, 0x21, 0xff},
	"failed":    color.RGBA{0xE7, 0x4C, 0x3C, 0xff},
	"succeeded": color.RGBA{0x2E, 0xCC, 0x71, 0xff},
}

func run() {
	maxWidth := 900.0
	maxHeight := 600.0
	cfg := pixelgl.WindowConfig{
		Title:     "Concourse Summary",
		Bounds:    pixel.R(0, 0, maxWidth, maxHeight),
		Resizable: true,
		VSync:     false,
	}
	win, err := pixelgl.NewWindow(cfg)
	if err != nil {
		panic(err)
	}

	data := make([]Pipeline, 0, 0)
	dataChanged := true
	countdown := int32(0)
	go func() {
		for range time.Tick(time.Second) {
			atomic.AddInt32(&countdown, -1)
			win.SetTitle(fmt.Sprintf("Concourse Summary (%d)", countdown))
		}
	}()
	refreshData := func() {
		data = GetData()
		atomic.StoreInt32(&countdown, 30)
		win.SetTitle(fmt.Sprintf("Concourse Summary (%d)", countdown))
		dataChanged = true
	}
	go func() {
		refreshData()
		for range time.Tick(30 * time.Second) {
			refreshData()
		}
	}()

	var fontFace font.Face
	fontFace, err = loadTTF("rubik.ttf", 14)
	if err != nil {
		fmt.Println("Warning: could no load rubik.ttf:", err)
		fontFace = basicfont.Face7x13
	}

	fps := time.Tick(time.Second / 2)
	for !win.Closed() {
		if maxWidth != win.Bounds().W() || maxHeight != win.Bounds().H() {
			dataChanged = true
		}
		maxWidth = win.Bounds().W()
		maxHeight = win.Bounds().H()
		w, h, perRow := maxWidth, maxHeight, 1.0
		if len(data) > 0 {
			perRow = math.Ceil(math.Sqrt(float64(len(data))))
			w = maxWidth / perRow
			h = maxHeight / perRow
		}

		if win.JustPressed(pixelgl.MouseButtonLeft) {
			pos := win.MousePosition()
			col := int(pos.X / w)
			row := int((maxHeight - pos.Y) / h)
			idx := row*int(perRow) + col
			fmt.Printf("Mouse: %+v ; %d\n", pos, idx)
			if idx < len(data) {
				val := data[idx]
				url := fmt.Sprintf(
					"https://%s/teams/%s/pipelines/%s", // "?groups=java",
					hostName,
					val.TeamName,
					val.Name,
				)
				fmt.Println(url)
				open.Run(url)
			}
		}

		if dataChanged {
			dataChanged = false
			win.Clear(colors["bg"])

			for idx, datum := range data {
				col := float64(idx % int(perRow))
				row := float64(idx / int(perRow))
				bounds := pixel.R((col*w)+10, maxHeight-(row*h)-10, ((col+1)*w)-10, maxHeight-((row+1)*h)+10)

				imd := imdraw.New(nil)
				if datum.Paused {
					imd.Color = colors["paused"]
					imd.Push(bounds.Min.Add(pixel.V(-10, 10)), bounds.Max.Add(pixel.V(10, -10)))
					imd.Rectangle(0)
				}
				if datum.Running {
					imd.Color = colors["running"]
					imd.Push(bounds.Min.Add(pixel.V(-10, 10)), bounds.Max.Add(pixel.V(10, -10)))
					imd.Rectangle(0)
				}
				imd.Color = color.RGBA{0, 0, 0, 255}
				imd.Push(bounds.Min, bounds.Max)
				imd.Rectangle(0)
				total := 0
				for _, val := range datum.Statuses {
					total += val
				}
				pos := bounds.Min
				for _, key := range statuses {
					if datum.Statuses[key] > 0 {
						imd.Color = colors[key]
						size := float64(datum.Statuses[key]) * bounds.W() / float64(total)
						imd.Push(pos, pos.Add(pixel.V(size, bounds.H())))
						imd.Rectangle(0)
						pos = pos.Add(pixel.V(size, 0))
					}
				}

				atlas := text.NewAtlas(fontFace, text.ASCII)
				txt := text.New(bounds.Center().Add(pixel.V(0, -2)), atlas)
				txt.Color = colornames.White
				txt.Dot.X -= txt.BoundsOf(datum.Name).W() / 2
				fmt.Fprintln(txt, datum.Name)
				imd.Draw(win)
				txtScale := 1.0
				if (txt.BoundsOf(datum.Name).W() + 20.0) > bounds.W() {
					txtScale = bounds.W() / (txt.BoundsOf(datum.Name).W() + 20.0)
				}
				txt.Draw(win, pixel.IM.Scaled(bounds.Center(), txtScale))
			}
		}
		win.Update()
		<-fps
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage %s [HOSTNAME]\n  eg. %s buildpacks.ci.cf-app.com\n", os.Args[0], os.Args[0])
		os.Exit(1)
	}
	hostName = os.Args[1]

	pixelgl.Run(run)
}

func loadTTF(path string, size float64) (font.Face, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	font, err := truetype.Parse(bytes)
	if err != nil {
		return nil, err
	}

	return truetype.NewFace(font, &truetype.Options{
		Size:              size,
		GlyphCacheEntries: 10,
	}), nil
}
