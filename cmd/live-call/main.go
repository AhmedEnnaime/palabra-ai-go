package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"palabra-go/internals/audio"
	"palabra-go/internals/config"
	"palabra-go/internals/palabra"
	"strings"
	"sync"
	"time"
)

type LiveCallParticipant struct {
	Name          string
	Language      string
	AudioManager  *audio.AudioManager
	PalabraClient *palabra.PalabraClient
	Active        bool
	mu            sync.Mutex
}

func NewLiveCallParticipant(name, language string, cfg *config.Config) (*LiveCallParticipant, error) {
	audioMgr, err := audio.NewAudioManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create audio manager: %w", err)
	}
	palabraClient := palabra.NewPalabraClient(cfg.ClientID, cfg.ClientSecret)
	return &LiveCallParticipant{
		Name:          name,
		Language:      language,
		AudioManager:  audioMgr,
		PalabraClient: palabraClient,
		Active:        false,
	}, nil
}

func (p *LiveCallParticipant) Connect(targetLanguage string) error {
	log.Printf("[%s] Connecting to Palabra...", p.Name)
	if err := p.PalabraClient.CreateSession(); err != nil {
		return fmt.Errorf("session creation failed: %w", err)
	}
	if err := p.PalabraClient.ConnectWebRTC(); err != nil {
		return fmt.Errorf("WebRTC connection failed: %w", err)
	}
	if err := p.PalabraClient.PublishAudioTrack(); err != nil {
		return fmt.Errorf("audio track publishing failed: %w", err)
	}
	time.Sleep(2 * time.Second)
	if err := p.PalabraClient.StartTranslation(p.Language, targetLanguage); err != nil {
		return fmt.Errorf("translation start failed: %w", err)
	}

	p.Active = true
	log.Printf("[%s] Connected! Speaking: %s, Hearing: %s", p.Name, p.Language, targetLanguage)
	return nil
}

func (p *LiveCallParticipant) StartAudio() error {
	err := p.AudioManager.StartRecording(func(audioData []byte, duration time.Duration) {
		p.mu.Lock()
		active := p.Active
		p.mu.Unlock()

		if !active {
			return
		}
		if p.PalabraClient.AudioTrack != nil {
			if err := p.PalabraClient.SendAudioSample(audioData, duration); err != nil {
				log.Printf("[%s] Error sending audio: %v", p.Name, err)
			}
		}
	})

	if err != nil {
		return fmt.Errorf("failed to start recording: %w", err)
	}
	if err := p.AudioManager.StartPlayBack(p.PalabraClient.AudioChannel); err != nil {
		return fmt.Errorf("failed to start playback: %w", err)
	}

	log.Printf("[%s] Audio active", p.Name)
	return nil
}

func (p *LiveCallParticipant) Stop() {
	p.mu.Lock()
	p.Active = false
	p.mu.Unlock()

	log.Printf("[%s] Stopping...", p.Name)

	if p.AudioManager != nil {
		p.AudioManager.StopRecording()
		p.AudioManager.StopPlayBack()
		p.AudioManager.Close()
	}

	if p.PalabraClient != nil {
		p.PalabraClient.Disconnect()
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)

	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║     Live Translated Call Demo with Real Audio           ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	if len(os.Args) > 1 && os.Args[1] == "--list-devices" {
		if err := audio.ListDevices(); err != nil {
			log.Fatalf("Error listing devices: %v", err)
		}
		return
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println("Important Notes:")
	fmt.Println("   • The translated voice will be a synthetic TTS voice (not your voice)")
	fmt.Println("   • You'll hear your English speech translated to French in TTS")
	fmt.Println("   • Press Enter to stop")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Your language (en/de/fr/es): ")
	myLang, _ := reader.ReadString('\n')
	myLang = strings.TrimSpace(myLang)
	if myLang == "" {
		myLang = "en"
	}
	fmt.Print("Translate to (en/de/fr/es): ")
	targetLang, _ := reader.ReadString('\n')
	targetLang = strings.TrimSpace(targetLang)
	if targetLang == "" {
		targetLang = "fr"
	}

	fmt.Println()
	fmt.Printf("Setting up: You speak %s → Hear %s (TTS voice)\n", myLang, targetLang)
	fmt.Println()
	participant, err := NewLiveCallParticipant("You", myLang, cfg)
	if err != nil {
		log.Fatalf("Failed to create participant: %v", err)
	}
	defer participant.Stop()
	if err := participant.Connect(targetLang); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	if err := participant.StartAudio(); err != nil {
		log.Fatalf("Failed to start audio: %v", err)
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║    🎙️  CALL ACTIVE - Start Speaking!                     ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("🎤 Speak in %s\n", strings.ToUpper(myLang))
	fmt.Printf("🔊 You'll hear yourself in %s (TTS voice)\n", strings.ToUpper(targetLang))
	fmt.Println()
	fmt.Println("Press Enter to end call...")
	fmt.Println()
	reader.ReadString('\n')

	fmt.Println("Ending call...")
	participant.Stop()

	fmt.Println("✅ Call ended successfully!")
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("Demo completed! This shows:")
	fmt.Println("   Real microphone capture")
	fmt.Println("   Audio encoding (Opus)")
	fmt.Println("   Sending to Palabra via WebRTC")
	fmt.Println("   Receiving translated audio")
	fmt.Println("   Audio decoding and playback")
	fmt.Println("═══════════════════════════════════════════════════════════")
}
