package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gordonklaus/portaudio"
)

func main() {
	fmt.Println("🎤 Microphone Level Test")
	fmt.Println("========================")
	fmt.Println("Speak into your microphone...")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	if err := portaudio.Initialize(); err != nil {
		log.Fatalf("Failed to initialize portaudio: %v", err)
	}
	defer portaudio.Terminate()
	defaultInput, err := portaudio.DefaultInputDevice()
	if err != nil {
		log.Fatalf("No default input device: %v", err)
	}
	fmt.Printf("Using: %s\n", defaultInput.Name)
	fmt.Println()

	const (
		sampleRate      = 48000
		framesPerBuffer = 960
		channels        = 1
	)

	inputBuffer := make([]int16, framesPerBuffer)
	maxLevel := int16(0)
	frameCount := 0
	hasSeenAudio := false

	stream, err := portaudio.OpenDefaultStream(
		channels,
		0,
		float64(sampleRate),
		framesPerBuffer,
		func(in []int16) {
			copy(inputBuffer, in)
			var sum float64
			var peak int16
			for _, sample := range inputBuffer {
				absVal := sample
				if absVal < 0 {
					absVal = -absVal
				}
				if absVal > peak {
					peak = absVal
				}
				sum += float64(sample) * float64(sample)
			}
			rms := math.Sqrt(sum / float64(len(inputBuffer)))

			frameCount++
			if peak > maxLevel {
				maxLevel = peak
			}
			if frameCount%10 == 0 {
				bars := int(rms / 500)
				if bars > 50 {
					bars = 50
				}

				status := "SILENT"
				if peak > 100 {
					status = "AUDIO!"
					hasSeenAudio = true
				}

				fmt.Printf("\r%s | RMS: %6.0f | Peak: %5d | Max: %5d | %s%s",
					status,
					rms,
					peak,
					maxLevel,
					string([]byte(make([]byte, bars))),
					"          ")
			}
		},
	)

	if err != nil {
		log.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	if err := stream.Start(); err != nil {
		log.Fatalf("Failed to start stream: %v", err)
	}
	defer stream.Stop()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if !hasSeenAudio {
				fmt.Printf("No audio detected yet. Try speaking louder or check mic permissions!\n")
			}
		}
	}()

	<-sigChan
	fmt.Println("Test complete!")
	if hasSeenAudio {
		fmt.Println("Microphone is working - audio was detected")
		fmt.Printf("Maximum level reached: %d (threshold is 100)\n", maxLevel)
	} else {
		fmt.Println("✗ No audio detected")
		fmt.Println("  Possible issues:")
		fmt.Println("  - App doesn't have microphone permissions")
		fmt.Println("  - Microphone is muted or volume is too low")
		fmt.Println("  - Wrong input device selected")
	}
}
