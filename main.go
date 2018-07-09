package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	yaml "gopkg.in/yaml.v2"
)

type Target struct {
	Api         string
	BearerToken string
}

var target Target
var statuses = []string{"aborted", "errored", "failed", "succeeded", "paused"}
var colors = map[string]color.RGBA{
	"bg":           color.RGBA{39, 55, 71, 255},
	"paused":       color.RGBA{0x44, 0xA8, 0xEB, 0xff},
	"pausedBorder": color.RGBA{0x34, 0x98, 0xDB, 0xff},
	"running":      color.RGBA{0xF2, 0xC5, 0x00, 0xff},
	"aborted":      color.RGBA{0x8F, 0x4B, 0x2D, 0xff},
	"errored":      color.RGBA{0xE6, 0x7E, 0x21, 0xff},
	"failed":       color.RGBA{0xE7, 0x4C, 0x3C, 0xff},
	"succeeded":    color.RGBA{0x2E, 0xCC, 0x71, 0xff},
	"green":        color.RGBA{0x71, 0xDD, 0x71, 0xff},
}

type Pipeline struct {
	Name     string
	Paused   bool
	TeamName string
	Running  bool
	Statuses map[string]int
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
		}
	}()
	refreshData := func() {
		data = GetData()
		atomic.StoreInt32(&countdown, 2)
		dataChanged = true
	}
	go func() {
		refreshData()
		for range time.Tick(2 * time.Second) {
			refreshData()
		}
	}()

	var fontFace font.Face
	fontFace, err = loadTTF("rubik.ttf", 14)
	if err != nil {
		fmt.Println("Warning: could no load rubik.ttf:", err)
		fontFace = basicfont.Face7x13
	}

	fps := time.Tick(time.Second / 20)
	for !win.Closed() {
		win.SetTitle(fmt.Sprintf("Concourse Summary (%d)", countdown))
		if maxWidth != win.Bounds().W() || maxHeight != win.Bounds().H() {
			dataChanged = true
			maxWidth = win.Bounds().W()
			maxHeight = win.Bounds().H()
		}
		w, h, perRow := maxWidth, maxHeight, 1.0
		if len(data) > 0 {
			perRow = math.Ceil(math.Sqrt(float64(len(data))))
			w = maxWidth / perRow
			numRows := math.Ceil(float64(len(data)) / float64(perRow))
			h = maxHeight / numRows
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
					"%s/teams/%s/pipelines/%s", // "?groups=java",
					target.Api,
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
					imd.Color = colors["pausedBorder"]
					imd.Push(bounds.Min.Add(pixel.V(-10, 10)), bounds.Max.Add(pixel.V(10, -10)))
					imd.Rectangle(0)
				} else if datum.Running {
					imd.Color = colors["running"]
					imd.Push(bounds.Min.Add(pixel.V(-10, 10)), bounds.Max.Add(pixel.V(10, -10)))
					imd.Rectangle(0)
				}
				imd.Color = color.RGBA{0x5D, 0x6D, 0x7D, 0xff}
				imd.Push(bounds.Min, bounds.Max)
				imd.Rectangle(0)
				total := 0
				for _, val := range datum.Statuses {
					total += val
				}
				pos := bounds.Min
				for _, key := range statuses {
					if key == "succeeded" && datum.Statuses[key] == total && total > 0 {
						imd.Color = colors["green"]
						imd.Push(bounds.Min, bounds.Max)
						imd.Rectangle(0)
					} else if datum.Statuses[key] > 0 {
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
		fmt.Printf("Usage %s [HOSTNAME or Fly Target]\n  eg. %s buildpacks.ci.cf-app.com\n", os.Args[0], os.Args[0])
		os.Exit(1)
	}
	target = loadFlyRc(os.Args[1])

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

func loadFlyRc(target string) Target {
	flyrc := struct {
		Targets map[string]struct {
			Api   string `yaml:"api"`
			Token struct {
				Type  string `yaml:"type"`
				Value string `yaml:"value"`
			} `yaml:"token"`
		} `yaml:"targets"`
	}{}
	body, err := ioutil.ReadFile(filepath.Join(os.Getenv("HOME"), ".flyrc"))
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(body, &flyrc)
	if err != nil {
		panic(err)
	}
	if val, ok := flyrc.Targets[target]; ok {
		return Target{Api: val.Api, BearerToken: val.Token.Value}
	} else if strings.HasPrefix(target, "http") {
		return Target{Api: target, BearerToken: ""}
	} else {
		fmt.Println("Target must be either a target in flyrc or a url")
		os.Exit(2)
		return Target{}
	}
}

func GetJSON(path string, data interface{}) error {
	client := http.Client{
		Timeout: time.Second * 2, // Maximum of 2 secs
	}

	req, err := http.NewRequest(http.MethodGet, target.Api+path, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "concourse-summary-gl")
	if target.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer+"+target.BearerToken)
	}

	res, getErr := client.Do(req)
	if getErr != nil {
		return err
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("Received %d from %s", res.StatusCode, path)
	}

	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return err
	}

	return json.Unmarshal(body, data)
}

type Job struct {
	Name      string `json:"name"`
	Pipeline  string `json:"pipeline_name"`
	Team      string `json:"team_name"`
	Paused    bool   `json:"paused"`
	NextBuild struct {
		Status string `json:"status"`
	} `json:"next_build"`
	Build struct {
		Status string `json:"status"`
	} `json:"finished_build"`
}

func GetData() []Pipeline {
	pipelines := make([]Pipeline, 0, 0)
	if err := GetJSON("/api/v1/pipelines", &pipelines); err != nil {
		panic(err)
	}
	lookup := make(map[string]*Pipeline)
	for i, _ := range pipelines {
		p := &pipelines[i]
		p.Statuses = make(map[string]int)
		lookup[p.Name] = p
	}

	jobs := make([]Job, 0, 0)
	if err := GetJSON("/api/v1/jobs", &jobs); err != nil {
		panic(err)
	}
	for _, job := range jobs {
		if p := lookup[job.Pipeline]; p != nil {
			if job.Paused {
				p.Statuses["paused"]++
			} else {
				p.Statuses[job.Build.Status]++
				if job.NextBuild.Status != "" {
					p.Running = true
				}
			}
		}
	}

	return pipelines
}
