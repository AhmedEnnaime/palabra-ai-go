package palabra

import "encoding/json"

type TranslationConfig struct {
	MessageType string                `json:"message_type"`
	Data        TranslationConfigData `json:"data"`
}

type TranslationConfigData struct {
	InputStream  StreamConfig   `json:"input_stream"`
	OutputStream StreamConfig   `json:"output_stream"`
	Pipeline     PipelineConfig `json:"pipeline"`
}

type StreamConfig struct {
	ContentType string        `json:"content_type"`
	Source      *StreamSource `json:"source,omitempty"`
	Target      *StreamTarget `json:"target,omitempty"`
}

type StreamSource struct {
	Type string `json:"type"`
}

type StreamTarget struct {
	Type string `json:"type"`
}

type PipelineConfig struct {
	Transcription TranscriptionConfig `json:"transcription"`
	Translations  []TranslationLang   `json:"translations"`
}

type TranscriptionConfig struct {
	SourceLanguage string `json:"source_language"`
}

type TranslationLang struct {
	TargetLanguage string `json:"target_language"`
}

type SessionResponse struct {
	Data SessionData `json:"data"`
	OK   bool        `json:"ok"`
}

type SessionData struct {
	ID             string   `json:"id"`
	Publisher      string   `json:"publisher"`
	Subscriber     []string `json:"subscriber"`
	WebRTCRoomName string   `json:"webrtc_room_name"`
	WebRTCURL      string   `json:"webrtc_url"`
	WSURL          string   `json:"ws_url"`
}

type TranscriptionMessage struct {
	MessageType string          `json:"message_type"`
	Data        json.RawMessage `json:"data"`
}

type TranscriptionData struct {
	Transcription Transcription `json:"transcription"`
}

type Transcription struct {
	TranscriptionID string                 `json:"transcription_id"`
	Language        string                 `json:"language"`
	Text            string                 `json:"text"`
	Segments        []TranscriptionSegment `json:"segments"`
}

type TranscriptionSegment struct {
	Text           string  `json:"text"`
	Start          float64 `json:"start"`
	End            float64 `json:"end"`
	StartTimestamp float64 `json:"start_timestamp"`
	EndTimestamp   float64 `json:"end_timestamp"`
}
