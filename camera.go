// camera.go — Camera capture with auto-reconnect.
// Mirrors raspberry_pi/camera.py exactly.
package main

import (
	"fmt"
	"time"

	"gocv.io/x/gocv"
)

// Camera wraps a GoCV VideoCapture with auto-reconnect logic.
type Camera struct {
	index  int
	width  int
	height int
	fps    int
	cap    *gocv.VideoCapture
}

// NewCamera opens the camera at the given device index.
// Panics if the camera cannot be opened on first try.
func NewCamera(index, width, height, fps int) (*Camera, error) {
	c := &Camera{
		index:  index,
		width:  width,
		height: height,
		fps:    fps,
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

// connect (re)opens the VideoCapture and applies camera properties.
func (c *Camera) connect() error {
	if c.cap != nil {
		c.cap.Close()
	}

	fmt.Printf("[Camera] Connecting to camera %d...\n", c.index)
	cap, err := gocv.OpenVideoCapture(c.index)
	if err != nil {
		return fmt.Errorf("[Camera] cannot open camera %d: %w", c.index, err)
	}
	if !cap.IsOpened() {
		return fmt.Errorf("[Camera] camera %d reported not opened", c.index)
	}

	cap.Set(gocv.VideoCaptureFrameWidth, float64(c.width))
	cap.Set(gocv.VideoCaptureFrameHeight, float64(c.height))
	cap.Set(gocv.VideoCaptureFPS, float64(c.fps))

	actualW := cap.Get(gocv.VideoCaptureFrameWidth)
	actualH := cap.Get(gocv.VideoCaptureFrameHeight)
	actualFPS := cap.Get(gocv.VideoCaptureFPS)
	fmt.Printf("[Camera] Connected: %.0fx%.0f @ %.0ffps\n", actualW, actualH, actualFPS)

	c.cap = cap
	return nil
}

// Read captures one frame. Returns (frame, true) on success.
// On failure it attempts one reconnect; if that also fails it returns (zero Mat, false).
func (c *Camera) Read() (gocv.Mat, bool) {
	if c.cap == nil || !c.cap.IsOpened() {
		fmt.Println("[Camera] Not connected, attempting reconnect...")
		if err := c.connect(); err != nil {
			fmt.Println("[Camera]", err)
			return gocv.NewMat(), false
		}
	}

	frame := gocv.NewMat()
	ok := c.cap.Read(&frame)
	if !ok {
		fmt.Println("[Camera] Frame read failed, attempting reconnect...")
		frame.Close()
		time.Sleep(time.Second)
		if err := c.connect(); err != nil {
			fmt.Println("[Camera]", err)
			return gocv.NewMat(), false
		}
		return gocv.NewMat(), false
	}

	return frame, true
}

// Release closes the underlying VideoCapture.
func (c *Camera) Release() {
	if c.cap != nil {
		c.cap.Close()
		c.cap = nil
		fmt.Println("[Camera] Released")
	}
}
