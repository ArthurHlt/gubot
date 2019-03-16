package tts

import (
	"io"
)

type provider interface {
	MessageAudio(message string, config *TTSConfig) (io.ReadCloser, error)
}

var providers = map[string]provider{
	"watson":   &watsonProvider{},
	"voicerss": &voicerssProvider{},
}

func RetrieveProvider(config *TTSConfig) provider {
	if config.TtsWatson.Url != "" {
		return providers["watson"]
	}
	if config.TtsVoicerss.Token != "" {
		return providers["voicerss"]
	}
	return nil
}
