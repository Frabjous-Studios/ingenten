package main

import (
	"errors"
	"fmt"
	"github.com/Frabjous-Studios/ingenten"
	uiimage "github.com/ebitenui/ebitenui/image"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"golang.org/x/image/colornames"
	"image"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	var err error
	gameWidth, gameHeight := 640, 480

	ebiten.SetWindowSize(gameWidth, gameHeight)
	ebiten.SetWindowTitle("Pixel Font Demo")

	game := &Game{
		Width:  gameWidth,
		Height: gameHeight,

		grey: ebiten.NewImage(1, 1),
	}
	game.grey.Set(0, 0, colornames.Grey)

	starterText, err := loadFile("initial_text.txt")
	if err != nil {
		starterText = ""
	}

	game.font, err = ingenten.LoadPixelFont("pixel_fonts.png", os.DirFS("."))
	if err != nil {
		log.Fatal("error loading:", err)
	}
	fmt.Printf("%#v", game.font)

	game.text = starterText
	game.nineSlice, err = LoadNineSlice()
	if err != nil {
		log.Fatal("error loading:", err)
	}

	debounce = make(map[ebiten.Key]int)
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

func LoadNineSlice() (*uiimage.NineSlice, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "9patch-") || !strings.HasSuffix(name, ".png") {
			continue
		}
		return loadNineSlice(name)
	}
	return nil, errors.New("please run this program next to a file named like '9patch-4,4,4-4,4,4.png")
}

func loadNineSlice(file string) (*uiimage.NineSlice, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	rows, cols, err := parseFilename(file)
	if err != nil {
		return nil, err
	}
	return uiimage.NewNineSlice(ebiten.NewImageFromImage(img), rows, cols), nil
}

func loadFile(file string) (string, error) {
	bytes, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("could not open file: %v", file)
	}
	return string(bytes), nil
}

func parseFilename(file string) (rows [3]int, cols [3]int, err error) {
	file = strings.TrimSuffix(file, filepath.Ext(file))
	toks := strings.Split(file, "-")
	if len(toks) != 3 {
		err = errors.New("expected filename to be formatted like 9patch-4,4,4-4,4,4.png")
		return
	}
	rows, err = parseTriple(toks[1])
	if err != nil {
		return
	}
	cols, err = parseTriple(toks[2])
	if err != nil {
		return
	}
	return
}

func parseTriple(str string) (result [3]int, err error) {
	toks := strings.Split(str, ",")
	if len(toks) != 3 {
		err = errors.New("expected filename to be formatted like 9patch-4,4,4-4,4,4.png")
		return
	}
	for i := 0; i < 3; i++ {
		result[i], err = strconv.Atoi(toks[i])
		if err != nil {
			return
		}
	}
	return result, err
}

type Game struct {
	Width, Height int
	ticks         int

	nineSlice *uiimage.NineSlice
	font      *ingenten.PixelFont
	text      string

	grey *ebiten.Image
}

func (g *Game) Layout(outsideWidth int, outsideHeight int) (screenWidth int, screenHeight int) {
	return g.Width, g.Height
}

var TPS int
var debounce map[ebiten.Key]int
var runes []rune

const debounceMS = 150

func (g *Game) Update() error {
	TPS = ebiten.TPS()
	g.ticks++
	runes = ebiten.AppendInputChars(runes[:0])
	g.text += string(runes)

	if repeatingKeyPressed(ebiten.KeyEnter) {
		g.text += "\n"
	}
	if repeatingKeyPressed(ebiten.KeyBackspace) {
		if len(g.text) > 0 {
			g.text = g.text[:len(g.text)-1]
		}
	}

	return nil
}

func repeatingKeyPressed(key ebiten.Key) bool {
	const (
		delay    = 30
		interval = 3
	)
	d := inpututil.KeyPressDuration(key)
	if d == 1 {
		return true
	}
	if d >= delay && (d-delay)%interval == 0 {
		return true
	}
	return false
}

func (g *Game) Draw(screen *ebiten.Image) {
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Scale(float64(g.Width), float64(g.Height))
	screen.DrawImage(g.grey, opts)
	opts.GeoM.Reset()

	origin := image.Pt(30, 30)
	measured := g.font.Measure(g.text)

	const padding = 2
	const topPadding = 2
	w, h := g.nineSlice.MinSize()
	g.nineSlice.Draw(screen, measured.Dx()+w+2*padding, measured.Dy()+h+2*padding+topPadding, func(opts *ebiten.DrawImageOptions) {
		opts.GeoM.Translate(float64(origin.X), float64(origin.Y))
	})

	g.font.PrintTo(screen, origin.Add(image.Pt(padding+w/2, topPadding+padding+h/2)), g.text)
}
