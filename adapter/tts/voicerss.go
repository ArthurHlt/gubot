package tts

import (
	"github.com/ArthurHlt/gubot/robot"
	"io"
	"net/url"
)

const (
	voiceRssUrl = "https://api.voicerss.org/"
)

type VoicerssConfig struct {
	Token       string
	Voice       string
	AudioFormat string
}

type voicerssProvider struct {
}

func (a voicerssProvider) MessageAudio(message string, config *TTSConfig) (io.ReadCloser, error) {
	cfg := config.TtsVoicerss
	format := cfg.AudioFormat
	if format == "" {
		format = "22khz_16bit_stereo"
	}
	voice := cfg.Voice
	if voice == "" {
		voice = "en-gb"
	}

	resp, err := robot.HttpClient().PostForm(voiceRssUrl, url.Values{
		"src": []string{message},
		"key": []string{cfg.Token},
		"c":   []string{"OGG"},
		"f":   []string{format},
		"hl":  []string{voice},
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
