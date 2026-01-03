package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"palabra-go/internals/config"
	"palabra-go/internals/palabra"
	"palabra-go/internals/test"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	runTest := flag.Bool("test", false, "Run integration tests")
	testLangs := flag.Bool("test-langs", false, "Test multiple language pairs")
	sourceLang := flag.String("source", "en", "Source language code")
	targetLang := flag.String("target", "de", "Target language code")
	flag.Parse()

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}
	log.Println("╔══════════════════════════════════════════════════════════╗")
	log.Println("║         Palabra.ai Real-Time Translation Service        ║")
	log.Println("╚══════════════════════════════════════════════════════════╝")
	log.Printf("Configuration loaded successfully")
	log.Printf("API URL: %s\n", cfg.APIUrl)
	if *runTest {
		if err := test.RunIntegrationTest(cfg.ClientID, cfg.ClientSecret); err != nil {
			log.Fatalf("Integration test failed: %v", err)
		}
		return
	}

	if *testLangs {
		if err := test.TestMultipleLanguages(cfg.ClientID, cfg.ClientSecret); err != nil {
			log.Fatalf("Language test failed: %v", err)
		}
		return
	}

	if err := runApplication(cfg, *sourceLang, *targetLang); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func runApplication(cfg *config.Config, sourceLang, targetLang string) error {
	log.Println("\n[1/4] Creating Palabra client...")
	client := palabra.NewPalabraClientWithURL(cfg.ClientID, cfg.ClientSecret, cfg.APIUrl)
	log.Println("\n[2/4] Creating streaming session...")
	if err := client.CreateSession(); err != nil {
		return err
	}
	log.Println("\n[3/4] Establishing WebRTC connection...")
	if err := client.ConnectWebRTC(); err != nil {
		return err
	}
	log.Println("\n[4/4] Publishing audio track...")
	if err := client.PublishAudioTrack(); err != nil {
		return err
	}
	log.Println("\n⏳ Waiting for connections to stabilize...")
	time.Sleep(3 * time.Second)

	log.Printf("\n🚀 Starting translation: %s -> %s", sourceLang, targetLang)
	if err := client.StartTranslation(sourceLang, targetLang); err != nil {
		return err
	}
	log.Println("\n╔══════════════════════════════════════════════════════════╗")
	log.Println("║    Translation Service Running                           ║")
	log.Println("╚══════════════════════════════════════════════════════════╝")
	log.Println("  Press Ctrl+C to stop")
	log.Println(" Transcriptions and translations will appear below:")
	log.Println("───────────────────────────────────────────────────────────")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("\n───────────────────────────────────────────────────────────")
	log.Println("Shutdown signal received...")
	if err := client.Disconnect(); err != nil {
		log.Printf("⚠️  Warning: Error during disconnect: %v", err)
	}

	log.Println("\nApplication stopped successfully")
	return nil
}
