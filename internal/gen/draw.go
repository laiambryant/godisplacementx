package gen

// Generate renders a displacement map for the given parameters and size using
// the provided RNG. It is a faithful port of draw.ts (the per-iteration loop and
// the six drawing primitives). The canvas is square in the original app; width
// and height are accepted independently but several formulas use width for both
// axes exactly as the original does.
func Generate(p Params, width, height int, g *RNG) (*Canvas, error) {
	c := NewCanvas(width, height)
	c.Fill(uint8(clampInt(p.BackgroundBrightness, 0, 255)))

	var sprites *SpriteSet
	if p.SpritesEnabled {
		s, err := LoadSprites(p.SpritesPacks)
		if err != nil {
			return nil, err
		}
		sprites = s
	}

	mode := ModeSourceOver
	for i := 0; i < p.Iterations; i++ {
		if m, ok := RandomItem(g, p.CompositionModes); ok {
			mode = m
		}
		switch g.Integer(0, 5) {
		case 0:
			if !p.RectEnabled {
				break
			}
			drawRect(c, g, p, mode)
		case 1:
			if !p.GridEnabled {
				break
			}
			drawGrid(c, g, p, mode)
		case 2:
			if !p.ColsEnabled {
				break
			}
			drawCols(c, g, p, mode)
		case 3:
			if !p.RowsEnabled {
				break
			}
			drawRows(c, g, p, mode)
		case 4:
			if !p.LinesEnabled {
				break
			}
			drawLines(c, g, p, mode)
		case 5:
			if !p.SpritesEnabled {
				break
			}
			drawSprite(c, g, p, sprites, mode)
		}
	}
	return c, nil
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func drawRect(c *Canvas, g *RNG, p Params, mode CompositionMode) {
	fw, fh := float64(c.W), float64(c.H)
	gray := uint8(clampInt(g.Integer(p.RectBrightness[0], p.RectBrightness[1]), 0, 255))
	alpha := g.Integer(p.RectAlpha[0], p.RectAlpha[1])

	rectW := jsRound(float64(g.Integer(jsRound(fw/16), jsRound(fw/8))) * (float64(p.RectScale) / 100))
	rectH := jsRound(float64(g.Integer(jsRound(fw/16), jsRound(fw/8))) * (float64(p.RectScale) / 100))

	seamless := p.SeamlessTextureEnabled
	xMin, xMax := jsRound(-float64(rectW)/2), jsRound(fw-float64(rectW)/2)
	yMin, yMax := jsRound(-float64(rectH)/2), jsRound(fh-float64(rectH)/2)
	if seamless {
		xMin, xMax = 0, jsRound(fw)
		yMin, yMax = 0, jsRound(fh)
	}
	x := g.Integer(xMin, xMax)
	y := g.Integer(yMin, yMax)

	drawSeamless(c.W, c.H, x, y, rectW, rectH, seamless, func(x, y, rw, rh int) {
		c.FillRect(x, y, rw, rh, gray, alpha, mode)
	})
}

func drawGrid(c *Canvas, g *RNG, p Params, mode CompositionMode) {
	fw, fh := float64(c.W), float64(c.H)
	gray := uint8(clampInt(g.Integer(p.GridBrightness[0], p.GridBrightness[1]), 0, 255))
	alpha := g.Integer(p.GridAlpha[0], p.GridAlpha[1])

	seamless := p.SeamlessTextureEnabled
	x0Min := jsRound(-fw / 16)
	y0Min := jsRound(-fh / 16)
	if seamless {
		x0Min, y0Min = 0, 0
	}
	x0 := g.Integer(x0Min, jsRound(fw))
	y0 := g.Integer(y0Min, jsRound(fh))
	xn := g.Integer(p.GridAmount[0], p.GridAmount[1])
	yn := g.Integer(p.GridAmount[0], p.GridAmount[1])
	scale := float64(p.GridScale) / 100
	gap := float64(p.GridGap) / 100
	size := jsRound(float64(g.Integer(jsRound(fw/256), jsRound(fw/16))) * scale)
	step := size + jsRound(float64(size)*gap)

	x := x0
	for i := 0; i < xn; i++ {
		y := y0
		for j := 0; j < yn; j++ {
			drawSeamless(c.W, c.H, x, y, size, size, seamless, func(x, y, rw, rh int) {
				c.FillRect(x, y, rw, rh, gray, alpha, mode)
			})
			y += step
		}
		x += step
	}
}

func drawCols(c *Canvas, g *RNG, p Params, mode CompositionMode) {
	fw, fh := float64(c.W), float64(c.H)
	gray := uint8(clampInt(g.Integer(p.ColsBrightness[0], p.ColsBrightness[1]), 0, 255))
	alpha := g.Integer(p.ColsAlpha[0], p.ColsAlpha[1])

	seamless := p.SeamlessTextureEnabled
	x0Min := jsRound(-fw / 16)
	y0Min := jsRound(-fh / 16)
	if seamless {
		x0Min, y0Min = 0, 0
	}
	x0 := g.Integer(x0Min, jsRound(fw))
	y0 := g.Integer(y0Min, jsRound(fh))
	xn := g.Integer(p.ColsAmount[0], p.ColsAmount[1])
	scale := float64(p.ColsScale) / 100
	gap := float64(p.ColsGap) / 100
	sizeW := jsRound(float64(g.Integer(jsRound(fw/256), jsRound(fw/16))) * scale)
	sizeH := jsRound(float64(sizeW) * float64(g.Integer(1, 10)))
	step := sizeW + jsRound(float64(sizeW)*gap)

	x := x0
	for i := 0; i < xn; i++ {
		drawSeamless(c.W, c.H, x, y0, sizeW, sizeH, seamless, func(x, y, rw, rh int) {
			c.FillRect(x, y, rw, rh, gray, alpha, mode)
		})
		x += step
	}
}

func drawRows(c *Canvas, g *RNG, p Params, mode CompositionMode) {
	fw, fh := float64(c.W), float64(c.H)
	gray := uint8(clampInt(g.Integer(p.RowsBrightness[0], p.RowsBrightness[1]), 0, 255))
	alpha := g.Integer(p.RowsAlpha[0], p.RowsAlpha[1])

	seamless := p.SeamlessTextureEnabled
	x0Min := jsRound(-fw / 16)
	y0Min := jsRound(-fh / 16)
	if seamless {
		x0Min, y0Min = 0, 0
	}
	x0 := g.Integer(x0Min, jsRound(fw))
	y0 := g.Integer(y0Min, jsRound(fh))
	yn := g.Integer(p.RowsAmount[0], p.RowsAmount[1])
	scale := float64(p.RowsScale) / 100
	gap := float64(p.RowsGap) / 100
	sizeH := jsRound(float64(g.Integer(jsRound(fw/256), jsRound(fw/16))) * scale)
	sizeW := jsRound(float64(sizeH) * float64(g.Integer(1, 10)))
	step := sizeH + jsRound(float64(sizeH)*gap)

	y := y0
	for i := 0; i < yn; i++ {
		drawSeamless(c.W, c.H, x0, y, sizeW, sizeH, seamless, func(x, y, rw, rh int) {
			c.FillRect(x, y, rw, rh, gray, alpha, mode)
		})
		y += step
	}
}

func drawLines(c *Canvas, g *RNG, p Params, mode CompositionMode) {
	fw, fh := float64(c.W), float64(c.H)
	gray := uint8(clampInt(g.Integer(p.LinesBrightness[0], p.LinesBrightness[1]), 0, 255))
	alpha := g.Integer(p.LinesAlpha[0], p.LinesAlpha[1])

	if g.Boolean() {
		// Horizontal line.
		y := g.Integer(jsRound(-fh/16), jsRound(fh))
		thickness := jsRound(float64(g.Integer(p.LinesWidth[0], p.LinesWidth[1])) * (fh / 2500))
		c.FillRect(0, y, c.W, thickness, gray, alpha, mode)
	} else {
		// Vertical line.
		x := g.Integer(jsRound(-fw/16), jsRound(fw))
		thickness := jsRound(float64(g.Integer(p.LinesWidth[0], p.LinesWidth[1])) * (fw / 2500))
		c.FillRect(x, 0, thickness, c.H, gray, alpha, mode)
	}
}

func drawSprite(c *Canvas, g *RNG, p Params, sprites *SpriteSet, mode CompositionMode) {
	if sprites == nil {
		return
	}
	n := sprites.Len()
	if n == 0 {
		// randomItem returns undefined for an empty list and consumes no RNG.
		return
	}
	idx := g.Integer(0, n-1)

	fw, fh := float64(c.W), float64(c.H)
	size := g.Integer(jsRound(fw/32), jsRound(fw/2))

	seamless := p.SeamlessTextureEnabled
	xMin := jsRound(-fw / 16)
	yMin := jsRound(-fh / 16)
	if seamless {
		xMin, yMin = 0, 0
	}
	x := g.Integer(xMin, jsRound(fw))
	y := g.Integer(yMin, jsRound(fh))
	angle := g.Integer(0, 3) * 90

	rot := 0
	if p.SpritesRotationEnabled {
		rot = angle
	}
	img := sprites.Render(idx, size, rot)
	if img == nil {
		return
	}
	drawSeamless(c.W, c.H, x, y, size, size, seamless, func(x, y, rw, rh int) {
		c.DrawImage(img, x, y, mode)
	})
}
