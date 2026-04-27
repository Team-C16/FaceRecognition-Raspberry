// config.go — Raspberry Pi Go Client Configuration
// Mirrors raspberry_pi/config.py exactly.
package main

// ── Backend ─────────────────────────────────────────────────
const (
	BackendHost  = "127.0.0.1"
	BackendPort  = 8000
	BackendWSURL = "ws://" + BackendHost + ":8000/ws"
	BackendHTTPURL = "http://" + BackendHost + ":8000"
)

// ── Camera ──────────────────────────────────────────────────
const (
	CameraIndex  = 0
	CameraWidth  = 640
	CameraHeight = 480
	CameraFPS    = 30
)

// ── Face Detection (Haar Cascade) ───────────────────────────
const (
	// ScaleFactor: how much image is reduced at each scale.
	// 1.1 = accurate/slow, 1.3 = fast/less accurate
	DetectionScaleFactor  = 1.2
	DetectionMinNeighbors = 5
	MinFaceSize           = 60 // pixels
)

// ── Processing ──────────────────────────────────────────────
const (
	// Minimum seconds between sending frames to the backend.
	SendIntervalSeconds = 1.0

	// Extra pixels of padding around the detected face crop.
	FacePadding = 20

	// JPEG compression quality sent to backend (lower = smaller payload).
	JPEGQuality = 85
)
