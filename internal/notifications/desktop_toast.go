//go:build !noaudio && !darwin

package notifications

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ayn2op/discordo/internal/config"
	"github.com/ebitengine/oto/v3"
	"github.com/gen2brain/beeep"
	"github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/go-vorbis"
)

func sendDesktopNotification(title string, message string, image string, playSound bool, duration int) error {
	if err := beeep.Notify(title, message, image); err != nil {
		return err
	}

	if playSound {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return beeep.Beep(beeep.DefaultFreq, duration)
		}

		soundFile := cfg.Notifications.Sound.File
		if soundFile != "" {
			if err := playSoundFile(soundFile); err == nil {
				return nil
			}
		}

		return beeep.Beep(beeep.DefaultFreq, duration)
	}

	return nil
}

func playSoundFile(filePath string) error {
	if _, err := os.Stat(filePath); err != nil {
		return err
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   44100,
		ChannelCount: 2,
		Format:       oto.FormatFloat32LE,
	})
	if err != nil {
		return err
	}
	<-ready

	var decoder interface {
		Read([]byte) (int, error)
	}

	switch ext {
	case ".mp3":
		mp3Decoder, err := mp3.NewDecoder(file)
		if err != nil {
			return err
		}
		decoder = mp3Decoder
	case ".ogg", ".oga":
		oggDecoder, _, _, err := vorbis.Decode(file)
		if err != nil {
			return err
		}
		decoder = oggDecoder
	default:
		// For unsupported formats, close the file and return an error to trigger fallback
		return fmt.Errorf("unsupported audio format: %s", ext)
	}

	player := ctx.NewPlayer(decoder)
	player.Play()
	for player.IsPlaying() {
	}
	return player.Close()
}
