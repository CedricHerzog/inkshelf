//go:build linux && arm

package pocketbook

import (
	"context"
	"fmt"
	"image"
	"log"
	"syscall"
	"time"
	"unsafe"

	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// The framebuffer's ioctls (linux/fb.h), which tell its size and memory layout, and the parts
// of their answers that are read. A row of framebuffer memory (its stride) may be longer than
// its visible width.
const (
	fbioGetVScreenInfo = 0x4600
	fbioGetFScreenInfo = 0x4602

	// varScreenInfoSize is the size of struct fb_var_screeninfo.
	varScreenInfoSize = 160
	// Offsets of int32s in fb_var_screeninfo.
	varScreenInfoXRes         = 0
	varScreenInfoYRes         = 4
	varScreenInfoBitsPerPixel = 24

	// fixScreenInfoSize is the size of struct fb_fix_screeninfo on 32-bit ARM.
	fixScreenInfoSize = 128
	// Offsets of int32s in fb_fix_screeninfo.
	fixScreenInfoMemoryLength = 20 // smem_len
	fixScreenInfoLineLength   = 44 // line_length
)

// framebufferPath is the e-ink panel's framebuffer device.
const framebufferPath = "/dev/fb0"

// openFramebuffer opens and maps the framebuffer, and returns its width in pixels. The app
// restarted by remote control execs itself: close-on-exec keeps it from inheriting this one.
func (reader *readerFirmware) openFramebuffer() (framebufferWidth int, err error) {
	if reader.framebufferFD, err = syscall.Open(framebufferPath, syscall.O_RDWR|syscall.O_CLOEXEC, 0); err != nil {
		return 0, err
	}
	var varScreenInfo [varScreenInfoSize]byte
	if err := ioctl(reader.framebufferFD, fbioGetVScreenInfo, unsafe.Pointer(&varScreenInfo[0])); err != nil {
		return 0, fmt.Errorf("FBIOGET_VSCREENINFO: %w", err)
	}
	var fixScreenInfo [fixScreenInfoSize]byte
	if err := ioctl(reader.framebufferFD, fbioGetFScreenInfo, unsafe.Pointer(&fixScreenInfo[0])); err != nil {
		return 0, fmt.Errorf("FBIOGET_FSCREENINFO: %w", err)
	}
	framebufferWidth = int(int32At(varScreenInfo[:], varScreenInfoXRes))
	reader.framebufferHeight = int(int32At(varScreenInfo[:], varScreenInfoYRes))
	bitsPerPixel := int32At(varScreenInfo[:], varScreenInfoBitsPerPixel)
	memorySize := int(int32At(fixScreenInfo[:], fixScreenInfoMemoryLength))
	reader.framebufferStride = int(int32At(fixScreenInfo[:], fixScreenInfoLineLength))
	if bitsPerPixel != 8 || reader.framebufferStride < framebufferWidth || memorySize < reader.framebufferStride*reader.framebufferHeight {
		return 0, fmt.Errorf("%dx%d at %d bpp, stride %d, %d bytes: not an 8-bit framebuffer",
			framebufferWidth, reader.framebufferHeight, bitsPerPixel, reader.framebufferStride, memorySize)
	}
	reader.framebuffer, err = syscall.Mmap(reader.framebufferFD, 0, memorySize, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return 0, fmt.Errorf("mmap: %w", err)
	}
	return framebufferWidth, nil
}

// flush is what Refresh and FlashRefresh do on the reader: it copies rect of canvas into the
// framebuffer, turning it, then asks the driver to refresh that area of the panel. It logs how
// long each step took, as page turns are timed from the log.
func (reader *readerFirmware) flush(canvas *image.Gray, rect image.Rectangle, waveform Waveform, full bool) {
	if rect.Empty() {
		return
	}
	if !reader.visible() {
		log.Printf("refresh %v skipped: not visible (%s)", rect, reader.describe())
		return
	}
	defer timing.Start(context.Background(), "refresh").End()
	panelRect := framebufferRect(rect, reader.framebufferHeight)
	start := time.Now()
	wait := timing.Start(context.Background(), "wait for the panel")
	reader.waitForOverlappingRefresh(panelRect)
	wait.End()
	copyStart := time.Now()
	copyRotated(reader.framebuffer, reader.framebufferStride, reader.framebufferHeight, canvas, rect)
	sendStart := time.Now()
	reader.marker++
	update := mxcfbUpdate(panelRect, waveform, full, reader.marker)
	if err := ioctl(reader.framebufferFD, mxcfbSendUpdate, unsafe.Pointer(&update[0])); err != nil {
		log.Printf("refresh %v: %v", rect, err)
		return
	}
	if len(reader.running) == maxRunningRefreshes {
		// Forgotten, the oldest could no longer be waited for: wait for it now.
		reader.wait(reader.running[0].marker)
		reader.running = reader.running[1:]
	}
	reader.running = append(reader.running, runningRefresh{rect: panelRect, marker: reader.marker})
	sent := time.Now()
	flash := ""
	if full {
		flash = " full"
	}
	log.Printf("refresh %v %v%s: wait %v, copy %v, send %v", rect, waveform, flash, copyStart.Sub(start).Round(time.Millisecond),
		sendStart.Sub(copyStart).Round(100*time.Microsecond), sent.Sub(sendStart).Round(100*time.Microsecond))
}

// runningRefresh is a refresh sent to the panel, which may still be running.
type runningRefresh struct {
	rect   image.Rectangle // in framebuffer coordinates
	marker uint32
}

// maxRunningRefreshes is how many refreshes flush remembers. A refresh lasts under a second, and
// the app sends at most a few at a time (the reader's two toolbar bars).
const maxRunningRefreshes = 4

// waitForOverlappingRefresh waits for the refreshes still running that overlap panelRect to end,
// so as not to change pixels that the panel is still refreshing. Refreshes elsewhere go on
// meanwhile.
func (reader *readerFirmware) waitForOverlappingRefresh(panelRect image.Rectangle) {
	kept := reader.running[:0]
	for _, refresh := range reader.running {
		if !refresh.rect.Overlaps(panelRect) {
			kept = append(kept, refresh)
			continue
		}
		reader.wait(refresh.marker)
	}
	reader.running = kept
}

// wait waits for the refresh with this marker to end.
func (reader *readerFirmware) wait(marker uint32) {
	// The ioctl fails when the refresh is already over, which is fine: there is nothing to wait
	// for.
	_ = ioctl(reader.framebufferFD, mxcfbWaitForUpdate, unsafe.Pointer(&marker))
}
