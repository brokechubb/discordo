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
	pages              *tview.Pages
	flex               *tview.Flex
	guildsTree         *guildsTree
	messagesList       *messagesList
	messageInput       *messageInput
	notificationArea   *tview.TextView
	hasPersistentNotif bool // Track if there's a persistent notification
}

func newApplication(cfg *config.Config) *application {
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
		SetRegions(true).
		SetWrap(true).
		SetScrollable(false)
	a.notificationArea.Box = ui.ConfigureBox(a.notificationArea.Box, &a.cfg.Theme)
	a.notificationArea.SetTitle("Notifications")

	right := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.messagesList, 0, 1, false).
		AddItem(a.messageInput, 3, 1, false)

	// Create a main flex that includes the notification area at the top
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(a.notificationArea, 3, 1, false). // Small notification area at top
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexColumn).
			AddItem(a.guildsTree, 0, 3, true).        // Guilds tree
			AddItem(right, 0, 7, false), 0, 1, false) // Main content area

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
		_ = a.focusGuildsTree()
		return nil
	case a.cfg.Keys.FocusMessagesList:
		a.messageInput.removeMentionsList()
		a.SetFocus(a.messagesList)
		return nil
	case a.cfg.Keys.FocusMessageInput:
		if !a.messageInput.GetDisabled() {
			a.SetFocus(a.messageInput)
		}
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
	if a.flex.GetItemCount() == 2 {
		a.SetFocus(a.guildsTree)
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
	a.showNotification(message, duration, false)
}

// ShowPersistentNotification displays a persistent notification that stays until cleared
func (a *application) ShowPersistentNotification(message string) {
	a.showNotification(message, 0, true)
}

// showNotification handles both temporary and persistent notifications
func (a *application) showNotification(message string, duration time.Duration, persistent bool) {
	if a.notificationArea != nil {
		// Clear previous content and add new notification
		a.notificationArea.Clear()
		fmt.Fprintf(a.notificationArea, "[::b]Notification:[-::-] %s", message)

		// Track persistent notification state
		a.hasPersistentNotif = persistent

		// Auto-clear only for temporary notifications
		if !persistent && duration > 0 {
			go func() {
				time.Sleep(duration)
				if a.notificationArea != nil {
					a.QueueUpdateDraw(func() {
						// Only clear if no persistent notification is active
						if a.notificationArea.GetText(false) != "" && !a.hasPersistentNotif {
							a.notificationArea.Clear()
						}
					})
				}
			}()
		}
	}
}

// ClearNotification clears the current notification
func (a *application) ClearNotification() {
	if a.notificationArea != nil {
		a.QueueUpdateDraw(func() {
			a.notificationArea.Clear()
			a.hasPersistentNotif = false
		})
	}
}
