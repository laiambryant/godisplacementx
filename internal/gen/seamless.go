package gen

// drawSeamless mirrors the original drawSeamless helper: when seamless tiling is
// enabled it normalizes the position into the canvas and repeats the draw across
// the wrap-around edges; otherwise it draws once.
func drawSeamless(canvasW, canvasH, x, y, rectW, rectH int, seamless bool, draw func(x, y, rectW, rectH int)) {
	if !seamless {
		draw(x, y, rectW, rectH)
		return
	}
	for x+rectW > canvasW {
		x -= canvasW
	}
	for y+rectH > canvasH {
		y -= canvasH
	}
	for ox := 0; x+ox <= canvasW; ox += canvasW {
		for oy := 0; y+oy <= canvasH; oy += canvasH {
			draw(x+ox, y+oy, rectW, rectH)
		}
	}
}
