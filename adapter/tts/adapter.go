package tts

import (
	"fmt"
	"github.com/ArthurHlt/gubot/robot"
	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/vorbis"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"github.com/writeas/go-strip-markdown"
	"io"
	"os"
	"path/filepath"
	"time"
)

const ttsNS = "7c4e25d3-f9ea-4b64-b5e5-739bd35f556c"

func init() {
	robot.RegisterAdapter(&TTSAdapter{
		messageChan: make(chan string, 100),
	})
}

type TTSConfig struct {
	TtsSpeedPercent int
	TtsEnableCache  bool
	TtsWatson       WatsonConfig
	TtsVoicerss     VoicerssConfig
}

type TTSAdapter struct {
	config      *TTSConfig
	gubot       *robot.Gubot
	messageChan chan string
}

func (TTSAdapter) Name() string {
	return "TTS"
}

func (a TTSAdapter) Send(_ robot.Envelop, message string) error {
	a.messageChan <- stripmd.Strip(message)
	return nil
}

func (a TTSAdapter) Reply(envelop robot.Envelop, message string) error {
	return a.Send(envelop, fmt.Sprintf("%s, %s", envelop.User.Name, message))
}

func (a *TTSAdapter) Run(config interface{}, r *robot.Gubot) error {
	entry := log.WithField("adapter", "tts")
	a.config = config.(*TTSConfig)
	if a.config.TtsSpeedPercent <= 0 {
		a.config.TtsSpeedPercent = 100
	}
	prov := RetrieveProvider(a.config)
	if prov == nil {
		return fmt.Errorf("You must define a provider")
	}
	a.gubot = r
	speedPercent := a.config.TtsSpeedPercent
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

func (a TTSAdapter) messageAudio(message string) (io.ReadCloser, error) {
	prov := RetrieveProvider(a.config)
	if !a.config.TtsEnableCache {
		return prov.MessageAudio(message, a.config)
	}
	tmpDir := filepath.Join(os.TempDir(), "gubot-tts")
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		return nil, err
	}
	ns := uuid.NewV5(uuid.FromStringOrNil(ttsNS), message)
	audFile := filepath.Join(tmpDir, fmt.Sprintf("%s.ogg", ns))
	if _, err := os.Stat(audFile); err == nil {
		return os.Open(audFile)
	}

	f, err := os.Create(audFile)
	if err != nil {
		return nil, err
	}
	audWatson, err := prov.MessageAudio(message, a.config)
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

func (TTSAdapter) Config() interface{} {
	return TTSConfig{}
}
