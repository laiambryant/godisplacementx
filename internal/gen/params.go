package gen

// Dual is a [min, max] pair, mirroring the NumberDual type used by dual-range
// sliders in the original app.
type Dual [2]int

// SpritesPack identifies one of the bundled sprite packs.
type SpritesPack string

const (
	PackClassic   SpritesPack = "classic"
	PackBigdata   SpritesPack = "bigdata"
	PackAggromaxx SpritesPack = "aggromaxx"
	PackCrappack  SpritesPack = "crappack"
)

// spritePackCounts mirrors getSprites() in store.ts: how many numbered SVGs
// each pack contains, in the order they are added.
var spritePackOrder = []SpritesPack{PackClassic, PackBigdata, PackAggromaxx, PackCrappack}
var spritePackCounts = map[SpritesPack]int{
	PackClassic:   17,
	PackBigdata:   5,
	PackAggromaxx: 12,
	PackCrappack:  27,
}

// Params is the full set of generator settings. JSON tags match the original
// zustand store field names so the GUI/CLI can exchange the same shape.
type Params struct {
	Iterations           int  `json:"iterations"`
	BackgroundBrightness int  `json:"backgroundBrightness"`
	RectEnabled          bool `json:"rectEnabled"`
	RectBrightness       Dual `json:"rectBrightness"`
	RectAlpha            Dual `json:"rectAlpha"`
	RectScale            int  `json:"rectScale"`
	GridEnabled          bool `json:"gridEnabled"`
	GridBrightness       Dual `json:"gridBrightness"`
	GridAlpha            Dual `json:"gridAlpha"`
	GridScale            int  `json:"gridScale"`
	GridAmount           Dual `json:"gridAmount"`
	GridGap              int  `json:"gridGap"`
	ColsEnabled          bool `json:"colsEnabled"`
	ColsBrightness       Dual `json:"colsBrightness"`
	ColsAlpha            Dual `json:"colsAlpha"`
	ColsScale            int  `json:"colsScale"`
	ColsAmount           Dual `json:"colsAmount"`
	ColsGap              int  `json:"colsGap"`
	RowsEnabled          bool `json:"rowsEnabled"`
	RowsBrightness       Dual `json:"rowsBrightness"`
	RowsAlpha            Dual `json:"rowsAlpha"`
	RowsScale            int  `json:"rowsScale"`
	RowsAmount           Dual `json:"rowsAmount"`
	RowsGap              int  `json:"rowsGap"`
	LinesEnabled         bool `json:"linesEnabled"`
	LinesBrightness      Dual `json:"linesBrightness"`
	LinesAlpha           Dual `json:"linesAlpha"`
	LinesWidth           Dual `json:"linesWidth"`

	SpritesEnabled         bool              `json:"spritesEnabled"`
	SpritesPacks           []SpritesPack     `json:"spritesPacks"`
	SpritesRotationEnabled bool              `json:"spritesRotationEnabled"`
	SeamlessTextureEnabled bool              `json:"seamlessTextureEnabled"`
	CompositionModes       []CompositionMode `json:"compositionModes"`
}

// Range definitions mirror src/components/pages/Generator/constants.ts.
type intRange struct {
	Min, Max, Default, Step int
}
type dualRange struct {
	Min, Max int
	Default  Dual
	Step     int
}

var (
	rIterations           = intRange{Min: 10, Max: 2000, Default: 100, Step: 1}
	rBackgroundBrightness = intRange{Min: 0, Max: 255, Default: 32, Step: 1}
	rBrightness           = dualRange{Min: 0, Max: 255, Default: Dual{0, 255}, Step: 1}
	rAlpha50to100         = dualRange{Min: 0, Max: 100, Default: Dual{50, 100}, Step: 1}
	rAlpha80to100         = dualRange{Min: 0, Max: 100, Default: Dual{80, 100}, Step: 1}
	rScale20to200         = intRange{Min: 20, Max: 200, Default: 100, Step: 1}
	rAmount2to10          = dualRange{Min: 2, Max: 10, Default: Dual{2, 5}, Step: 1}
	rGap                  = intRange{Min: 10, Max: 1000, Default: 100, Step: 10}
	rLinesWidth           = dualRange{Min: 1, Max: 50, Default: Dual{5, 10}, Step: 1}
)

// allCompositionModes lists every mode in the same order as randCompositionModes
// in the original store.ts.
var allCompositionModes = []CompositionMode{
	ModeColorBurn, ModeColorDodge, ModeDarken, ModeDifference, ModeExclusion,
	ModeHardLight, ModeLighten, ModeLighter, ModeLuminosity, ModeMultiply,
	ModeOverlay, ModeScreen, ModeSoftLight, ModeSourceAtop, ModeSourceOver, ModeXor,
}

// Default returns the initial settings, mirroring the zustand store defaults.
func Default() Params {
	return Params{
		Iterations:           rIterations.Default,
		BackgroundBrightness: rBackgroundBrightness.Default,
		RectEnabled:          true,
		RectBrightness:       rBrightness.Default,
		RectAlpha:            rAlpha50to100.Default,
		RectScale:            rScale20to200.Default,
		GridEnabled:          true,
		GridBrightness:       rBrightness.Default,
		GridAlpha:            rAlpha80to100.Default,
		GridScale:            rScale20to200.Default,
		GridAmount:           rAmount2to10.Default,
		GridGap:              rGap.Default,
		ColsEnabled:          true,
		ColsBrightness:       rBrightness.Default,
		ColsAlpha:            rAlpha80to100.Default,
		ColsScale:            rScale20to200.Default,
		ColsAmount:           rAmount2to10.Default,
		ColsGap:              rGap.Default,
		RowsEnabled:          true,
		RowsBrightness:       rBrightness.Default,
		RowsAlpha:            rAlpha80to100.Default,
		RowsScale:            rScale20to200.Default,
		RowsAmount:           rAmount2to10.Default,
		RowsGap:              rGap.Default,
		LinesEnabled:         true,
		LinesBrightness:      rBrightness.Default,
		LinesAlpha:           rAlpha80to100.Default,
		LinesWidth:           rLinesWidth.Default,

		SpritesEnabled:         false,
		SpritesPacks:           []SpritesPack{PackClassic},
		SpritesRotationEnabled: true,
		SeamlessTextureEnabled: false,
		CompositionModes:       []CompositionMode{ModeSourceOver},
	}
}

// RandomItem mirrors randomItem<T>: returns a random element, or ok=false when
// the slice is empty.
func RandomItem[T any](g *RNG, items []T) (T, bool) {
	var zero T
	if len(items) == 0 {
		return zero, false
	}
	return items[g.Integer(0, len(items)-1)], true
}

func randSetting(g *RNG, r intRange) int { return g.Integer(r.Min, r.Max) }

func randDualSetting(g *RNG, r dualRange) Dual {
	return Dual{g.Integer(r.Min, r.Max), g.Integer(r.Min, r.Max)}
}

func randSpritesPacks(g *RNG) []SpritesPack {
	packs := []SpritesPack{}
	for _, p := range spritePackOrder {
		if g.Boolean() {
			packs = append(packs, p)
		}
	}
	return packs
}

func randCompositionModes(g *RNG) []CompositionMode {
	modes := []CompositionMode{}
	for _, m := range allCompositionModes {
		if g.Boolean() {
			modes = append(modes, m)
		}
	}
	return modes
}

// Randomize mirrors the store's randomize() action: it randomizes every layer's
// parameters and enabled flags. The RNG call order matches the original.
func (p *Params) Randomize(g *RNG) {
	p.Iterations = randSetting(g, rIterations)
	p.BackgroundBrightness = randSetting(g, rBackgroundBrightness)
	p.RectEnabled = g.Boolean()
	p.RectBrightness = randDualSetting(g, rBrightness)
	p.RectAlpha = randDualSetting(g, rAlpha50to100)
	p.RectScale = randSetting(g, rScale20to200)
	p.GridEnabled = g.Boolean()
	p.GridBrightness = randDualSetting(g, rBrightness)
	p.GridAlpha = randDualSetting(g, rAlpha80to100)
	p.GridScale = randSetting(g, rScale20to200)
	p.GridAmount = randDualSetting(g, rAmount2to10)
	p.GridGap = randSetting(g, rGap)
	p.ColsEnabled = g.Boolean()
	p.ColsBrightness = randDualSetting(g, rBrightness)
	p.ColsAlpha = randDualSetting(g, rAlpha80to100)
	p.ColsScale = randSetting(g, rScale20to200)
	p.ColsAmount = randDualSetting(g, rAmount2to10)
	p.ColsGap = randSetting(g, rGap)
	p.RowsEnabled = g.Boolean()
	p.RowsBrightness = randDualSetting(g, rBrightness)
	p.RowsAlpha = randDualSetting(g, rAlpha80to100)
	p.RowsScale = randSetting(g, rScale20to200)
	p.RowsAmount = randDualSetting(g, rAmount2to10)
	p.RowsGap = randSetting(g, rGap)
	p.LinesEnabled = g.Boolean()
	p.LinesBrightness = randDualSetting(g, rBrightness)
	p.LinesAlpha = randDualSetting(g, rAlpha80to100)
	p.LinesWidth = randDualSetting(g, rLinesWidth)
	p.SpritesEnabled = g.Boolean()
	p.SpritesPacks = randSpritesPacks(g)
	p.SpritesRotationEnabled = g.Boolean()
	p.CompositionModes = randCompositionModes(g)
}
