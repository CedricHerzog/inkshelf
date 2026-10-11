package fonts

import (
	"image"
	"image/color"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

// glyph is a rasterized glyph: an alpha mask saying how much of each pixel the glyph covers.
// With full hinting, advances and kerning are whole pixels, so a glyph is always drawn at a
// whole pixel and one mask per character is exact.
//
// HOT PATH. Drawing text is blending these cached masks: every line of a book page goes
// through DrawSpaced or DrawGlyphs, then blend. Measure BenchmarkDrawString before and after
// changing them: blend runs for every pixel of every glyph.
type glyph struct {
	// mask is nil if there is nothing to draw, as for a space.
	mask *image.Alpha
	// offset is where the mask's top-left corner is, from the dot.
	offset image.Point
	// advance is how far the dot moves after the glyph.
	advance fixed.Int26_6
}

// DrawTo draws text into dst in gray, with the top-left corner of its line at topLeft. It
// places glyphs as imagefont.Drawer does, but rasterizes each character once and blends it
// straight into dst: imagefont.Drawer rasterizes every glyph on every call, and image/draw has
// no fast path for *image.Gray.
func (font *Font) DrawTo(dst *image.Gray, topLeft image.Point, text string, gray color.Gray) {
	font.DrawSpaced(dst, topLeft, text, gray, 0)
}

// DrawSpaced draws text as DrawTo does, with spacing pixels added after each character (letter
// spacing; negative tightens).
func (font *Font) DrawSpaced(dst *image.Gray, topLeft image.Point, text string, gray color.Gray, spacing int) {
	dotX, dotY := fixed.I(topLeft.X), topLeft.Y+font.ascent
	previous := rune(-1)
	for _, character := range text {
		if previous >= 0 {
			dotX += font.kern(previous, character)
		}
		rasterized := font.glyph(character)
		if rasterized.mask != nil {
			blend(dst, rasterized.mask, image.Pt(dotX.Round(), dotY).Add(rasterized.offset), gray.Y)
		}
		dotX += rasterized.advance + fixed.I(spacing)
		previous = character
	}
}

// glyph returns character rasterized, from the cache, rasterizing it on first use.
func (font *Font) glyph(character rune) glyph {
	if cached, ok := font.glyphs[character]; ok {
		return cached
	}
	// A character missing from the font is cached as nothing to draw (ok is false, rect empty).
	rect, mask, maskPoint, advance, _ := font.face.Glyph(fixed.Point26_6{}, character)
	rasterized := glyph{offset: rect.Min, advance: advance}
	if source, ok := mask.(*image.Alpha); ok && !rect.Empty() {
		// The face reuses its mask for the next glyph, so keep a copy.
		rasterized.mask = image.NewAlpha(image.Rectangle{Max: rect.Size()})
		for row := range rect.Dy() {
			copy(rasterized.mask.Pix[row*rasterized.mask.Stride:][:rect.Dx()], source.Pix[source.PixOffset(maskPoint.X, maskPoint.Y+row):])
		}
	}
	font.glyphs[character] = rasterized
	return rasterized
}

// blend draws gray through mask into dst, with the mask's top-left corner at origin. Each
// pixel moves from its gray toward gray in proportion to the mask's alpha, rounded.
func blend(dst *image.Gray, mask *image.Alpha, origin image.Point, gray uint8) {
	rect := mask.Rect.Add(origin).Intersect(dst.Rect)
	for offset := range rect.Dy() {
		row := rect.Min.Y + offset
		coverage := mask.Pix[mask.PixOffset(rect.Min.X-origin.X, row-origin.Y):][:rect.Dx()]
		pixels := dst.Pix[dst.PixOffset(rect.Min.X, row):][:len(coverage)]
		for index, alpha := range coverage {
			if alpha != 0 {
				pixels[index] = uint8((uint32(pixels[index])*(255-uint32(alpha)) + uint32(gray)*uint32(alpha) + 127) / 255)
			}
		}
	}
}

// DrawGlyphs draws glyphs (from Shape) into dst in gray, with the top-left corner of their
// line at topLeft. Each glyph is rasterized once, as DrawTo's characters are.
func (font *Font) DrawGlyphs(dst *image.Gray, topLeft image.Point, glyphs []Glyph, gray color.Gray) {
	dot := image.Pt(topLeft.X, topLeft.Y+font.ascent)
	for _, shaped := range glyphs {
		rasterized := font.glyphMask(shaped.Index)
		if rasterized.mask != nil {
			blend(dst, rasterized.mask, dot.Add(image.Pt(shaped.X, shaped.Y)).Add(rasterized.offset), gray.Y)
		}
		dot.X += shaped.Advance
	}
}

// glyphMask returns the glyph of index rasterized with its dot at the origin, from the cache.
// Its advance is not set: shaping gives it.
func (font *Font) glyphMask(index sfnt.GlyphIndex) glyph {
	if cached, ok := font.masks[index]; ok {
		return cached
	}
	var rasterized glyph
	segments, err := font.file.parsed.LoadGlyph(&font.buffer, index, fixed.I(font.size), nil)
	if bounds := segments.Bounds(); err == nil && len(segments) > 0 {
		rect := image.Rect(bounds.Min.X.Floor(), bounds.Min.Y.Floor(), bounds.Max.X.Ceil(), bounds.Max.Y.Ceil())
		if !rect.Empty() {
			rasterized = glyph{mask: font.rasterize(segments, rect), offset: rect.Min}
		}
	}
	font.masks[index] = rasterized
	return rasterized
}

// rasterize draws segments, whose bounds rect holds, into a mask of rect's size.
func (font *Font) rasterize(segments sfnt.Segments, rect image.Rectangle) *image.Alpha {
	if font.rasterizer == nil {
		font.rasterizer = vector.NewRasterizer(rect.Dx(), rect.Dy())
	}
	rasterizer := font.rasterizer
	rasterizer.Reset(rect.Dx(), rect.Dy())
	point := func(at fixed.Point26_6) (float32, float32) {
		return float32(at.X)/64 - float32(rect.Min.X), float32(at.Y)/64 - float32(rect.Min.Y)
	}
	for _, segment := range segments {
		switch segment.Op {
		case sfnt.SegmentOpMoveTo:
			rasterizer.MoveTo(point(segment.Args[0]))
		case sfnt.SegmentOpLineTo:
			rasterizer.LineTo(point(segment.Args[0]))
		case sfnt.SegmentOpQuadTo:
			controlX, controlY := point(segment.Args[0])
			endX, endY := point(segment.Args[1])
			rasterizer.QuadTo(controlX, controlY, endX, endY)
		case sfnt.SegmentOpCubeTo:
			firstX, firstY := point(segment.Args[0])
			secondX, secondY := point(segment.Args[1])
			endX, endY := point(segment.Args[2])
			rasterizer.CubeTo(firstX, firstY, secondX, secondY, endX, endY)
		}
	}
	mask := image.NewAlpha(image.Rectangle{Max: rect.Size()})
	rasterizer.Draw(mask, mask.Bounds(), image.Opaque, image.Point{})
	return mask
}
