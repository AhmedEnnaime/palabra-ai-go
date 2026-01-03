package audio

import (
	"fmt"
	"log"
	"time"

	"github.com/gordonklaus/portaudio"
	"gopkg.in/hraban/opus.v2"
)

const (
	SampleRate      = 48000
	Channels        = 1
	FramesPerBuffer = 960
	BitRate         = 32000
)

type AudioManager struct {
	inputStream  *portaudio.Stream
	outputStream *portaudio.Stream
	encoder      *opus.Encoder
	decoder      *opus.Decoder
	isRecording  bool
	isPlaying    bool
}

func NewAudioManager() (*AudioManager, error) {
	if err := portaudio.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize portaudio: %w", err)
	}
	encoder, err := opus.NewEncoder(SampleRate, Channels, opus.AppVoIP)
	if err != nil {
		portaudio.Terminate()
		return nil, fmt.Errorf("failed to create opus encoder: %w", err)
	}
	encoder.SetBitrate(BitRate)
	decoder, err := opus.NewDecoder(SampleRate, Channels)
	if err != nil {
		portaudio.Terminate()
		return nil, fmt.Errorf("failed to create opus decoder: %w", err)
	}

	return &AudioManager{
		encoder: encoder,
		decoder: decoder,
	}, nil
}

func (am *AudioManager) StartRecording(onAudioData func([]byte, time.Duration)) error {
	if am.isRecording {
		return fmt.Errorf("already recording")
	}
	inputBuffer := make([]int16, FramesPerBuffer)
	opusBuffer := make([]byte, 4000)

	encodedCount := 0
	silenceCount := 0

	stream, err := portaudio.OpenDefaultStream(
		Channels,
		0,
		float64(SampleRate),
		FramesPerBuffer,
		func(in []int16) {
			copy(inputBuffer, in)
			hasAudio := false
			for _, sample := range inputBuffer {
				if sample > 100 || sample < -100 {
					hasAudio = true
					break
				}
			}

			if !hasAudio {
				silenceCount++
				if silenceCount%50 == 1 {
					log.Printf("Microphone input is silent (count: %d) - Speak louder or check mic!", silenceCount)
				}
			}

			n, err := am.encoder.Encode(inputBuffer, opusBuffer)
			if err != nil {
				log.Printf("Encoding error: %v", err)
				return
			}
			if n > 0 && onAudioData != nil {
				encodedCount++
				if encodedCount <= 3 || (hasAudio && encodedCount <= 10) {
					log.Printf("Encoded packet #%d: %d bytes %s", encodedCount, n,
						map[bool]string{true: "(has audio 🎵)", false: "(silence)"}[hasAudio])
				}
				duration := time.Duration(FramesPerBuffer) * time.Second / time.Duration(SampleRate)
				onAudioData(opusBuffer[:n], duration)
			}
		},
	)

	if err != nil {
		return fmt.Errorf("failed to open input stream: %w", err)
	}

	if err := stream.Start(); err != nil {
		stream.Close()
		return fmt.Errorf("failed to start input stream: %w", err)
	}

	am.inputStream = stream
	am.isRecording = true
	log.Println("Microphone recording started")
	return nil
}

func (am *AudioManager) StopRecording() error {
	if !am.isRecording {
		return nil
	}

	if am.inputStream != nil {
		if err := am.inputStream.Stop(); err != nil {
			log.Printf("Warning: error stopping input stream: %v", err)
		}
		if err := am.inputStream.Close(); err != nil {
			log.Printf("Warning: error closing input stream: %v", err)
		}
		am.inputStream = nil
	}

	am.isRecording = false
	log.Println("Microphone recording stopped")
	return nil
}

func (am *AudioManager) StartPlayBack(audioChannel <-chan []byte) error {
	if am.isPlaying {
		return fmt.Errorf("already playing")
	}

	pcmBuffer := make([]int16, FramesPerBuffer*2)
	var currentPCM []int16
	var currentIndex int

	decodedCount := 0
	playedFrames := 0
	channelReadCount := 0
	lastLogTime := time.Now()
	nonSilentPackets := 0

	stream, err := portaudio.OpenDefaultStream(
		0,
		Channels,
		float64(SampleRate),
		FramesPerBuffer,
		func(out []int16) {
			for i := range out {
				out[i] = 0
			}
			if currentIndex >= len(currentPCM) {
				select {
				case opusData := <-audioChannel:
					channelReadCount++

					if channelReadCount <= 3 {
						log.Printf("Received opus packet #%d from channel: %d bytes", channelReadCount, len(opusData))
					}

					if opusData != nil && len(opusData) > 0 {
						n, err := am.decoder.Decode(opusData, pcmBuffer)
						if err != nil {
							log.Printf("Decoding error (packet #%d, %d bytes): %v", channelReadCount, len(opusData), err)
							return
						}
						hasAudio := false
						for i := 0; i < n; i++ {
							if pcmBuffer[i] > 500 || pcmBuffer[i] < -500 {
								hasAudio = true
								break
							}
						}

						decodedCount++
						if hasAudio {
							nonSilentPackets++
							if nonSilentPackets <= 5 {
								log.Printf("AUDIO DETECTED in packet #%d: %d PCM samples (%.2f ms)",
									decodedCount,
									n,
									float64(n)/float64(SampleRate)*1000)
							}
						} else if decodedCount <= 3 {
							log.Printf("Decoded packet #%d: %d PCM samples (%.2f ms) [silence/comfort noise]",
								decodedCount,
								n,
								float64(n)/float64(SampleRate)*1000)
						}
						if time.Since(lastLogTime) > 5*time.Second {
							log.Printf("Playback status: %d packets received, %d with audio content",
								decodedCount, nonSilentPackets)
							lastLogTime = time.Now()
						}

						currentPCM = pcmBuffer[:n]
						currentIndex = 0
					}
				default:
					return
				}
			}
			if currentIndex < len(currentPCM) {
				n := copy(out, currentPCM[currentIndex:])
				currentIndex += n
				playedFrames += n

				if playedFrames <= FramesPerBuffer*3 && playedFrames%FramesPerBuffer == 0 {
					log.Printf("Playing audio: %d frames written", playedFrames)
				}
			}
		},
	)

	if err != nil {
		return fmt.Errorf("failed to open output stream: %w", err)
	}

	if err := stream.Start(); err != nil {
		stream.Close()
		return fmt.Errorf("failed to start output stream: %w", err)
	}

	am.outputStream = stream
	am.isPlaying = true
	log.Println("Speaker playback started")
	log.Println("Waiting for audio data from channel...")
	return nil
}

func (am *AudioManager) StopPlayBack() error {
	if !am.isPlaying {
		return nil
	}

	if am.outputStream != nil {
		if err := am.outputStream.Stop(); err != nil {
			log.Printf("Warning: error stopping output stream: %v", err)
		}
		if err := am.outputStream.Close(); err != nil {
			log.Printf("Warning: error closing output stream: %v", err)
		}
		am.outputStream = nil
	}

	am.isPlaying = false
	log.Println("Speaker playback stopped")
	return nil
}

func (am *AudioManager) Close() error {
	am.StopRecording()
	am.StopPlayBack()

	if err := portaudio.Terminate(); err != nil {
		return fmt.Errorf("failed to terminate portaudio: %w", err)
	}

	log.Println("Audio manager closed")
	return nil
}

func ListDevices() error {
	if err := portaudio.Initialize(); err != nil {
		return err
	}
	defer portaudio.Terminate()
	devices, err := portaudio.Devices()
	if err != nil {
		return err
	}
	fmt.Println("Available Audio Devices:")
	fmt.Println("========================")
	for i, device := range devices {
		fmt.Printf("%d: %s\n", i, device.Name)
		fmt.Printf("   Max Input Channels: %d\n", device.MaxInputChannels)
		fmt.Printf("   Max Output Channels: %d\n", device.MaxOutputChannels)
		fmt.Printf("   Default Sample Rate: %.0f Hz\n", device.DefaultSampleRate)
		fmt.Println()
	}

	defaultInput, err := portaudio.DefaultInputDevice()
	if err == nil {
		fmt.Printf("Default Input: %s\n", defaultInput.Name)
	}

	defaultOutput, err := portaudio.DefaultOutputDevice()
	if err == nil {
		fmt.Printf("Default Output: %s\n", defaultOutput.Name)
	}

	return nil
}
