// detector.go — CPU-optimized Haar Cascade face detection.
// Mirrors raspberry_pi/face_detector.py exactly.
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"

	"gocv.io/x/gocv"
)

// FaceData holds the detection result for a single face.
type FaceData struct {
	Box        image.Rectangle // Bounding box: x1, y1, x2, y2
	FaceImg    gocv.Mat        // Cropped face image (must be Closed when done)
	Confidence float64         // Always 1.0 for Haar Cascade
}

// FaceDetector wraps the OpenCV Haar Cascade classifier.
type FaceDetector struct {
	classifier gocv.CascadeClassifier
}

// NewFaceDetector loads the built-in OpenCV Haar Cascade model.
func NewFaceDetector() *FaceDetector {
	// Usually found at /usr/share/opencv4/haarcascades/haarcascade_frontalface_default.xml on Linux,
	// or in the gocv/data directory.
	// For cross-platform simplicity, we assume the XML file is in the same directory or provide a path.
	// A common path: "haarcascade_frontalface_default.xml"
	cascadePath := "haarcascade_frontalface_default.xml"

	classifier := gocv.NewCascadeClassifier()
	if !classifier.Load(cascadePath) {
		log.Fatalf("[FaceDetector] Failed to load Haar Cascade from %s. Please make sure the file exists.", cascadePath)
	}

	fmt.Println("[FaceDetector] Initialized with Haar Cascade (CPU)")
	fmt.Printf("[FaceDetector] Scale Factor: %.2f\n", DetectionScaleFactor)
	fmt.Printf("[FaceDetector] Min Neighbors: %d\n", DetectionMinNeighbors)

	return &FaceDetector{
		classifier: classifier,
	}
}

// Detect finds faces in the given frame and returns a slice of FaceData.
// The caller is responsible for calling face.FaceImg.Close() on each returned face.
func (fd *FaceDetector) Detect(frame gocv.Mat) []FaceData {
	if frame.Empty() {
		return nil
	}

	// 1. Convert to grayscale
	gray := gocv.NewMat()
	defer gray.Close()
	gocv.CvtColor(frame, &gray, gocv.ColorBGRToGray)

	// 2. Equalize histogram
	gocv.EqualizeHist(gray, &gray)

	// 3. Detect faces
	rects := fd.classifier.DetectMultiScaleWithParams(
		gray,
		DetectionScaleFactor,
		DetectionMinNeighbors,
		0, // flags
		image.Pt(MinFaceSize, MinFaceSize), // minSize
		image.Pt(0, 0),                     // maxSize
	)

	var faces []FaceData
	cols := frame.Cols()
	rows := frame.Rows()

	for _, r := range rects {
		// 4. Add padding
		x1 := r.Min.X - FacePadding
		if x1 < 0 {
			x1 = 0
		}
		y1 := r.Min.Y - FacePadding
		if y1 < 0 {
			y1 = 0
		}
		x2 := r.Max.X + FacePadding
		if x2 > cols {
			x2 = cols
		}
		y2 := r.Max.Y + FacePadding
		if y2 > rows {
			y2 = rows
		}

		paddedRect := image.Rect(x1, y1, x2, y2)

		// 5. Crop face from the ORIGINAL BGR frame
		// Region returns a Mat that points to the same underlying data,
		// so we Clone it to have an independent copy that survives the frame cycle.
		region := frame.Region(paddedRect)
		faceImg := region.Clone()
		region.Close()

		faces = append(faces, FaceData{
			Box:        paddedRect,
			FaceImg:    faceImg,
			Confidence: 1.0, // Haar doesn't provide confidence
		})
	}

	return faces
}

// DrawDetections draws bounding boxes on the frame (modifies frame in place).
func (fd *FaceDetector) DrawDetections(frame gocv.Mat, faces []FaceData) {
	green := color.RGBA{0, 255, 0, 0}
	for _, f := range faces {
		gocv.Rectangle(&frame, f.Box, green, 2)
		
		labelY := f.Box.Min.Y - 10
		if labelY < 10 {
			labelY = f.Box.Min.Y + 20
		}
		pt := image.Pt(f.Box.Min.X, labelY)
		gocv.PutText(&frame, "FACE", pt, gocv.FontHersheySimplex, 0.6, green, 2)
	}
}
