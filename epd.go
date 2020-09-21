// Package epd provides driver for Waveshare's E-paper e-ink display
package epd  // import "go.riyazali.net/epd"

import (
	"errors"
	"image"
	"image/color"
	"math"
	"time"
)

// ErrInvalidImageSize is returned if the given image bounds doesn't fit into display bounds
var ErrInvalidImageSize = errors.New("invalid image size")

// LookupTable defines a type holding the instruction lookup table
// This lookup table is used by the device when performing refreshes
type Mode uint8

// WriteablePin is a GPIO pin through which the driver can write digital data
type WriteablePin interface {
	// High sets the pins output to digital high
	High()

	// Low sets the pins output to digital low
	Low()
}

// ReadablePin is a GPIO pin through which the driver can read digital data
type ReadablePin interface {
	// Read reads from the pin and return the data as a byte
	Read() uint8
}

// Transmit is a function that sends the data payload across to the device via the SPI line
type Transmit func(data ...byte)

const (
	FullUpdate Mode = iota
	PartialUpdate
)

// fullUpdate is a lookup table used whilst in full update mode
var fullUpdate = []byte{
	0x50, 0xAA, 0x55, 0xAA, 0x11, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0xFF, 0xFF, 0x1F, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

// partialUpdate is a lookup table used whilst in partial update mode
var partialUpdate = []byte{
	0x10, 0x18, 0x18, 0x08, 0x18, 0x18,
	0x08, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x13, 0x14, 0x44, 0x12,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

// EPD defines the base type for the e-paper display driver
type EPD struct {
	// dimensions of the display
	Height int
	Width  int

	// pins used by this driver
	rst  WriteablePin // for reset signal
	dc   WriteablePin // for data/command select signal; D=HIGH C=LOW
	cs   WriteablePin // for chip select signal; this pin is active low
	busy ReadablePin  // for reading in busy signal

	// SPI transmitter
	transmit Transmit
}

// New creates a new EPD device driver
func New(rst, dc, cs WriteablePin, busy ReadablePin, transmit Transmit) *EPD {
	return &EPD{296, 128, rst, dc, cs, busy, transmit}
}

// reset resets the display back to defaults
func (epd *EPD) reset() {
	epd.rst.High()
	time.Sleep(200 * time.Millisecond)
	epd.rst.Low()
	time.Sleep(10 * time.Millisecond)
	epd.rst.High()
	time.Sleep(200 * time.Millisecond)
}

// command transmits single byte of command instruction over the SPI line
func (epd *EPD) command(c byte) {
	epd.dc.Low()
	epd.cs.Low()
	epd.transmit(c)
	epd.cs.High()
}

// data transmits single byte of data payload over SPI line
func (epd *EPD) data(d byte) {
	epd.dc.High()
	epd.cs.Low()
	epd.transmit(d)
	epd.cs.High()
}

// idle reads from busy line and waits for the device to get into idle state
func (epd *EPD) idle() {
	for epd.busy.Read() == 0x1 {
		time.Sleep(200 * time.Millisecond)
	}
}

// mode sets the device's mode (based on the LookupTable)
// The device can either be in FullUpdate mode where the whole display is updated each time an image is rendered
// or in PartialUpdate mode where only the changed section is updated (and it doesn't cause any flicker)
//
// Waveshare recommends doing full update of the display at least once per-day to prevent ghost image problems
func (epd *EPD) Mode(mode Mode) {
	epd.reset()

	// command+data below is taken from the python sample driver

	// DRIVER_OUTPUT_CONTROL
	epd.command(0x01)
	epd.data(byte((epd.Height - 1) & 0xFF))
	epd.data(byte(((epd.Height - 1) >> 8) & 0xFF))
	epd.data(0x00)

	// BOOSTER_SOFT_START_CONTROL
	epd.command(0x0C)
	epd.data(0xD7)
	epd.data(0xD6)
	epd.data(0x9D)

	// WRITE_VCOM_REGISTER
	epd.command(0x2C)
	epd.data(0xA8)

	// SET_DUMMY_LINE_PERIOD
	epd.command(0x3A)
	epd.data(0x1A)

	// SET_GATE_TIME
	epd.command(0x3B)
	epd.data(0x08)

	// DATA_ENTRY_MODE_SETTING
	epd.command(0x11)
	epd.data(0x03)

	// WRITE_LUT_REGISTER
	epd.command(0x32)
	var lut = fullUpdate
	if mode == PartialUpdate {
		lut = partialUpdate
	}
	for _, b := range lut {
		epd.data(b)
	}
}

// Sleep puts the device into "deep sleep" mode where it draws zero (0) current
//
// Waveshare recommends putting the device in "deep sleep" mode (or disconnect from power)
// if doesn't need updating/refreshing.
func (epd *EPD) Sleep() {
	epd.command(0x10)
	epd.data(0x01)
}

// turnOnDisplay activates the display and renders the image that's there in the device's RAM
func (epd *EPD) turnOnDisplay() {
	epd.command(0x22)
	epd.data(0xC4)
	epd.command(0x20)
	epd.command(0xFF)
	epd.idle()
}

// window sets the window plane used by device when drawing the image in the buffer
func (epd *EPD) window(x0, x1 byte, y0, y1 uint16) {
	epd.command(0x44)
	epd.data((x0 >> 3) & 0xFF)
	epd.data((x1 >> 3) & 0xFF)

	epd.command(0x45)
	epd.data(byte(y0 & 0xFF))
	epd.data(byte((y0 >> 8) & 0xFF))
	epd.data(byte(y1 & 0xFF))
	epd.data(byte((y1 >> 8) & 0xFF))
}

// cursor sets the cursor position in the device window frame
func (epd *EPD) cursor(x uint8, y uint16) {
	epd.command(0x4E)
	epd.data((x >> 3) & 0xFF)

	epd.command(0x4F)
	epd.data(byte(y & 0xFF))
	epd.data(byte((y >> 8) & 0xFF))

	epd.idle()
}

// Clear clears the display and paints the whole display into c color
func (epd *EPD) Clear(c color.Color) {
	var img = image.White
	if c != color.White {
		img = image.Black // anything other than white is treated as black
	}
	_ = epd.Draw(img)
}

// Draw renders the given image onto the display
func (epd *EPD) Draw(img image.Image) error {
	var isvertical = img.Bounds().Size().X == epd.Width && img.Bounds().Size().Y == epd.Height
	var _, uniform = img.(*image.Uniform) // special case for uniform images which have infinite bound
	if !uniform && !isvertical {
		return ErrInvalidImageSize
	}

	epd.window(0, byte(epd.Width-1), 0, uint16(epd.Height-1))
	for i := 0; i < epd.Height; i++ {
		epd.cursor(0, uint16(i))
		epd.command(0x24) // WRITE_RAM
		for j := 0; j < epd.Width; j += 8 {
			// this loop converts individual pixels into a single byte
			// 8-pixels at a time and then sends that byte to render
			var b = 0xFF
			for px := 0; px < 8; px++ {
				var pixel = img.At(j+px, i)
				if isdark(pixel.RGBA()) {
					b &= ^(0x80 >> (px % 8))
				}
			}
			epd.data(byte(b))
		}
	}
	epd.turnOnDisplay()
	return nil
}

// isdark is a utility method which returns true if the pixel color is considered dark else false
// this function is taken from https://git.io/JviWg
func isdark(r, g, b, _ uint32) bool {
	return math.Sqrt(
		0.299*math.Pow(float64(r), 2)+
			0.587*math.Pow(float64(g), 2)+
			0.114*math.Pow(float64(b), 2)) <= 130
}
