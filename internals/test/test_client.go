package test

import (
	"fmt"
	"log"
	"palabra-go/internals/palabra"
	"time"
)

func RunIntegrationTest(clientID, clientSecret string) error {
	log.Println("╔══════════════════════════════════════════════════════════╗")
	log.Println("║    Palabra.ai Integration Test Suite                    ║")
	log.Println("╚══════════════════════════════════════════════════════════╝")

	log.Println("\n[Test 1/5] Creating Palabra client...")
	client := palabra.NewPalabraClient(clientID, clientSecret)
	if client == nil {
		return fmt.Errorf("failed to create client")
	}
	log.Println("✓ Client created successfully")
	log.Println("\n[Test 2/5] Creating streaming session...")
	if err := client.CreateSession(); err != nil {
		return fmt.Errorf("session creation failed: %w", err)
	}
	log.Println("✓ Session created successfully")

	log.Println("\n[Test 3/5] Establishing WebRTC connection...")
	if err := client.ConnectWebRTC(); err != nil {
		return fmt.Errorf("WebRTC connection failed: %w", err)
	}
	log.Println("✓ WebRTC connection established")
	log.Println("\n[Test 4/5] Publishing audio track...")
	if err := client.PublishAudioTrack(); err != nil {
		return fmt.Errorf("audio track publishing failed: %w", err)
	}
	log.Println("✓ Audio track published")
	log.Println("\n⏳ Waiting for connections to stabilize (5 seconds)...")
	time.Sleep(5 * time.Second)

	log.Println("\n[Test 5/5] Starting translation (English -> German)...")
	if err := client.StartTranslation("en", "de"); err != nil {
		return fmt.Errorf("translation start failed: %w", err)
	}
	log.Println("✓ Translation configuration sent successfully")
	log.Println("\n╔══════════════════════════════════════════════════════════╗")
	log.Println("║    All Tests Passed! Monitoring for 30 seconds...       ║")
	log.Println("╚══════════════════════════════════════════════════════════╝")
	log.Println("\nℹ️  In production, audio would be streamed here.")
	log.Println("ℹ️  Transcriptions and translations will appear below:")
	log.Println("───────────────────────────────────────────────────────────")

	time.Sleep(30 * time.Second)

	log.Println("\n───────────────────────────────────────────────────────────")
	log.Println("Cleaning up...")
	if err := client.Disconnect(); err != nil {
		log.Printf("⚠️  Warning: Disconnect error: %v", err)
	}
	log.Println("\n╔══════════════════════════════════════════════════════════╗")
	log.Println("║    ✅ All Integration Tests Completed Successfully!      ║")
	log.Println("╚══════════════════════════════════════════════════════════╝")

	return nil
}

func TestMultipleLanguages(clientID, clientSecret string) error {
	log.Println("\n╔══════════════════════════════════════════════════════════╗")
	log.Println("║    Testing Multiple Language Pairs                      ║")
	log.Println("╚══════════════════════════════════════════════════════════╝")

	languagePairs := []struct {
		source string
		target string
		name   string
	}{
		{"en", "de", "English -> German"},
		{"en", "es", "English -> Spanish"},
		{"en", "fr", "English -> French"},
	}
	for i, pair := range languagePairs {
		log.Printf("\n[%d/%d] Testing %s...", i+1, len(languagePairs), pair.name)
		client := palabra.NewPalabraClient(clientID, clientSecret)

		if err := client.CreateSession(); err != nil {
			return fmt.Errorf("failed to create session for %s: %w", pair.name, err)
		}

		if err := client.ConnectWebRTC(); err != nil {
			return fmt.Errorf("failed to connect for %s: %w", pair.name, err)
		}

		if err := client.PublishAudioTrack(); err != nil {
			return fmt.Errorf("failed to publish track for %s: %w", pair.name, err)
		}

		time.Sleep(3 * time.Second)
		if err := client.StartTranslation(pair.source, pair.target); err != nil {
			return fmt.Errorf("failed to start translation for %s: %w", pair.name, err)
		}

		log.Printf("✓ %s configured successfully", pair.name)

		time.Sleep(5 * time.Second)
		if err := client.Disconnect(); err != nil {
			log.Printf("⚠️  Warning: Disconnect error for %s: %v", pair.name, err)
		}
	}
	log.Println("\n✅ All language pair tests completed!")
	return nil
}
