//go:build noaudio && !darwin

package notifications

import (
	"os"
	"os/exec"
	"runtime"

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
			if err := playSoundFile(soundFile); err == nil {
				return nil
			}
		}

		// Fall back to regular notification sound if airdrop sound is not available or fails
		soundFile = cfg.Notifications.Sound.File
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
	// Audio disabled for cross-compilation builds, but try to play using system commands
	if _, err := os.Stat(filePath); err != nil {
		return err
	}

	// Try to play the sound file using system commands
	switch runtime.GOOS {
	case "linux":
		// Try using aplay, paplay, or mpg123 for Linux
		commands := []string{"paplay", "aplay", "mpg123", "ogg123"}
		for _, cmd := range commands {
			if err := exec.Command(cmd, filePath).Run(); err == nil {
				return nil
			}
		}
	case "windows":
		// On Windows, try using PowerShell to play the sound
		psCmd := "(New-Object Media.SoundPlayer '" + filePath + "').PlaySync();"
		if err := exec.Command("powershell", "-c", psCmd).Run(); err == nil {
			return nil
		}
	}

	// Fall back to system beep if sound file couldn't be played
	return beeep.Beep(beeep.DefaultFreq, 500)
}
