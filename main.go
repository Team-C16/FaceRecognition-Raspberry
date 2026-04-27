// main.go — Main entry point for Raspberry Pi face detection in Go.
// Mirrors raspberry_pi/main.py exactly.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gocv.io/x/gocv"
)

func runWebSocketMode(display bool) {
	fmt.Println("\n[Main] Starting face detection loop (WebSocket mode)")
	fmt.Println("[Main] Press Ctrl+C to stop\n")

	camera, err := NewCamera(CameraIndex, CameraWidth, CameraHeight, CameraFPS)
	if err != nil {
		fmt.Printf("Camera error: %v\n", err)
		return
	}
	defer camera.Release()

	detector := NewFaceDetector()
	sender := NewWebSocketSender()
	if err := sender.Connect(); err != nil {
		fmt.Printf("Initial WebSocket connection failed: %v\n", err)
		// We don't abort; it will auto-reconnect on next send.
	}
	defer sender.Close()

	var window *gocv.Window
	if display {
		window = gocv.NewWindow("Face Detection (Go)")
		defer window.Close()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	lastSendTime := time.Time{}
	frameCount := 0
	fpsStart := time.Now()

	for {
		select {
		case <-sigChan:
			fmt.Println("\n[Main] Stopping...")
			return
		default:
		}

		frame, ok := camera.Read()
		if !ok || frame.Empty() {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		faces := detector.Detect(frame)
		frameCount++
		frameH := frame.Rows()

		// Calculate FPS every 30 frames
		if frameCount%30 == 0 {
			elapsed := time.Since(fpsStart).Seconds()
			fps := float64(frameCount) / elapsed
			fmt.Printf("[Main] FPS: %.1f | Faces detected: %d\n", fps, len(faces))
		}

		now := time.Now()

		// Send interval check
		if now.Sub(lastSendTime).Seconds() >= SendIntervalSeconds {
			if len(faces) == 0 {
				// ── RULE 4: Ghost Blink Fix ──
				// Notify backend to reset counter
				_, err := sender.SendNoFace()
				if err != nil {
					fmt.Printf("[WebSocket] SendNoFace error: %v\n", err)
				}
				lastSendTime = now
			} else {
				// Send faces
				faceCount := len(faces)
				for _, f := range faces {
					result, err := sender.SendFace(f.FaceImg, [4]int{f.Box.Min.X, f.Box.Min.Y, f.Box.Max.X, f.Box.Max.Y}, f.Confidence, frameH, faceCount)
					if err != nil {
						fmt.Printf("[WebSocket] SendFace error: %v\n", err)
					} else if result != nil {
						fmt.Printf("  → %s | %s (score: %.3f) | validated: %v\n", result.Label, result.Name, result.Score, result.IsValidated)
					}
				}
				lastSendTime = now
			}
		}

		// Cleanup cropped face mats
		for _, f := range faces {
			f.FaceImg.Close()
		}

		// Display logic
		if display && window != nil {
			displayFrame := frame.Clone()
			detector.DrawDetections(displayFrame, faces)
			window.IMShow(displayFrame)
			if window.WaitKey(1) == 'q' {
				displayFrame.Close()
				break
			}
			displayFrame.Close()
		}

		frame.Close()
		
		// Small sleep to prevent 100% CPU on fast cameras
		time.Sleep(10 * time.Millisecond)
	}
}

func runHTTPMode(display bool) {
	fmt.Println("\n[Main] Starting face detection loop (HTTP mode)")
	fmt.Println("[Main] Press Ctrl+C to stop\n")

	camera, err := NewCamera(CameraIndex, CameraWidth, CameraHeight, CameraFPS)
	if err != nil {
		fmt.Printf("Camera error: %v\n", err)
		return
	}
	defer camera.Release()

	detector := NewFaceDetector()
	sender := NewHTTPSender()

	var window *gocv.Window
	if display {
		window = gocv.NewWindow("Face Detection (Go - HTTP)")
		defer window.Close()
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	lastSendTime := time.Time{}
	frameCount := 0
	fpsStart := time.Now()

	for {
		select {
		case <-sigChan:
			fmt.Println("\n[Main] Stopping...")
			return
		default:
		}

		frame, ok := camera.Read()
		if !ok || frame.Empty() {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		faces := detector.Detect(frame)
		frameCount++

		if frameCount%30 == 0 {
			elapsed := time.Since(fpsStart).Seconds()
			fps := float64(frameCount) / elapsed
			fmt.Printf("[Main] FPS: %.1f | Faces detected: %d\n", fps, len(faces))
		}

		now := time.Now()

		if len(faces) > 0 && now.Sub(lastSendTime).Seconds() >= SendIntervalSeconds {
			for _, f := range faces {
				result, err := sender.SendFace(f.FaceImg, [4]int{f.Box.Min.X, f.Box.Min.Y, f.Box.Max.X, f.Box.Max.Y}, f.Confidence)
				if err != nil {
					fmt.Printf("[HTTP] Error: %v\n", err)
				} else if result != nil {
					fmt.Printf("  → Recognized: %s (score: %.3f)\n", result.Name, result.Score)
				}
			}
			lastSendTime = now
		}

		for _, f := range faces {
			f.FaceImg.Close()
		}

		if display && window != nil {
			displayFrame := frame.Clone()
			detector.DrawDetections(displayFrame, faces)
			window.IMShow(displayFrame)
			if window.WaitKey(1) == 'q' {
				displayFrame.Close()
				break
			}
			displayFrame.Close()
		}

		frame.Close()
		time.Sleep(10 * time.Millisecond)
	}
}

func main() {
	useHTTP := flag.Bool("http", false, "Use HTTP instead of WebSocket for backend communication")
	display := flag.Bool("display", false, "Show video feed with detection overlays (requires display)")
	flag.Parse()

	if *useHTTP {
		runHTTPMode(*display)
	} else {
		runWebSocketMode(*display)
	}
}
