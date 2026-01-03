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
	"time"
)

type User struct {
	Name          string
	Language      string
	AudioManager  *audio.AudioManager
	PalabraClient *palabra.PalabraClient
	AudioChannel  chan []byte
	IsTalking     bool
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║     Two-User Translated Call Simulation                 ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println("This demo simulates a call between Alice (English) and Bob (German)")
	fmt.Println()
	fmt.Println("Setup:")
	fmt.Println("  • Alice speaks English, hears German")
	fmt.Println("  • Bob speaks German, hears English")
	fmt.Println()
	fmt.Println("You'll switch between speaking as Alice and Bob")
	fmt.Println("Press Enter to continue...")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')

	fmt.Println("\n═══ Setting up Alice (English) ═══")
	alice, err := createUser("Alice", "en", "de", cfg)
	if err != nil {
		log.Fatalf("Failed to create Alice: %v", err)
	}
	defer alice.AudioManager.Close()
	defer alice.PalabraClient.Disconnect()
	time.Sleep(2 * time.Second)

	fmt.Println("\n═══ Setting up Bob (German) ═══")
	bob, err := createUser("Bob", "de", "en", cfg)
	if err != nil {
		log.Fatalf("Failed to create Bob: %v", err)
	}
	defer bob.AudioManager.Close()
	defer bob.PalabraClient.Disconnect()

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║     Call Active - Ready to Talk!                       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  'a' - Speak as Alice (English)")
	fmt.Println("  'b' - Speak as Bob (German)")
	fmt.Println("  'q' - End call")
	fmt.Println()

	currentUser := alice
	for {
		fmt.Printf("\n[%s speaking %s] Command (a/b/q): ",
			currentUser.Name, strings.ToUpper(currentUser.Language))
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		switch input {
		case "a":
			if currentUser != alice {
				switchUser(currentUser, alice)
				currentUser = alice
			}
			fmt.Println("🎤 Alice: Speak English now...")
			time.Sleep(5 * time.Second)
			fmt.Println("   (5 seconds elapsed)")
		case "b":
			if currentUser != bob {
				switchUser(currentUser, bob)
				currentUser = bob
			}
			fmt.Println("Bob: Sprechen Sie Deutsch jetzt...")
			time.Sleep(5 * time.Second)
			fmt.Println("   (5 seconds elapsed)")
		case "q":
			fmt.Println("Ending call...")
			return
		default:
			fmt.Println("Invalid command. Use 'a', 'b', or 'q'")
		}
	}
}

func createUser(name, sourceLang, targetLang string, cfg *config.Config) (*User, error) {
	audioMgr, err := audio.NewAudioManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create audio manager for %s: %w", name, err)
	}
	client := palabra.NewPalabraClient(cfg.ClientID, cfg.ClientSecret)
	log.Printf("[%s] Creating session...", name)
	if err := client.CreateSession(); err != nil {
		return nil, fmt.Errorf("session creation failed for %s: %w", name, err)
	}
	if err := client.ConnectWebRTC(); err != nil {
		return nil, fmt.Errorf("WebRTC connection failed for %s: %w", name, err)
	}
	if err := client.PublishAudioTrack(); err != nil {
		return nil, fmt.Errorf("audio track publishing failed for %s: %w", name, err)
	}
	time.Sleep(2 * time.Second)
	if err := client.StartTranslation(sourceLang, targetLang); err != nil {
		return nil, fmt.Errorf("translation start failed for %s: %w", name, err)
	}
	log.Printf("[%s]Ready! Speaks: %s, Hears: %s", name, sourceLang, targetLang)
	return &User{
		Name:          name,
		Language:      sourceLang,
		AudioManager:  audioMgr,
		PalabraClient: client,
		AudioChannel:  make(chan []byte, 100),
		IsTalking:     false,
	}, nil
}

func switchUser(from, to *User) {
	if from.IsTalking {
		from.AudioManager.StopRecording()
		from.AudioManager.StopPlayBack()
		from.IsTalking = false
	}
	if !to.IsTalking {
		to.AudioManager.StartRecording(func(audioData []byte, d time.Duration) {
			if to.PalabraClient.AudioTrack != nil {
				if err := to.PalabraClient.SendAudioSample(audioData, d); err != nil {
					log.Printf("[%s] Error sending audio: %v", to.Name, err)
				}
			}
		})
		to.AudioManager.StartPlayBack(to.AudioChannel)
		to.IsTalking = true
		fmt.Printf("Switched to %s (%s)\n", to.Name, to.Language)
	}
}
