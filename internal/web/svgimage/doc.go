// Package svgimage sizes and draws SVG images. EPUB 3 counts SVG among its core image types,
// with JPEG, PNG, GIF and WebP: books use it for covers that wrap a raster <image> in an <svg>,
// and for vector illustrations and ornaments.
//
// An SVG is drawn in these steps:
//
//   - The tree: the markup is read whole into nodes (tree.go), leniently, within limits on its
//     size and depth.
//   - The cascade: each element's style is computed, as in a browser, from its presentation
//     attributes (fill="red"), the CSS of the chapter it is inline in
//     (DocumentStyleAttribute), its own <style> sheets and its style attribute (cascade.go).
//   - The serialization: shapes, paths and gradients are drawn by github.com/srwiley/oksvg,
//     which reads only a part of SVG. So the tree is written back with each element's style in
//     forms oksvg reads, and without what it would draw wrong (serialize.go, paint.go).
//   - What oksvg cannot draw is drawn here: the raster <image>s under the shapes
//     (rasterize.go), pattern fills tiled between layers of shapes (pattern.go), and text last,
//     through a TextDrawer given by the caller (text.go).
//
// Filters, masks, clip paths, markers, nested <svg>s and the transforms of an <image>'s
// parents are left out.
//
// Coordinates are in user units, the SVG's own, until a transform maps them to the pixels of
// the image drawn.
package svgimage
