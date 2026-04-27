// sender.go — WebSocket and HTTP communication with the backend.
// Mirrors raspberry_pi/sender.py exactly.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"gocv.io/x/gocv"
)

// Response models the JSON response from the backend.
type Response struct {
	Name               string  `json:"name"`
	Score              float64 `json:"score"`
	Matched            bool    `json:"matched"`
	IsValidated        bool    `json:"is_validated"`
	LivenessConf       float64 `json:"liveness_conf"`
	ConsecutiveFrames  int     `json:"consecutive_frames"`
	Lockdown           bool    `json:"lockdown"`
	TooClose           bool    `json:"too_close"`
	Label              string  `json:"label"`
	Timestamp          float64 `json:"timestamp"`
	Error              string  `json:"error"`
}

// encodeFace compresses the cropped face to JPEG and returns a base64 string.
func encodeFace(faceImg gocv.Mat) (string, error) {
	buf, err := gocv.IMEncodeWithParams(gocv.JPEGFileExt, faceImg, []int{gocv.IMWriteJpegQuality, JPEGQuality})
	if err != nil {
		return "", err
	}
	defer buf.Close()
	return base64.StdEncoding.EncodeToString(buf.GetBytes()), nil
}

// ── WebSocketSender ─────────────────────────────────────────

type WebSocketSender struct {
	conn *websocket.Conn
}

func NewWebSocketSender() *WebSocketSender {
	return &WebSocketSender{}
}

func (s *WebSocketSender) Connect() error {
	if s.conn != nil {
		s.conn.Close()
	}
	
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second

	fmt.Printf("[WebSocket] Connecting to %s...\n", BackendWSURL)
	conn, _, err := dialer.Dial(BackendWSURL, nil)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}

	s.conn = conn
	fmt.Printf("[WebSocket] Connected to %s\n", BackendWSURL)
	return nil
}

func (s *WebSocketSender) IsConnected() bool {
	return s.conn != nil
}

func (s *WebSocketSender) SendFace(faceImg gocv.Mat, box [4]int, confidence float64, frameHeight, faceCount int) (*Response, error) {
	if !s.IsConnected() {
		if err := s.Connect(); err != nil {
			return nil, err
		}
	}

	b64Img, err := encodeFace(faceImg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode face: %w", err)
	}

	// Calculate face height ratio
	faceHeightPx := box[3] - box[1]
	faceHeightRatio := 0.0
	if frameHeight > 0 {
		faceHeightRatio = float64(faceHeightPx) / float64(frameHeight)
	}

	payload := map[string]interface{}{
		"type":              "recognize",
		"face":              b64Img,
		"box":               box,
		"confidence":        confidence,
		"face_height_ratio": faceHeightRatio,
		"face_count":        faceCount,
		"timestamp":         float64(time.Now().UnixNano()) / 1e9,
	}

	if err := s.conn.WriteJSON(payload); err != nil {
		s.conn.Close()
		s.conn = nil
		return nil, fmt.Errorf("write error: %w", err)
	}

	// Set read deadline
	s.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	
	var resp Response
	if err := s.conn.ReadJSON(&resp); err != nil {
		s.conn.Close()
		s.conn = nil
		return nil, fmt.Errorf("read error: %w", err)
	}

	return &resp, nil
}

func (s *WebSocketSender) SendNoFace() (*Response, error) {
	if !s.IsConnected() {
		if err := s.Connect(); err != nil {
			return nil, err
		}
	}

	payload := map[string]interface{}{
		"type":      "no_face",
		"timestamp": float64(time.Now().UnixNano()) / 1e9,
	}

	if err := s.conn.WriteJSON(payload); err != nil {
		s.conn.Close()
		s.conn = nil
		return nil, fmt.Errorf("write error: %w", err)
	}

	s.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	
	var resp Response
	if err := s.conn.ReadJSON(&resp); err != nil {
		s.conn.Close()
		s.conn = nil
		return nil, fmt.Errorf("read error: %w", err)
	}

	return &resp, nil
}

func (s *WebSocketSender) Close() {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
		fmt.Println("[WebSocket] Disconnected")
	}
}

// ── HTTPSender (Fallback) ───────────────────────────────────

type HTTPSender struct {
	client *http.Client
	url    string
}

func NewHTTPSender() *HTTPSender {
	url := BackendHTTPURL + "/recognize"
	fmt.Printf("[HTTP] Using endpoint: %s\n", url)
	return &HTTPSender{
		client: &http.Client{Timeout: 5 * time.Second},
		url:    url,
	}
}

func (s *HTTPSender) SendFace(faceImg gocv.Mat, box [4]int, confidence float64) (*Response, error) {
	buf, err := gocv.IMEncodeWithParams(gocv.JPEGFileExt, faceImg, []int{gocv.IMWriteJpegQuality, JPEGQuality})
	if err != nil {
		return nil, fmt.Errorf("encode failed: %w", err)
	}
	defer buf.Close()
	imgBytes := buf.GetBytes()

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add image
	part, err := writer.CreateFormFile("image", "face.jpg")
	if err != nil {
		return nil, err
	}
	part.Write(imgBytes)

	// Add metadata fields
	boxJson, _ := json.Marshal(box)
	writer.WriteField("box", string(boxJson))
	writer.WriteField("confidence", strconv.FormatFloat(confidence, 'f', -1, 64))

	writer.Close()

	req, err := http.NewRequest("POST", s.url, &requestBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	
	var apiResp Response
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %s", string(bodyBytes))
	}

	return &apiResp, nil
}
