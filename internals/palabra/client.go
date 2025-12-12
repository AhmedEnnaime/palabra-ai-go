package palabra

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

type PalabraClient struct {
	ClientID     string
	ClientSecret string
	APIUrl       string
	Session      *SessionData
	PeerConn     *webrtc.PeerConnection
	DataChannel  *webrtc.DataChannel
}

func NewPalabraClient(clientID, clientSecret string) *PalabraClient {
	return &PalabraClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		APIUrl:       "https://api.palabra.dev/session-storage/session",
	}
}

func NewPalabraClientWithURL(clientID, clientSecret, apiURL string) *PalabraClient {
	return &PalabraClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		APIUrl:       apiURL,
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
	log.Printf("✓ Session created: Room=%s", session.WebRTCRoomName)
	log.Printf("  WebRTC URL: %s", session.WebRTCURL)
	log.Printf("  WS URL: %s", session.WSURL)
	return nil
}

func (pc *PalabraClient) ConnectToWebRTC() error {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return fmt.Errorf("failed to create peer connection: %w", err)
	}
	pc.PeerConn = peerConnection
	dataChannel, err := peerConnection.CreateDataChannel("data", nil)
	if err != nil {
		return fmt.Errorf("failed to create data channel: %w", err)
	}
	pc.DataChannel = dataChannel
	dataChannel.OnMessage(func(msg webrtc.DataChannelMessage) {
		pc.handleTranscriptionMessage(msg.Data)
	})
	dataChannel.OnOpen(func() {
		log.Println("✓ Data channel opened")
	})
	dataChannel.OnClose(func() {
		log.Println("✗ Data channel closed")
	})
	peerConnection.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		log.Printf("✓ Received track: %s (type: %s)", tr.ID(), tr.Kind())
		go func() {
			packetCount := 0
			for {
				rtpPacket, _, readErr := tr.ReadRTP()
				if readErr != nil {
					log.Printf("Track read ended: %v", readErr)
					return
				}
				if rtpPacket != nil {
					packetCount++
					if packetCount%100 == 0 {
						log.Printf("Received %d audio packets (latest: %d bytes)", packetCount, len(rtpPacket.Payload))
					}
				}
			}
		}()
	})
	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("ICE Connection State: %s", state.String())
	})
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("Peer Connection State: %s", state.String())
	})

	log.Println("✓ WebRTC connection initialized")
	return nil
}

func (pc *PalabraClient) publishAudioTrack() error {
	audioTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		"pion-audio",
	)
	if err != nil {
		return fmt.Errorf("failed to create audio track: %w", err)
	}
	retpSender, err := pc.PeerConn.AddTrack(audioTrack)
	if err != nil {
		return fmt.Errorf("failed to add track: %w", err)
	}
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := retpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()
	log.Println("✓ Audio track published")
	return nil
}

func (pc *PalabraClient) SendAudioSample(track *webrtc.TrackLocalStaticSample, data []byte, duration time.Duration) error {
	return track.WriteSample(media.Sample{
		Data:     data,
		Duration: duration,
	})
}

func (pc *PalabraClient) StartTranslation(sourceLang, targetLang string) error {
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
	maxAttempts := 30
	for i := 0; i < maxAttempts; i++ {
		if pc.DataChannel != nil && pc.DataChannel.ReadyState() == webrtc.DataChannelStateOpen {
			break
		}
		if i == maxAttempts-1 {
			return fmt.Errorf("data channel not open after %d seconds", maxAttempts)
		}
		time.Sleep(1 * time.Second)
	}
	err = pc.DataChannel.Send(configJSON)
	if err != nil {
		return fmt.Errorf("failed to send config: %w", err)
	}

	log.Printf("✓ Translation started: %s -> %s", sourceLang, targetLang)
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
		log.Printf("📝 [VALIDATED] %s", transData.Transcription.Text)
	case "partial_transcription":
		var transData TranscriptionData
		if err := json.Unmarshal(msg.Data, &transData); err != nil {
			log.Printf("Failed to unmarshal transcription: %v", err)
			return
		}
		log.Printf("📝 [PARTIAL] %s", transData.Transcription.Text)
	case "translated_transcription":
		var transData TranscriptionData
		if err := json.Unmarshal(msg.Data, &transData); err != nil {
			log.Printf("Failed to unmarshal translation: %v", err)
			return
		}
		log.Printf("🌐 [TRANSLATION-%s] %s",
			transData.Transcription.Language,
			transData.Transcription.Text)
	default:
		log.Printf("Received message type: %s", msg.MessageType)
	}
}

func (pc *PalabraClient) Disconnect() error {
	log.Println("Disconnecting...")
	if pc.DataChannel != nil {
		if err := pc.DataChannel.Close(); err != nil {
			log.Printf("Warning: DataChannel close error: %v", err)
		}
	}
	if pc.PeerConn != nil {
		if err := pc.PeerConn.Close(); err != nil {
			return fmt.Errorf("failed to close peer connection: %w", err)
		}
	}
	log.Println("✓ Disconnected successfully")
	return nil
}
