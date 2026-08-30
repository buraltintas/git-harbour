package server

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var boldFont, _ = opentype.Parse(gobold.TTF)
var regularFont, _ = opentype.Parse(goregular.TTF)

func cardFace(f *opentype.Font, size float64) (font.Face, error) {
	return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
}

func cardText(dst draw.Image, face font.Face, c color.Color, x, y int, value string) {
	d := font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: face, Dot: fixed.P(x, y)}
	d.DrawString(value)
}

func cardRect(dst draw.Image, r image.Rectangle, c color.Color) {
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src)
}

func renderPVPShareCard(x PVPShare) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 630))
	bg := color.RGBA{13, 17, 23, 255}
	panel := color.RGBA{22, 27, 34, 255}
	text := color.RGBA{240, 246, 252, 255}
	muted := color.RGBA{139, 148, 158, 255}
	accent := color.RGBA{63, 185, 80, 255}
	cardRect(img, img.Bounds(), bg)
	cardRect(img, image.Rect(56, 48, 1144, 582), panel)
	cardRect(img, image.Rect(56, 48, 68, 582), accent)

	title, e := cardFace(boldFont, 30)
	if e != nil {
		return nil, e
	}
	defer title.Close()
	hero, e := cardFace(boldFont, 54)
	if e != nil {
		return nil, e
	}
	defer hero.Close()
	result, e := cardFace(boldFont, 38)
	if e != nil {
		return nil, e
	}
	defer result.Close()
	body, e := cardFace(regularFont, 24)
	if e != nil {
		return nil, e
	}
	defer body.Close()
	label, e := cardFace(boldFont, 20)
	if e != nil {
		return nil, e
	}
	defer label.Close()

	cardText(img, title, text, 100, 105, "GitHarbour")
	cardText(img, label, accent, 940, 101, x.Rank)
	cardText(img, hero, text, 100, 195, "@"+x.Winner.Login)
	cardText(img, body, muted, 105, 239, "VS @"+x.Loser.Login)
	cardText(img, result, accent, 100, 314, "VICTORY")

	accuracy := 0.0
	if x.WinnerResult.Shots > 0 {
		accuracy = 100 * float64(x.WinnerResult.Hits) / float64(x.WinnerResult.Shots)
	}
	cardText(img, body, text, 100, 366, fmt.Sprintf("%d shots  ·  %.0f%% accuracy  ·  %+.0d PvP rating", x.WinnerResult.Shots, accuracy, x.WinnerResult.RatingDelta))
	cardText(img, body, muted, 100, 535, "Your GitHub history is a battlefield.")

	levels := []color.RGBA{{33, 38, 45, 255}, {14, 68, 41, 255}, {0, 109, 50, 255}, {38, 166, 65, 255}, {57, 211, 83, 255}}
	for side := 0; side < 2; side++ {
		ox := 700 + side*218
		for week := 0; week < 5; week++ {
			for day := 0; day < 7; day++ {
				level := int(math.Abs(float64((week*7 + day*3 + side*2) % len(levels))))
				cardRect(img, image.Rect(ox+week*34, 205+day*34, ox+week*34+27, 205+day*34+27), levels[level])
			}
		}
	}
	return img, nil
}
