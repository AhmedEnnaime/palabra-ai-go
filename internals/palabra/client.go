package palabra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

type PalabraClient struct {
	ClientID     string
	ClientSecret string
	APIUrl       string
	Session      *SessionData
	Room         *lksdk.Room
	AudioTrack   *lksdk.LocalSampleTrack
	AudioChannel chan []byte
}

func NewPalabraClient(clientID, clientSecret string) *PalabraClient {
	return &PalabraClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		APIUrl:       "https://api.palabra.ai/session-storage/session",
		AudioChannel: make(chan []byte, 100),
	}
}

func NewPalabraClientWithURL(clientID, clientSecret, apiURL string) *PalabraClient {
	return &PalabraClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		APIUrl:       apiURL,
		AudioChannel: make(chan []byte, 100),
	}
}

func (pc *PalabraClient) CreateSession() error {
	payload := map[string]interface{}{
		"data": map[string]interface{}{
			"subscriber_count":        0,
			"publisher_can_subscribe": true,
		},
	}
	session, err := createStreamingSession(pc.APIUrl, pc.ClientID, pc.ClientSecret, payload)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	pc.Session = session
	log.Printf(" Session created: Room=%s", session.WebRTCRoomName)
	log.Printf("  WebRTC URL: %s", session.WebRTCURL)
	log.Printf("  WS URL: %s", session.WSURL)
	return nil
}

func (pc *PalabraClient) ConnectWebRTC() error {
	if pc.Session == nil {
		return fmt.Errorf("session not created yet")
	}

	log.Println("Connecting to WebRTC room...")
	log.Printf("  Room: %s", pc.Session.WebRTCRoomName)
	log.Printf("  URL: %s", pc.Session.WebRTCURL)
	log.Println("  Note: Connection may take 15-30 seconds for ICE negotiation...")
	roomCallback := &lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnDataPacket: func(data lksdk.DataPacket, params lksdk.DataReceiveParams) {
				if userPacket, ok := data.(*lksdk.UserDataPacket); ok {
					pc.handleTranscriptionMessage(userPacket.Payload)
				}
			},
			OnTrackSubscribed: func(track *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				log.Printf(" Received track: %s (type: %s, codec: %s)",
					track.ID(),
					track.Kind(),
					track.Codec().MimeType)
				go pc.readAudioTrack(track)
			},
		},
		OnDisconnected: func() {
			log.Println("✗ Room disconnected")
		},
		OnReconnecting: func() {
			log.Println("⟳ Reconnecting...")
		},
		OnReconnected: func() {
			log.Println(" Reconnected")
		},
	}
	room, err := lksdk.ConnectToRoomWithToken(
		pc.Session.WebRTCURL,
		pc.Session.Publisher,
		roomCallback,
		lksdk.WithAutoSubscribe(true),
	)

	if err != nil {
		return fmt.Errorf("failed to connect to room: %w", err)
	}

	pc.Room = room
	log.Println(" WebRTC connection initiated")
	log.Println("Waiting for connection to establish...")
	maxWait := 30 * time.Second
	checkInterval := 500 * time.Millisecond
	elapsed := time.Duration(0)

	for elapsed < maxWait {
		if pc.Room.LocalParticipant != nil {
			log.Println(" Local participant ready")
			break
		}
		time.Sleep(checkInterval)
		elapsed += checkInterval
	}

	if pc.Room.LocalParticipant == nil {
		pc.Room.Disconnect()
		return fmt.Errorf("connection timeout - local participant not ready after %v", maxWait)
	}
	log.Println("Allowing ICE connection to stabilize...")
	time.Sleep(3 * time.Second)

	log.Println(" WebRTC connection ready")
	return nil
}

func (pc *PalabraClient) readAudioTrack(track *webrtc.TrackRemote) {
	log.Println("Starting audio track reader...")

	packetCount := 0
	opusPacketCount := 0
	errorCount := 0

	for {
		rtpPacket, _, err := track.ReadRTP()
		if err != nil {
			if err == io.EOF {
				log.Println("Audio track ended (EOF)")
				return
			}
			errorCount++
			if errorCount%100 == 0 {
				log.Printf("Audio read error count: %d (last: %v)", errorCount, err)
			}
			continue
		}

		packetCount++
		if packetCount%100 == 0 {
			log.Printf("Received %d RTP packets, %d Opus packets forwarded", packetCount, opusPacketCount)
		}
		opusData := rtpPacket.Payload

		if len(opusData) == 0 {
			if packetCount < 10 {
				log.Printf("Empty RTP payload in packet #%d", packetCount)
			}
			continue
		}
		if opusPacketCount < 3 {
			log.Printf("Opus packet #%d: size=%d bytes, RTP seq=%d, timestamp=%d",
				opusPacketCount+1,
				len(opusData),
				rtpPacket.SequenceNumber,
				rtpPacket.Timestamp)
		}
		opusCopy := make([]byte, len(opusData))
		copy(opusCopy, opusData)
		select {
		case pc.AudioChannel <- opusCopy:
			opusPacketCount++
			if opusPacketCount == 1 {
				log.Println("First Opus packet sent to audio channel!")
			}
		default:
			if opusPacketCount < 10 {
				log.Printf("Audio channel full, dropping packet #%d", opusPacketCount)
			}
		}
	}
}

func (pc *PalabraClient) PublishAudioTrack() error {
	if pc.Room == nil {
		return fmt.Errorf("room not connected")
	}
	track, err := lksdk.NewLocalSampleTrack(webrtc.RTPCodecCapability{
		MimeType:  webrtc.MimeTypeOpus,
		ClockRate: 48000,
		Channels:  1,
	})
	if err != nil {
		return fmt.Errorf("failed to create audio track: %w", err)
	}
	pc.AudioTrack = track
	if _, err = pc.Room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{
		Name: "audio",
	}); err != nil {
		return fmt.Errorf("failed to publish track: %w", err)
	}

	log.Println(" Audio track published")
	return nil
}

func (pc *PalabraClient) StartTranslation(sourceLang, targetLang string) error {
	if pc.Room == nil {
		return fmt.Errorf("room not connected")
	}
	config := TranslationConfig{
		MessageType: "set_task",
		Data: TranslationConfigData{
			InputStream: StreamConfig{
				ContentType: "audio",
				Source: &StreamSource{
					Type: "webrtc",
				},
			},
			OutputStream: StreamConfig{
				ContentType: "audio",
				Target: &StreamTarget{
					Type: "webrtc",
				},
			},
			Pipeline: PipelineConfig{
				Transcription: TranscriptionConfig{
					SourceLanguage: sourceLang,
				},
				Translations: []TranslationLang{
					{
						TargetLanguage: targetLang,
					},
				},
			},
		},
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	dataPacket := lksdk.UserData(configJSON)
	err = pc.Room.LocalParticipant.PublishDataPacket(dataPacket, lksdk.WithDataPublishReliable(true))
	if err != nil {
		return fmt.Errorf("failed to send config: %w", err)
	}

	log.Printf(" Translation started: %s -> %s", sourceLang, targetLang)
	return nil
}

func (pc *PalabraClient) handleTranscriptionMessage(data []byte) {
	var msg TranscriptionMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("Failed to unmarshal message: %v", err)
		return
	}
	switch msg.MessageType {
	case "validated_transcription":
		var transData TranscriptionData
		if err := json.Unmarshal(msg.Data, &transData); err != nil {
			log.Printf("Failed to unmarshal transcription: %v", err)
			return
		}
		log.Printf("[VALIDATED] %s", transData.Transcription.Text)
		log.Printf("TIP: You should hear translated audio shortly after seeing this!")
	case "partial_transcription":
		var transData TranscriptionData
		if err := json.Unmarshal(msg.Data, &transData); err != nil {
			log.Printf("Failed to unmarshal transcription: %v", err)
			return
		}
		log.Printf("[PARTIAL] %s", transData.Transcription.Text)
	case "translated_transcription":
		var transData TranscriptionData
		if err := json.Unmarshal(msg.Data, &transData); err != nil {
			log.Printf("Failed to unmarshal translation: %v", err)
			return
		}
		log.Printf("[TRANSLATION-%s] %s",
			transData.Transcription.Language,
			transData.Transcription.Text)
		log.Printf("TIP: Translation audio should be playing now!")
	default:
		if msg.MessageType != "" {
			log.Printf("Received message type: %s", msg.MessageType)
		}
	}
}

func (pc *PalabraClient) SendAudioSample(data []byte, duration time.Duration) error {
	if pc.AudioTrack == nil {
		return fmt.Errorf("audio track not created")
	}
	sample := media.Sample{
		Data:     data,
		Duration: duration,
	}
	return pc.AudioTrack.WriteSample(sample, nil)
}

func (pc *PalabraClient) Disconnect() error {
	log.Println("Disconnecting...")

	if pc.Room != nil {
		pc.Room.Disconnect()
	}
	if pc.AudioChannel != nil {
		select {
		case <-pc.AudioChannel:
		default:
			close(pc.AudioChannel)
		}
	}

	log.Println(" Disconnected successfully")
	return nil
}

func (pc *PalabraClient) WaitForConnection(ctx context.Context, timeout time.Duration) error {
	if pc.Room == nil {
		return fmt.Errorf("room not initialized")
	}

	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			localParticipant := pc.Room.LocalParticipant
			if localParticipant != nil {
				log.Println(" Connection ready")
				return nil
			}

			if time.Since(start) > timeout {
				return fmt.Errorf("connection timeout after %v", timeout)
			}
		}
	}
}
