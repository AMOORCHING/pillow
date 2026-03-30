package tts

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// CartesiaProvider streams TTS via Cartesia Sonic over WebSocket.
type CartesiaProvider struct {
	apiKey  string
	voiceID string
	model   string

	mu        sync.Mutex
	conn      *websocket.Conn
	contextID int
	cancel    context.CancelFunc // cancel current playback
}

// NewCartesiaProvider creates a Cartesia TTS provider.
func NewCartesiaProvider(apiKey, voiceID, model string) *CartesiaProvider {
	if model == "" {
		model = "sonic-3"
	}
	if voiceID == "" {
		voiceID = "694f9389-aac1-45b6-b726-9d9369183238" // default english male
	}
	return &CartesiaProvider{
		apiKey:  apiKey,
		voiceID: voiceID,
		model:   model,
	}
}

func (c *CartesiaProvider) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	url := fmt.Sprintf("wss://api.cartesia.ai/tts/websocket?api_key=%s&cartesia_version=2025-04-16", c.apiKey)
	dialer := websocket.Dialer{
		ReadBufferSize:  256 * 1024,
		WriteBufferSize: 8 * 1024,
	}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("connecting to Cartesia: %w", err)
	}
	c.conn = conn
	return nil
}

type cartesiaRequest struct {
	ModelID      string            `json:"model_id"`
	Transcript   string            `json:"transcript"`
	Voice        cartesiaVoice     `json:"voice"`
	Language     string            `json:"language"`
	ContextID    string            `json:"context_id"`
	OutputFormat cartesiaOutFormat `json:"output_format"`
	Continue     bool              `json:"continue"`
}

type cartesiaVoice struct {
	Mode string `json:"mode"`
	ID   string `json:"id"`
}

type cartesiaOutFormat struct {
	Container  string `json:"container"`
	Encoding   string `json:"encoding"`
	SampleRate int    `json:"sample_rate"`
}

type cartesiaResponse struct {
	Type      string `json:"type"`
	Data      string `json:"data"` // base64-encoded PCM
	Done      bool   `json:"done"`
	ContextID string `json:"context_id"`
	Error     string `json:"error"`
}

func (c *CartesiaProvider) Speak(ctx context.Context, text string) error {
	if err := c.connect(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	c.mu.Lock()
	c.cancel = cancel
	c.contextID++
	ctxID := fmt.Sprintf("narration-%d", c.contextID)
	c.mu.Unlock()

	defer cancel()

	req := cartesiaRequest{
		ModelID:    c.model,
		Transcript: text,
		Voice:      cartesiaVoice{Mode: "id", ID: c.voiceID},
		Language:   "en",
		ContextID:  ctxID,
		OutputFormat: cartesiaOutFormat{
			Container:  "raw",
			Encoding:   "pcm_s16le",
			SampleRate: 24000,
		},
	}

	c.mu.Lock()
	err := c.conn.WriteJSON(req)
	c.mu.Unlock()
	if err != nil {
		c.reconnect()
		return fmt.Errorf("sending TTS request: %w", err)
	}

	// Collect PCM chunks
	var pcmData []byte
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return fmt.Errorf("connection closed")
		}

		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			c.reconnect()
			return fmt.Errorf("reading TTS response: %w", err)
		}

		var resp cartesiaResponse
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}

		if resp.Error != "" {
			return fmt.Errorf("Cartesia error: %s", resp.Error)
		}

		if resp.ContextID != ctxID {
			continue // stale response from previous context
		}

		if resp.Data != "" {
			chunk, err := base64.StdEncoding.DecodeString(resp.Data)
			if err == nil {
				pcmData = append(pcmData, chunk...)
			}
		}

		if resp.Done {
			break
		}
	}

	// Play collected PCM via sox/play
	return playPCM(ctx, pcmData)
}

func (c *CartesiaProvider) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
}

func (c *CartesiaProvider) Close() error {
	c.Stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

func (c *CartesiaProvider) reconnect() {
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.conn = nil
	c.mu.Unlock()
}

// playPCM plays raw PCM s16le 24kHz mono audio.
// Tries sox (play) first, falls back to ffplay, then afplay via temp file.
func playPCM(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// Try sox play
	cmd := exec.CommandContext(ctx, "play",
		"-t", "raw", "-r", "24000", "-b", "16", "-e", "signed", "-c", "1", "-")
	cmd.Stdin = newBytesReader(data)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fallback: convert to WAV in memory and use afplay via temp file
	return playViaAfplay(ctx, data)
}

func playViaAfplay(ctx context.Context, pcmData []byte) error {
	// Build a minimal WAV header + pcm data
	wav := buildWAV(pcmData, 24000, 16, 1)

	// Write to temp file and play
	f, err := createTempFile("pillow-*.wav")
	if err != nil {
		return err
	}
	defer removeFile(f)

	if _, err := f.Write(wav); err != nil {
		f.Close()
		return err
	}
	f.Close()

	cmd := exec.CommandContext(ctx, "afplay", f.Name())
	return cmd.Run()
}

// buildWAV creates a WAV file from raw PCM data.
func buildWAV(pcm []byte, sampleRate, bitsPerSample, channels int) []byte {
	dataLen := len(pcm)
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8

	buf := make([]byte, 44+dataLen)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(36+dataLen))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16) // chunk size
	binary.LittleEndian.PutUint16(buf[20:22], 1)  // PCM
	binary.LittleEndian.PutUint16(buf[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(buf[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(buf[34:36], uint16(bitsPerSample))
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataLen))
	copy(buf[44:], pcm)
	return buf
}

