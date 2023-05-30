package ingenten

import (
	"errors"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"image"
	"image/color"
	"io/fs"
	"strings"
)

// PixelFont represents a single pixel font.
type PixelFont struct {
	image *ebiten.Image

	letters map[rune]letter

	lineHeight int
}

// letter stores all information required for a letter.
type letter struct {
	rect image.Rectangle

	rightKern int
	leftKern  int
	descender int // pixels in the descender for this letter.
}

// Print lays out the runes for the provided text, left-justified, with the top-left corner of the text at the
// provided origin. No automatic wrapping is included.
func (pf *PixelFont) Print(screen *ebiten.Image, origin image.Point, text string) {
	pf.PrintOpts(screen, origin, text, &ebiten.DrawImageOptions{})

}

// PrintOpts works like Print, using options from the provided opts. GeoM is reset prior to drawing.
func (pf *PixelFont) PrintOpts(screen *ebiten.Image, origin image.Point, text string, opts *ebiten.DrawImageOptions) {
	opts.GeoM.Reset()
	pf.doLayout([]rune(text), func(p image.Point, l letter) {
		opts.GeoM.Translate(float64(origin.X+p.X), float64(origin.Y+p.Y))
		screen.DrawImage(pf.image.SubImage(l.rect).(*ebiten.Image), opts)
		opts.GeoM.Reset()
	})
}

// PrintRect prints the provided text, left-justified, with the top-left corner of the text at the provided origin.
// Text is line-wrapped so that all the characters lie within the provided rectangle. No word-wrapping is attempted.
func (pf *PixelFont) PrintRect(screen *ebiten.Image, rect image.Rectangle, text string) {
	pf.PrintRectOpts(screen, rect, text, &ebiten.DrawImageOptions{})

}

// PrintRectOpts works like PrintRect
func (pf *PixelFont) PrintRectOpts(screen *ebiten.Image, rect image.Rectangle, text string, opts *ebiten.DrawImageOptions) {
	opts.GeoM.Reset()
	pf.doLayoutRect([]rune(text), rect, func(p image.Point, l letter) {
		opts.GeoM.Translate(float64(p.X+rect.Min.X), float64(p.Y+rect.Min.Y))
		screen.DrawImage(pf.image.SubImage(l.rect).(*ebiten.Image), opts)
		opts.GeoM.Reset()
	})
}

// Measure measures the rectangle the text would be rendered in if it were to be written to screen using Print.
func (pf *PixelFont) Measure(text string, origin image.Point) image.Rectangle {
	result := image.Rectangle{}
	pf.doLayout([]rune(text), func(point image.Point, letter letter) {
		rect := image.Rectangle{Max: image.Pt(letter.rect.Dx(), pf.lineHeight)}
		result = result.Union(rect.Add(point))
	})
	return result.Add(origin)
}

// MeasureRect measures the rectangle the text would be rendered in if it were to be written to screen using PrintRect
func (pf *PixelFont) MeasureRect(text string, rect image.Rectangle) image.Rectangle {
	result := image.Rectangle{}
	pf.doLayoutRect([]rune(text), rect, func(point image.Point, letter letter) {
		rect := image.Rectangle{Max: image.Pt(letter.rect.Dx(), pf.lineHeight)}
		result = result.Union(rect.Add(point))
	})
	return result.Add(rect.Min)
}

// DoLayoutRect lays out the provided text in the provided rectangle, left-justified, with word wrapping. In case of
// long words that exceed the bounds of rect, the word-wrapping provided by this function extends the boundaries of
// rect, rather than breaking the word onto separate lines.
func (pf *PixelFont) DoLayoutRect(text string, rect image.Rectangle, do func(pos image.Point, img *ebiten.Image)) {
	pf.doLayoutRect([]rune(text), rect, func(pos image.Point, letter letter) {
		img := pf.image.SubImage(letter.rect).(*ebiten.Image)
		do(pos, img)
	})
}

// DoLayout calls do for each rune in the provided text, passing the relative position of the letter from the origin,
// and a subimage containing the glyph to be rendered.
//
// This func is intended to be used to create your own text effects or animations without worrying about layout.
func (pf *PixelFont) DoLayout(text string, do func(pos image.Point, img *ebiten.Image)) {
	pf.doLayout([]rune(text), func(point image.Point, letter letter) {
		img := pf.image.SubImage(letter.rect).(*ebiten.Image)
		do(point, img)
	})
}

// doLayoutIn performs left-justified layout along with word-wrapping. The layout provided by this function will
// overflow the provided rectangle in the case any word in runes is longer
func (pf *PixelFont) doLayoutRect(runes []rune, rect image.Rectangle, do func(image.Point, letter)) {
	space := pf.spaceWidth()

	rect = rect.Add(rect.Min.Mul(-1))
	curr := image.Point{}
	chunkStart := curr
	var start int     // indices into text for the current line
	var wordStart int // start of the current word
	for idx, r := range runes {
		if r == '\n' {
			curr = curr.Add(image.Pt(0, pf.lineHeight)) // line break
			curr.X = 0
			continue
		}
		l, ok := pf.letters[r]
		if !ok { // a space or a non-existent rune which might as well be a space.
			curr.X = curr.X + space
			wordStart = idx + 1
			continue
		}
		kern := 0
		if idx > 0 {
			kern = pf.getKerning(runes[idx-1], r)
		}
		next := curr.Add(image.Pt(l.rect.Dx()+kern, 0))
		if !next.In(rect) {
			pf.doLayout(runes[start:wordStart], func(point image.Point, letter letter) {
				do(point.Add(image.Pt(0, chunkStart.Y)), letter)
			})
			curr = curr.Add(image.Pt(0, pf.lineHeight)) // do a line break
			chunkStart = curr
			curr.X = 0
			start = wordStart // update the line start
		} else {
			curr = next
		}
	}
	pf.doLayout(runes[start:], func(point image.Point, letter letter) {
		do(point.Add(image.Pt(0, chunkStart.Y)), letter)
	})
}

// getKerning computes the kerning between the two runes.
func (pf *PixelFont) getKerning(left, right rune) int {
	l, lok := pf.letters[left]
	r, rok := pf.letters[right]
	if !lok && !rok {
		return pf.spaceWidth()
	} else if !lok {
		return max(r.leftKern, 1)
	} else if !rok {
		return max(r.rightKern, 1)
	}
	return max(max(r.leftKern, l.rightKern), 1)
}

// doLayout calls do for each rune in the provided func. If a letter in this pixel font is associated with the rune it
// is included. This func lays everything out with the first letter of the text starting at the origin: 0,0, the
// top-right corner of the screen, any translation is up to the caller.
//
// All letters missing from the font are rendered as spaces. This func handles linebreaks '\n' using the minimum line
// height for the font.
func (pf *PixelFont) doLayout(runes []rune, do func(image.Point, letter)) {
	space := pf.spaceWidth()
	curr := image.Point{}
	for idx, r := range runes {
		if r == '\n' { // handle line-breaks
			curr = curr.Add(image.Pt(0, pf.lineHeight))
			curr.X = 0
			continue
		}
		l, ok := pf.letters[r]
		if !ok { // missing glyphs are all treated as spaces
			curr.X = curr.X + space
			continue
		}
		arg := curr // adjust the height based on descender and total font line-height.
		arg.Y += pf.lineHeight - l.rect.Dy() + l.descender
		do(arg, l)
		kern := 0
		if idx > 0 {
			kern = pf.getKerning(runes[idx-1], r)
		}
		curr = curr.Add(image.Pt(l.rect.Dx()+kern, 0))
	}
}

// spaceWidth returns the width of a space or any missing character.
func (pf *PixelFont) spaceWidth() int {
	space := 5 // the width of a space or any missing character
	if l, ok := pf.letters['m']; ok {
		space = l.rect.Dx()/2 + 1
	}
	return space
}

// String prints the letters associated with each string.
func (pf *PixelFont) String() string {
	var result strings.Builder
	for _, runs := range rows {
		for _, run := range runs {
			for r := run[0]; r <= run[1]; r++ {
				result.WriteString(fmt.Sprintf("'%c': %v, ", rune(r), pf.letters[rune(r)]))
			}
		}
		result.WriteString("\n")
	}
	return result.String()
}

// LoadPixelFont loads and parses a PixelFont from the provided fs.FS. This func uses image.Decode, callers will need to
// include the correct anonymous import from the image package in order to use this func properly. For example:
//
//	import _ "image/png"
//
//	var pf = LoadPixelFont("my_font.png", os.DirFS("."))
//
// The image loaded contains all of the information needed to parse and use the font. An example font is included in
// the README for this package.
func LoadPixelFont(file string, fs fs.FS) (*PixelFont, error) {

	f, err := fs.Open(file)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return parseImage(img)
}

// rows stores the expected list of runs in the file. Each file takes four rows, each with multiple runs.
var rows = [4][][2]int{
	{{65, 90}},
	{{97, 122}},
	{{48, 64}},
	{{33, 47}, {91, 96}, {123, 126}},
}

func parseImage(img image.Image) (*PixelFont, error) {
	transp := img.At(0, 0)               // transparent color behind each cell
	start, err := findStart(img, transp) // start is the lower-left corner of the first cell
	if err != nil {
		return nil, err
	}
	result := &PixelFont{
		image:   ebiten.NewImage(img.Bounds().Dx(), img.Bounds().Dy()),
		letters: make(map[rune]letter),
	}
	rowStart := start // rowStart is the lower-left corner of the first cell of the current row
	var ok bool

	// TODO: fix kerning
	for _, runs := range rows {
		var rowBase image.Point
		rowStart = start
	nextRow:
		for _, run := range runs {
			for i := run[0]; i <= run[1]; i++ {
				letter, ok := scanCell(img, start, transp)
				if !ok {
					break nextRow
				}
				if rowBase.X == 0 && rowBase.Y == 0 {
					rowBase.X = letter.rect.Min.X
					rowBase.Y = letter.rect.Max.Y
				} else {
					letter.descender = letter.rect.Max.Y - rowBase.Y
				}
				result.letters[rune(i)] = letter

				start, ok = findNextCell(img, letter.rect, transp)
				if !ok {
					break nextRow
				}
			}
		}
		start, ok = nextRow(img, rowStart, transp)
		if !ok {
			break
		}
	}
	result.pack(img, transp)
	return result, nil
}

// pack packs the letters from img into an Ebiten image.
func (pf *PixelFont) pack(img image.Image, transp color.Color) {
	const padding = 1 // this... is in case we find we need it someday.
	offset := image.Pt(0, 0)
	maxDescender := 0
	for _, runs := range rows {
		maxY := 0
		for _, run := range runs {
			for r := rune(run[0]); r <= rune(run[1]); r++ {
				l := pf.letters[r]
				for x := 0; x < l.rect.Dx(); x++ {
					for y := 0; y < l.rect.Dy(); y++ {
						c := img.At(l.rect.Min.X+x, l.rect.Min.Y+y)
						if c != transp {
							pf.image.Set(offset.X+x, offset.Y+y, c)
						}
					}
				}
				toWrite := l
				toWrite.rect = image.Rectangle{Min: offset, Max: offset.Add(image.Pt(l.rect.Dx(), l.rect.Dy()))}
				pf.letters[r] = toWrite
				offset.X += l.rect.Dx() + padding
				maxY = max(maxY, l.rect.Dy())
				maxDescender = max(l.descender, maxDescender)
			}
		}
		offset = offset.Add(image.Pt(0, maxY+padding))
		offset.X = 0
		pf.lineHeight = max(pf.lineHeight, maxY)
	}
	pf.lineHeight -= maxDescender
}

// scanCell scans the next cell, returning the rectangle of the original image around the glyph. Returns true iff the
// there is no next glyph on the provided row.
func scanCell(img image.Image, start image.Point, transp color.Color) (letter, bool) {
	x, y := start.X, start.Y
	var result image.Rectangle
	for img.At(x, y) == transp {
		y--
	}
	y++
	result.Min = image.Pt(x, y)

	leftKern := 1
	x = start.X + 1

leftKernDone:
	for x < img.Bounds().Dx() {
		for y := start.Y; y > result.Min.Y; y-- {
			if img.At(x, y) != transp {
				break leftKernDone
			}
		}
		x++
		leftKern++ // we completed a full pass; increment the kerning
	}

	// find the next adjacent full "vertical bar" of transparent pixels
	rightKernStart := 0
	rightKern := 0
nextBar:
	for x < img.Bounds().Dx() {
		x++
		if x == img.Bounds().Dx() {
			return letter{}, false
		}
		// shadow y in this loop
		for y := start.Y; y > result.Min.Y; y-- {
			if img.At(x, y) != transp {
				rightKernStart = 0
				continue nextBar
			}
		}
		if rightKernStart == 0 {
			rightKernStart = x
		}
		rightKern = x
		y := start.Y
		desc := 0
		for img.At(x, y) == transp { // scan down for line height
			y++
			desc++
		}
		y--
		if img.At(x+1, y) != transp {
			result.Max = image.Pt(x, y+1)
			break
		}
	}
	rightKern = rightKern - rightKernStart
	return letter{rect: result, leftKern: leftKern - 1, rightKern: rightKern}, true
}

func nextRow(img image.Image, rowStart image.Point, transp color.Color) (image.Point, bool) {
	sawCellStart := false
	x := rowStart.X
	for y := rowStart.Y + 1; y < img.Bounds().Dy(); y++ {
		if img.At(x, y) == transp {
			sawCellStart = true
		} else if sawCellStart {
			return image.Pt(x, y-1), true
		}
	}
	if sawCellStart {
		return image.Pt(x, img.Bounds().Dy()-1), true
	}
	return image.Point{}, false
}

func findNextCell(img image.Image, lastCell image.Rectangle, transp color.Color) (image.Point, bool) {
	H, W := img.Bounds().Dy(), img.Bounds().Dx()
	y := lastCell.Min.Y
	for x := lastCell.Max.X + 1; x < W; x++ {
		if img.At(x, y) == transp {
			// go down to find line height
			for y := y; y < H; y++ {
				if img.At(x, y) != transp {
					return image.Pt(x, y-1), true
				}
			}
		}
	}
	return image.Point{}, false // no new cell
}

func findStart(img image.Image, transp color.Color) (image.Point, error) {
	W, H := img.Bounds().Dx(), img.Bounds().Dy()
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			if y == 0 && x == 0 {
				continue
			}
			if img.At(x, y) != transp {
				continue
			}
			for img.At(x, y) == transp {
				y++
			}
			return image.Pt(x, y-1), nil
		}
	}
	return image.Point{}, errors.New("unable to parse image; no matching guide pixels were found")
}

func max(x, y int) int {
	if x < y {
		return y
	}
	return x
}
