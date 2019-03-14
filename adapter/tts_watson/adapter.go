package tts_watson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ArthurHlt/gubot/robot"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	log "github.com/sirupsen/logrus"
	"github.com/writeas/go-strip-markdown"
	"net/http"
	"time"
)

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
			jsonMessage, err := json.Marshal(TTSRequest{
				Text: stripmd.Strip(message),
			})
			if err != nil {
				entry.Error(err)
				continue
			}

			req, err := http.NewRequest(
				"POST",
				fmt.Sprintf("%s/v1/synthesize", a.config.TtsWatsonUrl),
				bytes.NewBuffer(jsonMessage),
			)
			if err != nil {
				entry.Error(err)
				continue
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
				entry.Error(err)
				continue
			}
			streamer, format, err := vorbis.Decode(resp.Body)
			if err != nil {
				resp.Body.Close()
				entry.Error(err)
				continue
			}

			speed := beep.SampleRate(float64(format.SampleRate) * float64(speedPercent) / float64(100))
			err = speaker.Init(speed, format.SampleRate.N(time.Second))
			if err != nil {
				entry.Error(err)
				continue
			}

			speaker.Play(beep.Seq(streamer, beep.Callback(func() {
				streamer.Close()
				resp.Body.Close()
				done <- true
			})))
			<-done
		}
	}()
	return nil
}

func (TTSWatsonAdapter) Config() interface{} {
	return TTSWatsonConfig{}
}
