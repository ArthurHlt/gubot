package tts_watson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ArthurHlt/gubot/robot"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/writeas/go-strip-markdown"
	"net/http"
	"time"
)

func init() {
	robot.RegisterAdapter(&TTSWatsonAdapter{})
}

type TTSWatsonConfig struct {
	TtsWatsonToken        string
	TtsWatsonUrl          string
	TtsWatsonVoice        string
	TtsWatsonSpeedPercent int
}

type TTSWatsonAdapter struct {
	config *TTSWatsonConfig
	gubot  *robot.Gubot
}

func (TTSWatsonAdapter) Name() string {
	return "TTS_Watson"
}

type TTSRequest struct {
	Text string `json:"text"`
}

func (a TTSWatsonAdapter) Send(_ robot.Envelop, message string) error {
	jsonMessage, err := json.Marshal(TTSRequest{
		Text: stripmd.Strip(message),
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("%s/v1/synthesize", a.config.TtsWatsonUrl),
		bytes.NewBuffer(jsonMessage),
	)
	if err != nil {
		return err
	}
	req.SetBasicAuth("apikey", a.config.TtsWatsonToken)
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("accept", "audio/ogg;codecs=vorbis")
	q := req.URL.Query()
	if a.config.TtsWatsonVoice != "" {
		q.Set("voice", a.config.TtsWatsonVoice)
	}
	req.URL.RawQuery = q.Encode()
	resp, err := robot.HttpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	streamer, format, err := vorbis.Decode(resp.Body)
	if err != nil {
		return err
	}
	defer streamer.Close()

	speedPercent := a.config.TtsWatsonSpeedPercent
	if speedPercent <= 0 {
		speedPercent = 50
	}
	speed := beep.SampleRate(float64(format.SampleRate) * float64(speedPercent) / float64(100))

	err = speaker.Init(speed, format.SampleRate.N(time.Second/10))
	if err != nil {
		return err
	}
	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))
	<-done
	return nil
}

func (a TTSWatsonAdapter) Reply(envelop robot.Envelop, message string) error {
	return a.Send(envelop, fmt.Sprintf("%s, %s", envelop.User.Name, message))
}

func (a *TTSWatsonAdapter) Run(config interface{}, r *robot.Gubot) error {
	a.config = config.(*TTSWatsonConfig)
	a.gubot = r
	return nil
}

func (TTSWatsonAdapter) Config() interface{} {
	return TTSWatsonConfig{}
}
