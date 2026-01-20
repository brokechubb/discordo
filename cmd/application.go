package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/ayn2op/discordo/internal/config"
	"github.com/ayn2op/discordo/internal/keyring"
	"github.com/ayn2op/discordo/internal/login"
	"github.com/ayn2op/discordo/internal/ui"
	"github.com/ayn2op/tview"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/gdamore/tcell/v2"
)

const (
	flexPageName            = "flex"
	mentionsListPageName    = "mentionsList"
	attachmentsListPageName = "attachmentsList"
	confirmModalPageName    = "confirmModal"
)

type application struct {
	cfg *config.Config

	*tview.Application
	pages             *tview.Pages
	flex              *tview.Flex
	guildsTree        *guildsTree
	messagesList      *messagesList
	messageInput      *messageInput
	notificationArea  *tview.TextView
	tipccStatusArea   *tview.TextView // Tip.cc autoclaim status indicator
	selectedChannelID discord.ChannelID
	selectedGuildID   discord.GuildID
}

func newApplication(cfg *config.Config) *application {
	// Terminal optimizations are now handled in root.go via ui.ApplyKittyOptimizations()
	// before the application is created

	app := &application{
		cfg: cfg,

		Application:  tview.NewApplication(),
		pages:        tview.NewPages(),
		flex:         tview.NewFlex(),
		guildsTree:   newGuildsTree(cfg),
		messagesList: newMessagesList(cfg),
		messageInput: newMessageInput(cfg),
	}

	app.pages.SetInputCapture(app.onPagesInputCapture)
	app.
		EnableMouse(cfg.Mouse).
		SetInputCapture(app.onInputCapture).
		EnablePaste(true)
	return app
}

func (a *application) run(token string) error {
	if token == "" {
		loginForm := login.NewForm(a.Application, a.cfg, func(token string) {
			if err := a.run(token); err != nil {
				slog.Error("failed to run application", "err", err)
			}
		})

		return a.SetRoot(loginForm, true).Run()
	}

	if err := openState(token); err != nil {
		return err
	}

	a.init()
	return a.SetRoot(a.pages, true).Run()
}

func (a *application) quit() {
	if discordState != nil {
		if err := discordState.Close(); err != nil {
			slog.Error("failed to close the session", "err", err)
		}
	}

	a.Stop()
}

func (a *application) init() {
	a.pages.Clear()
	a.flex.Clear()

	// Create notification area
	a.notificationArea = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false).
		SetWordWrap(false).
		SetScrollable(false)
	a.notificationArea.Box = ui.ConfigureBox(a.notificationArea.Box, &a.cfg.Theme)
	a.notificationArea.SetTitle("Notifications")

	// Create Tip.cc status area
	a.tipccStatusArea = tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false).
		SetWordWrap(false)
	a.tipccStatusArea.Box = ui.ConfigureBox(a.tipccStatusArea.Box, &a.cfg.Theme)
	a.tipccStatusArea.SetTitle("Tip.cc")

	// Initialize Tip.cc status display
	a.updateTipCCStatus()

	right := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.messagesList, 0, 1, false).
		AddItem(a.messageInput, 3, 1, false)

	// Create a top bar with notification and Tip.cc status
	topBar := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(a.notificationArea, 0, 4, false). // Notifications take 4/5 of space
		AddItem(a.tipccStatusArea, 0, 1, false)   // Tip.cc status takes 1/5 of space

	// Use a.flex for the main content area (guilds tree + right panel)
	a.flex.SetDirection(tview.FlexColumn).
		AddItem(a.guildsTree, 0, 3, true). // Guilds tree
		AddItem(right, 0, 7, false)        // Right panel

	// Create main layout with top bar and content
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(topBar, 3, 1, false). // Top bar with notifications and Tip.cc status
		AddItem(a.flex, 0, 1, false)  // Main content area using a.flex

	a.pages.AddAndSwitchToPage(flexPageName, mainFlex, true)
}

func (a *application) onInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Name() {
	case a.cfg.Keys.Quit:
		a.quit()
		return nil
	case a.cfg.Keys.ClearNotification:
		a.ClearNotification()
		return nil
	case a.cfg.Keys.ToggleTipCCAutoClaim:
		// Add safety check to prevent freezing
		if a.cfg.Keys.ToggleTipCCAutoClaim != "" && a.tipccStatusArea != nil {
			go func() {
				a.toggleTipCCAutoClaim()
			}()
		}
		return nil
	case a.cfg.Keys.ToggleTriviaDrop:
		// Add safety check to prevent freezing
		if a.cfg.Keys.ToggleTriviaDrop != "" && a.tipccStatusArea != nil {
			go func() {
				a.toggleTriviaDrop()
			}()
		}
		return nil
	case "Ctrl+C":
		// https://github.com/ayn2op/tview/blob/a64fc48d7654432f71922c8b908280cdb525805c/application.go#L153
		return tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone)
	}

	return event
}

func (a *application) onPagesInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Name() {
	case a.cfg.Keys.FocusGuildsTree:
		a.messageInput.removeMentionsList()
		a.focusGuildsTree()
		return nil
	case a.cfg.Keys.FocusMessagesList:
		a.messageInput.removeMentionsList()
		a.SetFocus(a.messagesList)
		return nil
	case a.cfg.Keys.FocusMessageInput:
		a.focusMessageInput()
		return nil
	case a.cfg.Keys.FocusPrevious:
		a.focusPrevious()
		return nil
	case a.cfg.Keys.FocusNext:
		a.focusNext()
		return nil
	case a.cfg.Keys.Logout:
		a.quit()

		if err := keyring.DeleteToken(); err != nil {
			slog.Error("failed to delete token from keyring", "err", err)
			return nil
		}

		return nil
	case a.cfg.Keys.ToggleGuildsTree:
		a.toggleGuildsTree()
		return nil
	}

	return event
}

func (a *application) toggleGuildsTree() {
	// The guilds tree is visible if the number of items is two.
	if a.flex.GetItemCount() == 2 {
		a.flex.RemoveItem(a.guildsTree)
		if a.guildsTree.HasFocus() {
			a.SetFocus(a.flex)
		}
	} else {
		a.init()
		a.SetFocus(a.guildsTree)
	}
}

func (a *application) focusGuildsTree() bool {
	// The guilds tree is not hidden if the number of items is two.
	// Check if the guilds tree is visible in the flex container
	if a.flex != nil && a.flex.GetItemCount() >= 2 {
		a.SetFocus(a.guildsTree)
		return true
	}

	return false
}

func (a *application) focusMessageInput() bool {
	if !a.messageInput.GetDisabled() {
		a.SetFocus(a.messageInput)
		return true
	}

	return false
}

func (a *application) focusPrevious() {
	switch a.GetFocus() {
	case a.guildsTree:
		a.SetFocus(a.messageInput)
	case a.messagesList: // Handle both a.messagesList and a.flex as well as other edge cases (if there is).
		if ok := a.focusGuildsTree(); !ok {
			a.SetFocus(a.messageInput)
		}
	case a.messageInput:
		a.SetFocus(a.messagesList)
	}
}

func (a *application) focusNext() {
	switch a.GetFocus() {
	case a.guildsTree:
		a.SetFocus(a.messagesList)
	case a.messagesList:
		a.SetFocus(a.messageInput)
	case a.messageInput: // Handle both a.messageInput and a.flex as well as other edge cases (if there is).
		if ok := a.focusGuildsTree(); !ok {
			a.SetFocus(a.messagesList)
		}
	}
}

func (a *application) showConfirmModal(prompt string, buttons []string, onDone func(label string)) {
	previousFocus := a.GetFocus()

	modal := tview.NewModal().
		SetText(prompt).
		AddButtons(buttons).
		SetDoneFunc(func(_ int, buttonLabel string) {
			a.pages.RemovePage(confirmModalPageName).SwitchToPage(flexPageName)
			a.SetFocus(previousFocus)

			if onDone != nil {
				onDone(buttonLabel)
			}
		})

	a.pages.
		AddAndSwitchToPage(confirmModalPageName, ui.Centered(modal, 0, 0), true).
		ShowPage(flexPageName)
}

// ShowNotification displays a temporary notification in the notification area
func (a *application) ShowNotification(message string) {
	duration := 30 * time.Second // Default duration
	if a.cfg.Notifications.Duration > 0 {
		duration = time.Duration(a.cfg.Notifications.Duration) * time.Second
	}
	a.showNotification(message, duration)
}

// showNotification handles notifications
func (a *application) showNotification(message string, duration time.Duration) {
	if a.notificationArea != nil {
		// Clear previous content and add new notification
		a.notificationArea.Clear()

		// Truncate message to fit on one line
		maxMessageLen := 100 // More room without "Notification:" prefix
		if len(message) > maxMessageLen {
			message = message[:maxMessageLen] + "..."
		}

		fmt.Fprintf(a.notificationArea, "%s", message)

		// Scroll to top to ensure first line is always visible
		a.notificationArea.ScrollToBeginning()

		// Auto-clear notifications after duration
		if duration > 0 {
			go func() {
				time.Sleep(duration)
				if a.notificationArea != nil {
					// Use direct update instead of QueueUpdateDraw to avoid potential deadlocks
					a.notificationArea.Clear()
				}
			}()
		}
	}
}

// Just call the regular showNotification method since we removed the click functionality
func (a *application) showNotificationWithInfo(message string, duration time.Duration, channelID discord.ChannelID, guildID discord.GuildID, isDM bool) {
	// Strip out channel/guild context and just show the message like a regular notification
	a.showNotification(message, duration)
}

// ClearNotification clears the current notification
func (a *application) ClearNotification() {
	if a.notificationArea != nil {
		// Use direct update instead of QueueUpdateDraw to avoid potential deadlocks
		a.notificationArea.Clear()
		a.notificationArea.ScrollToBeginning()
	}
}

// updateTipCCStatus updates the Tip.cc autoclaim status indicator
func (a *application) updateTipCCStatus() {
	if a == nil || a.tipccStatusArea == nil || a.cfg == nil {
		return
	}

	// Debug logging to troubleshoot triviadrop status
	slog.Debug("Updating Tip.cc status",
		"auto_claim", a.cfg.TipCC.AutoClaim,
		"triviadrop_enabled", a.cfg.TipCC.TriviaDrop,
		"triviadrop_strategy", a.cfg.TipCC.TriviaStrategy,
		"triviadrop_delay", a.cfg.TipCC.TriviaDelay,
	)

	// Use a simple update instead of QueueUpdateDraw to avoid potential deadlocks
	a.tipccStatusArea.Clear()

	if a.cfg.TipCC.AutoClaim {
		status := "[::b][green]AUTOCLAIM[-]"
		if a.cfg.TipCC.TriviaDrop {
			status += " [::b][green]●[-] [cyan]🧩[-]"
		} else {
			status += " [::b][green]●[-]"
		}
		a.tipccStatusArea.Write([]byte(status))
	} else {
		var status string
		if a.cfg.TipCC.TriviaDrop {
			status = "[::b][red]AUTOCLAIM[-] [::b][red]●[-] [cyan]🧩[-]"
		} else {
			status = "[::b][red]AUTOCLAIM OFF[-]"
		}
		a.tipccStatusArea.Write([]byte(status))
	}
}

// toggleTipCCAutoClaim toggles the Tip.cc autoclaim feature
func (a *application) toggleTipCCAutoClaim() {
	if a == nil || a.cfg == nil {
		return
	}

	a.cfg.TipCC.AutoClaim = !a.cfg.TipCC.AutoClaim
	a.updateTipCCStatus()

	statusIndicator := "[red]●[-]"
	statusText := "disabled"
	if a.cfg.TipCC.AutoClaim {
		statusIndicator = "[green]●[-]"
		statusText = "enabled"
	}

	// Show notification without using complex UI updates that might cause deadlocks
	if a.notificationArea != nil {
		a.notificationArea.Clear()
		notificationText := fmt.Sprintf("Tip.cc auto-claim %s %s (Ctrl+A to toggle)", statusIndicator, statusText)

		// Truncate to fit on one line
		maxMessageLen := 80
		if len(notificationText) > maxMessageLen {
			notificationText = notificationText[:maxMessageLen] + "..."
		}

		fmt.Fprintf(a.notificationArea, "[::b]Notification:[-::-] %s", notificationText)
		a.notificationArea.ScrollToBeginning()
	}

	slog.Info("Tip.cc auto-claim toggled", "status", statusText)
}

// toggleTriviaDrop toggles the Tip.cc triviadrop feature
func (a *application) toggleTriviaDrop() {
	if a == nil || a.cfg == nil {
		return
	}

	a.cfg.TipCC.TriviaDrop = !a.cfg.TipCC.TriviaDrop
	a.updateTipCCStatus()

	statusIndicator := "[red]●[-]"
	statusText := "disabled"
	if a.cfg.TipCC.TriviaDrop {
		statusIndicator = "[green]●[-]"
		statusText = "enabled"
	}

	// Show notification without using complex UI updates that might cause deadlocks
	if a.notificationArea != nil {
		a.notificationArea.Clear()
		notificationText := fmt.Sprintf("Tip.cc triviadrop %s %s (Ctrl+T to toggle)", statusIndicator, statusText)

		// Truncate to fit on one line
		maxMessageLen := 80
		if len(notificationText) > maxMessageLen {
			notificationText = notificationText[:maxMessageLen] + "..."
		}

		fmt.Fprintf(a.notificationArea, "[::b]Notification:[-::-] %s", notificationText)
		a.notificationArea.ScrollToBeginning()
	}

	slog.Info("Tip.cc triviadrop toggled", "status", statusText)
}
