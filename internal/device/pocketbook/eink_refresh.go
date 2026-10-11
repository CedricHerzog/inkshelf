package pocketbook

import "image"

// Waveform is the sequence of voltages the panel drives its pixels with to change them, as
// numbered by the firmware (waveform_type_t in hwconfig.h). Faster waveforms show fewer grays
// or leave more ghosting: see the package documentation.
type Waveform int

// The waveforms of the InkPad 3's panel.
const (
	DU   Waveform = 1 // black and white only, fast
	GC16 Waveform = 2 // 16 grays, every pixel driven; libinkview's PartialUpdate and FullUpdate
	GC4  Waveform = 3 // 4 grays
	A2   Waveform = 4 // black and white only, fastest, leaves the most ghosting
	GL16 Waveform = 5 // 16 grays, for text on white; libinkview's PartialUpdateHQ
	DU4  Waveform = 8 // 4 grays, fast
)

// FlashRefresh shows the whole canvas on the panel with a full GC16 refresh: the panel flashes,
// and the ghosts of earlier pages are gone.
func FlashRefresh() {
	if current != nil {
		current.firmware.flush(current.canvas, current.canvas.Rect, GC16, true)
	}
}

// Refresh shows rect of the canvas on the panel with waveform, without a flash: only the
// pixels that changed are driven.
func Refresh(rect image.Rectangle, waveform Waveform) {
	if current != nil {
		current.firmware.flush(current.canvas, rect.Intersect(current.canvas.Rect), waveform, false)
	}
}

// The panel's driver, PocketBook's epdc.ko, offers the MXCFB ioctls of the i.MX6's e-ink
// driver (mxcfb.h) on /dev/fb0, although the InkPad 3 is not an i.MX6: a refresh is an ioctl
// on the framebuffer, given the rectangle and the waveform.
const (
	// mxcfbSendUpdate starts a refresh, described by an mxcfbUpdate, and returns at once.
	mxcfbSendUpdate = 0x4040462e
	// mxcfbWaitForUpdate waits until the refresh whose marker it is given is over.
	mxcfbWaitForUpdate = 0x4004462f
)

// The fields of the mxcfb_update_data that mxcfbUpdate fills.
const (
	// mxcfbUpdateSize is the size of mxcfb_update_data. Its last bytes describe an alternate
	// buffer, which is not used.
	mxcfbUpdateSize = 64
	// updateModePartial drives only the pixels that changed.
	updateModePartial = 0
	// updateModeFull drives every pixel of the rectangle, with a flash.
	updateModeFull = 1
	// tempUseAmbient lets the driver adjust the waveform to the panel's temperature: e-ink
	// particles move slower in the cold.
	tempUseAmbient = 0x1000
)

// mxcfbUpdate encodes the mxcfb_update_data of MXCFB_SEND_UPDATE for the framebuffer
// rectangle rect: {top, left, width, height}, waveform, update mode, marker, temperature,
// then flags and the alternate buffer, left zero. The marker names the refresh, for
// mxcfbWaitForUpdate.
func mxcfbUpdate(rect image.Rectangle, waveform Waveform, full bool, marker uint32) [mxcfbUpdateSize]byte {
	var data [mxcfbUpdateSize]byte
	mode := int32(updateModePartial)
	if full {
		mode = updateModeFull
	}
	putInt32s(data[:], int32(rect.Min.Y), int32(rect.Min.X), int32(rect.Dx()), int32(rect.Dy()),
		int32(waveform), mode, int32(marker), tempUseAmbient)
	return data
}
