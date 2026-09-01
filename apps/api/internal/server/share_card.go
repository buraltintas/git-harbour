package server

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/githarbour/githarbour/apps/api/internal/game"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var boldFont, _ = opentype.Parse(gobold.TTF)
var regularFont, _ = opentype.Parse(goregular.TTF)

func renderSoloShareCard(g *State, u User) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, 1200, 630))
	bg := color.RGBA{13, 17, 23, 255}
	panel := color.RGBA{22, 27, 34, 255}
	text := color.RGBA{240, 246, 252, 255}
	muted := color.RGBA{139, 148, 158, 255}
	accent := color.RGBA{63, 185, 80, 255}
	cardRect(img, img.Bounds(), bg)
	cardRect(img, image.Rect(56, 48, 1144, 582), panel)
	cardRect(img, image.Rect(56, 48, 68, 582), accent)
	title, err := cardFace(boldFont, 30)
	if err != nil {
		return nil, err
	}
	defer title.Close()
	hero, err := cardFace(boldFont, 48)
	if err != nil {
		return nil, err
	}
	defer hero.Close()
	body, err := cardFace(regularFont, 23)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	label, err := cardFace(boldFont, 20)
	if err != nil {
		return nil, err
	}
	defer label.Close()
	cardText(img, title, text, 100, 105, "GitHarbour")
	result, playerStart, enemyStart := "DEFEAT", g.PlayerStart, g.EnemyStart
	shots, targets, board := g.PlayerTargetShots, g.EnemyTargetCount, g.EnemyBoard
	fleetLine := ""
	if g.Ruleset == "contribution_fleet_v3" {
		playerActions, _, misses, _, clashes, _, wins, _ := fleetMetrics(g.FleetActions)
		fleetLine = fmt.Sprintf("%d units  ·  %d actions  ·  %d/%d clashes won  ·  %d misses  ·  %+.0d rating", len(g.PlayerDeployment), playerActions, wins, clashes, misses, g.RatingDelta)
	}
	if g.Ruleset == game.ContributionBattleshipRuleset {
		hits, misses := targetShotCounts(g.PlayerTargetShots)
		fleetLine = fmt.Sprintf("%d units  ·  %d shots  ·  %d hits  ·  %d misses  ·  %+.0d rating", len(g.PlayerDeployment), len(g.PlayerTargetShots), hits, misses, g.RatingDelta)
	}
	if len(g.PlayerBoard) != 70 || len(g.EnemyBoard) != 70 {
		result, playerStart, enemyStart = "ARCHIVED HISTORY HUNT", g.PeriodStart, ""
		shots, targets, board = g.Shots, g.TargetCount, g.Board
	} else if g.Winner == "player" {
		result = "VICTORY"
	}
	cardText(img, label, accent, 100, 154, result)
	cardText(img, hero, text, 100, 225, "@"+u.Login)
	periods := playerStart
	if enemyStart != "" {
		periods = fmt.Sprintf("%s–%s  vs  %s–%s", playerStart, dateEnd(playerStart), enemyStart, dateEnd(enemyStart))
	}
	cardText(img, body, muted, 100, 274, periods)
	accuracy := 0.0
	hits, _ := targetShotCounts(shots)
	if len(shots) > 0 {
		accuracy = 100 * float64(hits) / float64(len(shots))
	}
	if hits == 0 && targets > 0 {
		hits = targets
	}
	if fleetLine != "" {
		cardText(img, body, text, 100, 335, fleetLine)
	} else {
		cardText(img, body, text, 100, 335, fmt.Sprintf("%d targets  ·  %d shots  ·  %.0f%% accuracy  ·  %+.0d rating", hits, len(shots), accuracy, g.RatingDelta))
	}
	cardText(img, body, muted, 100, 535, "Your activity builds your fleet.")
	levels := []color.RGBA{{33, 38, 45, 255}, {14, 68, 41, 255}, {0, 109, 50, 255}, {38, 166, 65, 255}, {57, 211, 83, 255}}
	for i, cell := range board {
		x, y := i/7, i%7
		level := cell.ContributionLevel
		if level < 0 || level >= len(levels) {
			level = 0
		}
		cardRect(img, image.Rect(760+x*32, 170+y*32, 786+x*32, 196+y*32), levels[level])
	}
	return img, nil
}

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
