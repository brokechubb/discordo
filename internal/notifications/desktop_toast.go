//go:build !noaudio && !darwin

package notifications

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ayn2op/discordo/internal/config"
	"github.com/ebitengine/oto/v3"
	"github.com/gen2brain/beeep"
	"github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/go-vorbis"
)

// AudioPlayer manages audio playback to prevent conflicts and tearing
type AudioPlayer struct {
	ctx    *oto.Context
	player *oto.Player
	mutex  sync.Mutex
	cond   *sync.Cond
	busy   bool
}

var globalAudioPlayer *AudioPlayer

func init() {
	// Initialize global audio player with a single context
	var err error
	globalAudioPlayer, err = NewAudioPlayer()
	if err != nil {
		// If we can't initialize audio, we'll fall back to beep in the play functions
		globalAudioPlayer = nil
	}
}

func NewAudioPlayer() (*AudioPlayer, error) {
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   44100,
		ChannelCount: 2,
		Format:       oto.FormatFloat32LE,
	})
	if err != nil {
		return nil, err
	}
	<-ready

	ap := &AudioPlayer{
		ctx:  ctx,
		busy: false,
	}
	ap.cond = sync.NewCond(&ap.mutex)

	// Start a goroutine to keep the context alive
	go func() {
		// Context needs to stay alive for the application lifetime
		// We'll not close it here since we want it to persist
	}()

	return ap, nil
}

func (ap *AudioPlayer) PlayFile(filePath string) error {
	if ap == nil {
		return fmt.Errorf("audio player not initialized")
	}

	ap.mutex.Lock()

	// Wait until player is not busy
	for ap.busy {
		ap.cond.Wait()
	}

	ap.busy = true
	ap.mutex.Unlock()

	defer func() {
		ap.mutex.Lock()
		ap.busy = false
		ap.cond.Broadcast()
		ap.mutex.Unlock()
	}()

	// Load and play the file using the shared context
	return ap.playFileInternal(filePath)
}

func (ap *AudioPlayer) playFileInternal(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(filePath))

	var decoder io.Reader

	switch ext {
	case ".mp3":
		mp3Decoder, err := mp3.NewDecoder(file)
		if err != nil {
			return err
		}
		decoder = mp3Decoder
	case ".ogg", ".oga":
		vorbisStream, _, _, err := vorbis.Decode(file)
		if err != nil {
			return err
		}
		decoder = vorbisStream
	default:
		return fmt.Errorf("unsupported audio format: %s", ext)
	}

	// Create a new player using the shared context
	player := ap.ctx.NewPlayer(decoder)
	defer player.Close()

	player.Play()

	// Wait for playback to complete with timeout
	done := make(chan bool, 1)
	go func() {
		for player.IsPlaying() {
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second): // 5 second timeout to prevent hanging
		return fmt.Errorf("playback timeout")
	}
}

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
			// Try to play using the global audio player
			if globalAudioPlayer != nil {
				if err := globalAudioPlayer.PlayFile(soundFile); err == nil {
					return nil
				}
			}
			// Fallback to the old method if audio player fails
			if err := playSoundFileFallback(soundFile); err == nil {
				return nil
			}
		}

		return beeep.Beep(beeep.DefaultFreq, duration)
	}

	return nil
}

func sendDesktopNotificationAirdrop(title string, message string, image string, playSound bool, duration int) error {
	if err := beeep.Notify(title, message, image); err != nil {
		return err
	}

	if playSound {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			return beeep.Beep(beeep.DefaultFreq, duration)
		}

		// Use airdrop-specific sound file if configured, otherwise fall back to regular sound
		soundFile := cfg.Notifications.Sound.AirdropFile
		if soundFile != "" {
			// Try to play using the global audio player
			if globalAudioPlayer != nil {
				if err := globalAudioPlayer.PlayFile(soundFile); err == nil {
					return nil
				}
			}
			// Fallback to the old method if audio player fails
			if err := playSoundFileFallback(soundFile); err == nil {
				return nil
			}
		}

		// Fall back to regular notification sound if airdrop sound is not available or fails
		soundFile = cfg.Notifications.Sound.File
		if soundFile != "" {
			// Try to play using the global audio player
			if globalAudioPlayer != nil {
				if err := globalAudioPlayer.PlayFile(soundFile); err == nil {
					return nil
				}
			}
			// Fallback to the old method if audio player fails
			if err := playSoundFileFallback(soundFile); err == nil {
				return nil
			}
		}

		return beeep.Beep(beeep.DefaultFreq, duration)
	}

	return nil
}

// playSoundFileFallback is the original implementation for when the global audio player is not available
func playSoundFileFallback(filePath string) error {
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

	// Use a safer approach to wait for playback to complete
	// Limit the number of checks to avoid indefinite blocking
	for i := 0; i < 200; i++ { // Limit checks to avoid hanging
		if !player.IsPlaying() {
			break
		}
		time.Sleep(10 * time.Millisecond) // Brief sleep to avoid busy waiting
	}

	return player.Close()
}
