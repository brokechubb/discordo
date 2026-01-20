package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ayn2op/discordo/internal/http"
	"github.com/ayn2op/discordo/internal/notifications"
	"github.com/ayn2op/tview"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/session"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/state/store/defaultstore"
	"github.com/diamondburned/arikawa/v3/utils/handler"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/diamondburned/arikawa/v3/utils/ws"
	"github.com/diamondburned/ningen/v3"
	"github.com/diamondburned/ningen/v3/states/read"
)

func openState(token string) error {
	identifyProps := http.IdentifyProperties()

	api.UserAgent = http.BrowserUserAgent
	gateway.DefaultIdentity = identifyProps
	gateway.DefaultPresence = &gateway.UpdatePresenceCommand{
		Status: app.cfg.Status,
	}

	id := gateway.DefaultIdentifier(token)
	id.Compress = false

	session := session.NewCustom(id, http.NewClient(token), handler.New())
	state := state.NewFromSession(session, defaultstore.New())
	discordState = ningen.FromState(state)

	// Initialize interaction handler
	globalInteractionHandler = newInteractionHandler(app.cfg)

	// Handlers
	discordState.AddHandler(onRaw)
	discordState.AddHandler(onReady)
	discordState.AddHandler(onMessageCreate)
	discordState.AddHandler(onMessageUpdate)
	discordState.AddHandler(onMessageDelete)
	discordState.AddHandler(onReadUpdate)

	discordState.AddHandler(func(event *gateway.GuildMembersChunkEvent) {
		app.messagesList.setFetchingChunk(false, uint(len(event.Members)))
	})

	discordState.AddHandler(func(event *gateway.GuildMemberRemoveEvent) {
		app.messageInput.cache.Invalidate(event.GuildID.String()+" "+event.User.Username, discordState.MemberState.SearchLimit)
	})

	discordState.StateLog = func(err error) {
		slog.Error("state log", "err", err)
	}

	discordState.OnRequest = append(discordState.OnRequest, httputil.WithHeaders(http.Headers()), onRequest)
	return discordState.Open(context.TODO())
}

func onRequest(r httpdriver.Request) error {
	if req, ok := r.(*httpdriver.DefaultRequest); ok {
		slog.Debug("new HTTP request", "method", req.Method, "url", req.URL)
	}

	return nil
}

func onRaw(event *ws.RawEvent) {
	slog.Debug(
		"new raw event",
		"code", event.OriginalCode,
		"type", event.OriginalType,
		// "data", event.Raw,
	)
}

func onReadUpdate(event *read.UpdateEvent) {
	var guildNode *tview.TreeNode
	app.guildsTree.
		GetRoot().
		Walk(func(node, parent *tview.TreeNode) bool {
			switch node.GetReference() {
			case event.GuildID:
				node.SetTextStyle(app.guildsTree.getGuildNodeStyle(event.GuildID))
				guildNode = node
				return false
			case event.ChannelID:
				// private channel
				if !event.GuildID.IsValid() {
					style := app.guildsTree.getChannelNodeStyle(event.ChannelID)
					node.SetTextStyle(style)
					return false
				}
			}

			return true
		})

	if guildNode != nil {
		guildNode.Walk(func(node, parent *tview.TreeNode) bool {
			if node.GetReference() == event.ChannelID {
				node.SetTextStyle(app.guildsTree.getChannelNodeStyle(event.ChannelID))
				return false
			}

			return true
		})
	}

	app.Draw()
}

func onReady(r *gateway.ReadyEvent) {
	dmNode := tview.NewTreeNode("Direct Messages")
	root := app.guildsTree.
		GetRoot().
		ClearChildren().
		AddChild(dmNode)

	for _, folder := range r.UserSettings.GuildFolders {
		if folder.ID == 0 && len(folder.GuildIDs) == 1 {
			guild, err := discordState.Cabinet.Guild(folder.GuildIDs[0])
			if err != nil {
				slog.Error(
					"failed to get guild from state",
					"guild_id",
					folder.GuildIDs[0],
					"err",
					err,
				)
				continue
			}

			app.guildsTree.createGuildNode(root, *guild)
		} else {
			app.guildsTree.createFolderNode(folder)
		}
	}

	app.guildsTree.SetCurrentNode(root)
	app.SetFocus(app.guildsTree)
	app.Draw()
}

func onMessageCreate(message *gateway.MessageCreateEvent) {
	if app.guildsTree.selectedChannelID == message.ChannelID {
		app.messagesList.drawMessage(message.Message)
		app.Draw()
	}

	// Detect buttons in tip.cc messages and handle drops automatically if enabled (only in focused channel)
	if globalInteractionHandler != nil {
		// Only detect and auto-click buttons in the focused channel to prevent claiming airdrops in all servers
		if app.guildsTree.selectedChannelID == message.ChannelID {
			globalInteractionHandler.detectButtonsInMessage(message.Message, message.ChannelID)
		}

		// Auto-handle tip.cc drops if enabled in config and in focused channel
		const tipCCBotID = 617037497574359050
		if message.Author.ID == discord.UserID(tipCCBotID) {
			if (app.cfg.TipCC.AutoClaim || app.cfg.TipCC.TriviaDrop) && app.guildsTree.selectedChannelID == message.ChannelID {
				go func() {
					// Add delay if configured
					if app.cfg.TipCC.Delay > 0 {
						time.Sleep(time.Duration(app.cfg.TipCC.Delay) * time.Millisecond)
					}
					if err := globalInteractionHandler.handleTipCCDropMessage(message.Message, message.ChannelID); err != nil {
						slog.Error("failed to handle tip.cc drop", "err", err, "message_id", message.ID)
					} else {
						// Check if it is actually an airdrop before notifying
						if globalInteractionHandler.IsTipCCAirdropMessage(message.Message) {
							// Get channel name
							channel, err := discordState.Cabinet.Channel(message.ChannelID)
							channelName := message.ChannelID.String()
							if err == nil && channel.Name != "" {
								channelName = channel.Name
							}

							// Show regular notification when tip.cc drop is detected
							slog.Info("Tip.cc drop detected in focused channel (user account cannot auto-claim)", "message_id", message.ID, "channel_id", message.ChannelID)
							msg := fmt.Sprintf("[Tip.cc] Drop in #%s", channelName)
							app.ShowNotification(msg)

							// Also show desktop notification (toast) with sound
							if app.cfg.Notifications.Enabled {
								go func() {
									_ = notifications.SendAirdrop("Tip.cc Airdrop", msg, true, app.cfg.Notifications.Duration)
								}()
							}
						}
					}
				}()
			}
		}
	}

	// Check for mentions and DMs for on-screen notifications
	mentions := discordState.MessageMentions(&message.Message)
	channel, err := discordState.Cabinet.Channel(message.ChannelID)
	if err == nil {
		isDM := channel.Type == discord.DirectMessage || channel.Type == discord.GroupDM

		if isDM {
			app.ShowNotification(formatDMNotification(message))
		} else if mentions > 0 {
			app.ShowNotification(fmt.Sprintf("[#%s] [::b]%s[::-]: %s", channel.Name, message.Author.Username, truncateString(message.Content, 50)))
		}
	}

	if err := notifications.Notify(discordState, message, app.cfg); err != nil {
		slog.Error("Notification failed", "err", err)
	}
}

func onMessageUpdate(message *gateway.MessageUpdateEvent) {
	if app.guildsTree.selectedChannelID == message.ChannelID {
		onMessageDelete(&gateway.MessageDeleteEvent{ID: message.ID, ChannelID: message.ChannelID, GuildID: message.GuildID})
	}

	// Check for button updates in tip.cc messages (only in focused channel)
	if globalInteractionHandler != nil {
		// Only detect and auto-click buttons in the focused channel to prevent claiming airdrops in all servers
		if app.guildsTree.selectedChannelID == message.ChannelID {
			// Get the full message from state
			fullMessage, err := discordState.Cabinet.Message(message.ChannelID, message.ID)
			if err == nil {
				globalInteractionHandler.detectButtonsInMessage(*fullMessage, message.ChannelID)

				// Auto-handle tip.cc drops on update if enabled (handles delayed buttons)
				const tipCCBotID = 617037497574359050
				if fullMessage.Author.ID == discord.UserID(tipCCBotID) {
					if (app.cfg.TipCC.AutoClaim || app.cfg.TipCC.TriviaDrop) && app.guildsTree.selectedChannelID == message.ChannelID {
						go func() {
							// No delay on update? Or keep delay?
							// Updates might happen frequently, but usually for buttons appearing it's once.
							// Let's keep consistent with Create logic
							if app.cfg.TipCC.Delay > 0 {
								time.Sleep(time.Duration(app.cfg.TipCC.Delay) * time.Millisecond)
							}
							if err := globalInteractionHandler.handleTipCCDropMessage(*fullMessage, message.ChannelID); err != nil {
								slog.Error("failed to handle tip.cc drop on update", "err", err, "message_id", message.ID)
							}
						}()
					}
				}
			}
		}
	}
}

// truncateString truncates a string to a maximum length, adding ... if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// getMessageType returns the type of message content for notification formatting
func getMessageType(message *gateway.MessageCreateEvent) string {
	// Check for attachments first (images, files, etc.)
	if len(message.Attachments) > 0 {
		if len(message.Attachments) == 1 {
			return "[Image]"
		}
		return fmt.Sprintf("[%d attachments]", len(message.Attachments))
	}

	// Check for embeds
	if len(message.Embeds) > 0 {
		return "[Embed]"
	}

	// Check for stickers
	if len(message.Stickers) > 0 {
		return "[Sticker]"
	}

	// Check for reactions (might indicate interesting content)
	if len(message.Reactions) > 0 {
		return "[Reaction]"
	}

	// Default to text content
	return ""
}

// smartTruncate truncates string at word boundaries for better readability
func smartTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	// Try to find last space before maxLen to cut at word boundary
	lastSpace := strings.LastIndex(s[:maxLen], " ")
	if lastSpace > maxLen/2 { // Only cut at space if it's not too early
		return s[:lastSpace] + "..."
	}

	// Fallback to character truncation if no good word boundary
	return s[:maxLen] + "..."
}

// formatDMNotification creates an improved notification message for DMs
func formatDMNotification(message *gateway.MessageCreateEvent) string {
	username := message.Author.Username
	content := strings.TrimSpace(message.Content)

	// Check for recent DM from same user (within 2 minutes)
	now := time.Now()
	if lastTime, exists := lastDMNotification[message.Author.ID]; exists {
		if now.Sub(lastTime) < 2*time.Minute {
			// Increment message count for this user
			dmMessageCount[message.Author.ID]++

			count := dmMessageCount[message.Author.ID]

			// Get message type indicator
			msgType := getMessageType(message)

			// Format based on content type and count
			if msgType != "" && content == "" {
				// Pure media message with count
				return fmt.Sprintf("[DM] [::b]%s[::-] (%d): %s", username, count, msgType)
			} else if msgType != "" {
				// Mixed content with count
				truncated := smartTruncate(content, 40)
				return fmt.Sprintf("[DM] [::b]%s[::-] (%d): %s %s", username, count, msgType, truncated)
			} else if content == "" {
				// Empty content with count
				return fmt.Sprintf("[DM] [::b]%s[::-] (%d): <empty>", username, count)
			} else {
				// Regular text with count indicator
				if count <= 3 {
					// Show actual content for low counts
					truncated := smartTruncate(content, 50)
					return fmt.Sprintf("[DM] [::b]%s[::-] (%d): %s", username, count, truncated)
				} else {
					// Show "N more messages" for higher counts
					truncated := smartTruncate(content, 35)
					return fmt.Sprintf("[DM] [::b]%s[::-]: %s [+%d msgs]", username, truncated, count-1)
				}
			}
		}
	}

	// Reset count for new message series and update time
	dmMessageCount[message.Author.ID] = 1
	lastDMNotification[message.Author.ID] = now

	// Get message type indicator
	msgType := getMessageType(message)

	// Handle different message types
	if msgType != "" {
		if content == "" {
			// Pure media/file message
			return fmt.Sprintf("[DM] [::b]%s[::-]: %s", username, msgType)
		} else {
			// Mixed content - show type indicator + truncated content
			truncated := smartTruncate(content, 50) // More room for content now
			return fmt.Sprintf("[DM] [::b]%s[::-]: %s %s", username, msgType, truncated)
		}
	}

	// Pure text message
	if content == "" {
		return fmt.Sprintf("[DM] [::b]%s[::-]", username)
	}

	// Check for mentions in the content (important even in DMs)
	if me, err := discordState.Cabinet.Me(); err == nil && strings.Contains(content, "@"+me.Username) {
		truncated := smartTruncate(content, 50)
		return fmt.Sprintf("[DM] [::b]@%s[::-]: %s", username, truncated)
	}

	// Regular text message
	truncated := smartTruncate(content, 60) // More room for content with cleaner format
	return fmt.Sprintf("[DM] [::b]%s[::-]: %s", username, truncated)
}

func onMessageDelete(message *gateway.MessageDeleteEvent) {
	if app.guildsTree.selectedChannelID == message.ChannelID {
		messages, err := discordState.Cabinet.Messages(message.ChannelID)
		if err != nil {
			slog.Error("failed to get messages from state", "err", err, "channel_id", message.ChannelID)
			return
		}

		app.messagesList.reset()
		app.messagesList.drawMessages(messages)
		app.Draw()
	}
}
