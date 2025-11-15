//go:build noaudio && !darwin

package notifications

import (
	"github.com/ayn2op/discordo/internal/config"
	"github.com/gen2brain/beeep"
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
	// Audio disabled for cross-compilation builds
	// Fall back to system beep
	return beeep.Beep(beeep.DefaultFreq, 500)
}
