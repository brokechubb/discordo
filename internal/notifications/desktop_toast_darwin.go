//go:build darwin

package notifications

import (
	"os"

	"github.com/ayn2op/discordo/internal/config"
	gosxnotifier "github.com/deckarep/gosx-notifier"
)

func sendDesktopNotification(title string, message string, image string, playSound bool, _ int) error {
	n := gosxnotifier.NewNotification(message)
	n.Title = title
	n.ContentImage = image

	if playSound {
		cfg, err := config.Load(config.DefaultPath())
		if err != nil {
			n.Sound = gosxnotifier.Default
			return n.Push()
		}

		soundFile := cfg.Notifications.Sound.File
		if soundFile != "" {
			if _, err := os.Stat(soundFile); err == nil {
				n.Sound = gosxnotifier.Sound(soundFile)
			} else {
				n.Sound = gosxnotifier.Default
			}
		} else {
			n.Sound = gosxnotifier.Default
		}
	}

	return n.Push()
}
