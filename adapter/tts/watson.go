package tts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ArthurHlt/gubot/robot"
)

type WatsonConfig struct {
	Token string
	Url   string
	Voice string
	Rate  int
}

type watsonProvider struct {
}

func (a watsonProvider) MessageAudio(message string, config *TTSConfig) (io.ReadCloser, error) {
	watsonConfig := config.TtsWatson
	rate := watsonConfig.Rate
	if rate == 0 {
		rate = 22050
	}
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(struct {
		Text string `json:"text"`
	}{
		Text: message,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/v1/synthesize", watsonConfig.Url),
		buf,
	)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("apikey", watsonConfig.Token)
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("accept", fmt.Sprintf("audio/ogg;codecs=vorbis;rate=%d", rate))
	q := req.URL.Query()
	if watsonConfig.Voice != "" {
		q.Set("voice", watsonConfig.Voice)
	}
	req.URL.RawQuery = q.Encode()
	resp, err := robot.HttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
