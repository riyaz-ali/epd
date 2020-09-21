package main

import (
	"github.com/fogleman/gg"
	"github.com/stianeikeland/go-rpio/v4"
	"go.riyazali.net/epd"
	"image/color"
	"log"
)

func init() {
	//start the GPIO controller
	if err := rpio.Open(); err != nil {
		log.Fatalf("[FATAL] failed to start gpio: %v", err)
	}

	// Enable SPI on SPI0
	if err := rpio.SpiBegin(rpio.Spi0); err != nil {
		log.Fatalf("[FATAL] failed to enable SPI: %v", err)
	}

	// configure SPI settings
	rpio.SpiSpeed(4_000_000)
	rpio.SpiMode(0, 0)

	rpio.Pin(17).Mode(rpio.Output)
	rpio.Pin(25).Mode(rpio.Output)
	rpio.Pin(8).Mode(rpio.Output)
	rpio.Pin(24).Mode(rpio.Input)
}

func main() {
	defer rpio.Close()

	// initialize the driver
	var display = epd.New(rpio.Pin(17), rpio.Pin(25), rpio.Pin(8), ReadablePinPatch{rpio.Pin(24)}, rpio.SpiTransmit)
	display.Mode(epd.PartialUpdate)

	// create an image canvas and draw on it
	var img = gg.NewContext(display.Width, display.Height)
	img.SetColor(color.White)
	img.Clear()

	var cx, cy = float64(display.Width) / 2, float64(display.Height) / 2

	var s1 = "hello"
	var hs1, _ = img.MeasureString(s1)
	var s2 = "world"
	var hs2, ws2 = img.MeasureString(s2)

	img.SetColor(color.Black)
	img.DrawRectangle(cx-(hs2/2)-4, cy-(ws2/2)-6, hs2+8, ws2+6)
	img.Fill()

	img.SetColor(color.Black)
	img.DrawString(s1, cx-(hs1/2), cy-ws2-8)
	img.Stroke()

	img.SetColor(color.White)
	img.DrawString(s2, cx-(hs2/2), cy)
	img.Stroke()

	if e := display.Draw(img.Image()); e != nil {
		log.Printf("[ERROR] failed to draw: %v\n", e)
		display.Clear(color.White)
	}

	display.Sleep()
}

type ReadablePinPatch struct { rpio.Pin }

func (pin ReadablePinPatch) Read() uint8 { return uint8(pin.Pin.Read()) }
