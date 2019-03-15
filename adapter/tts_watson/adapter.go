package tts_watson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ArthurHlt/gubot/robot"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"github.com/writeas/go-strip-markdown"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const ttsWatsonNS = "7c4e25d3-f9ea-4b64-b5e5-739bd35f556c"

func init() {
	robot.RegisterAdapter(&TTSWatsonAdapter{
		messageChan: make(chan string, 100),
	})
}

type TTSWatsonConfig struct {
	TtsWatsonToken        string
	TtsWatsonUrl          string
	TtsWatsonVoice        string
	TtsWatsonSpeedPercent int
	TtsWatsonRate         int
	TtsWatsonEnableCache  bool
}

type TTSWatsonAdapter struct {
	config      *TTSWatsonConfig
	gubot       *robot.Gubot
	messageChan chan string
}

func (TTSWatsonAdapter) Name() string {
	return "TTS_Watson"
}

type TTSRequest struct {
	Text string `json:"text"`
}

func (a TTSWatsonAdapter) Send(_ robot.Envelop, message string) error {
	a.messageChan <- stripmd.Strip(message)
	return nil
}

func (a TTSWatsonAdapter) Reply(envelop robot.Envelop, message string) error {
	return a.Send(envelop, fmt.Sprintf("%s, %s", envelop.User.Name, message))
}

func (a *TTSWatsonAdapter) Run(config interface{}, r *robot.Gubot) error {
	entry := log.WithField("adapter", "tts_watson")
	a.config = config.(*TTSWatsonConfig)
	if a.config.TtsWatsonSpeedPercent <= 0 {
		a.config.TtsWatsonSpeedPercent = 50
	}
	if a.config.TtsWatsonRate == 0 {
		a.config.TtsWatsonRate = 22050
	}
	a.gubot = r
	speedPercent := a.config.TtsWatsonSpeedPercent
	done := make(chan bool)
	go func() {
		for message := range a.messageChan {
			resp, err := a.messageAudio(stripmd.Strip(message))
			if err != nil {
				entry.Error(err)
				continue
			}
			streamer, format, err := vorbis.Decode(resp)
			if err != nil {
				resp.Close()
				entry.Error(err)
				continue
			}

			speed := beep.SampleRate(float64(format.SampleRate) * float64(speedPercent) / float64(100))
			err = speaker.Init(speed, format.SampleRate.N(time.Second))
			if err != nil {
				streamer.Close()
				resp.Close()
				entry.Error(err)
				continue
			}

			speaker.Play(beep.Seq(streamer, beep.Callback(func() {
				streamer.Close()
				resp.Close()
				done <- true
			})))
			<-done
		}
	}()
	return nil
}

func (a TTSWatsonAdapter) messageAudio(message string) (io.ReadCloser, error) {
	if !a.config.TtsWatsonEnableCache {
		return a.messageAudioWatson(message)
	}
	tmpDir := filepath.Join(os.TempDir(), "gubot-tts")
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		return nil, err
	}
	ns := uuid.NewV5(uuid.FromStringOrNil(ttsWatsonNS), message)
	audFile := filepath.Join(tmpDir, fmt.Sprintf("%s.ogg", ns))
	if _, err := os.Stat(audFile); err == nil {
		return os.Open(audFile)
	}

	f, err := os.Create(audFile)
	if err != nil {
		return nil, err
	}
	audWatson, err := a.messageAudioWatson(message)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(f, audWatson)
	if err != nil {
		audWatson.Close()
		f.Close()
		return nil, err
	}
	audWatson.Close()
	f.Close()

	return os.Open(audFile)
}

func (a TTSWatsonAdapter) messageAudioWatson(message string) (io.ReadCloser, error) {
	jsonMessage, err := json.Marshal(TTSRequest{
		Text: message,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/v1/synthesize", a.config.TtsWatsonUrl),
		bytes.NewBuffer(jsonMessage),
	)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth("apikey", a.config.TtsWatsonToken)
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("accept", fmt.Sprintf("audio/ogg;codecs=vorbis;rate=%d", a.config.TtsWatsonRate))
	q := req.URL.Query()
	if a.config.TtsWatsonVoice != "" {
		q.Set("voice", a.config.TtsWatsonVoice)
	}
	req.URL.RawQuery = q.Encode()
	resp, err := robot.HttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (TTSWatsonAdapter) Config() interface{} {
	return TTSWatsonConfig{}
}
