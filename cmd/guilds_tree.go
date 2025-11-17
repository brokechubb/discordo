package cmd

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/ayn2op/discordo/internal/config"
	"github.com/ayn2op/discordo/internal/ui"
	"github.com/ayn2op/tview"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/ningen/v3"
	"github.com/gdamore/tcell/v2"
	"golang.design/x/clipboard"
)

type guildsTree struct {
	*tview.TreeView
	cfg               *config.Config
	selectedChannelID discord.ChannelID
	selectedGuildID   discord.GuildID
}

func newGuildsTree(cfg *config.Config) *guildsTree {
	gt := &guildsTree{
		TreeView: tview.NewTreeView(),
		cfg:      cfg,
	}

	gt.Box = ui.ConfigureBox(gt.Box, &cfg.Theme)
	gt.
		SetRoot(tview.NewTreeNode("")).
		SetTopLevel(1).
		SetGraphics(cfg.Theme.GuildsTree.Graphics).
		SetGraphicsColor(tcell.GetColor(cfg.Theme.GuildsTree.GraphicsColor)).
		SetSelectedFunc(gt.onSelected).
		SetTitle("Guilds").
		SetInputCapture(gt.onInputCapture)

	return gt
}

func (gt *guildsTree) createFolderNode(folder gateway.GuildFolder) {
	name := "Folder"
	if folder.Name != "" {
		name = fmt.Sprintf("[%s]%s[-]", folder.Color, folder.Name)
	}

	folderNode := tview.NewTreeNode(name).SetExpanded(gt.cfg.Theme.GuildsTree.AutoExpandFolders)
	gt.GetRoot().AddChild(folderNode)

	for _, gID := range folder.GuildIDs {
		guild, err := discordState.Cabinet.Guild(gID)
		if err != nil {
			slog.Error("failed to get guild from state", "guild_id", gID, "err", err)
			continue
		}

		gt.createGuildNode(folderNode, *guild)
	}
}

func (gt *guildsTree) unreadStyle(indication ningen.UnreadIndication) tcell.Style {
	var style tcell.Style
	switch indication {
	case ningen.ChannelRead:
		style = style.Dim(true)
	case ningen.ChannelMentioned:
		style = style.Underline(true)
		fallthrough
	case ningen.ChannelUnread:
		style = style.Bold(true)
	}

	return style
}

func (gt *guildsTree) getGuildNodeStyle(guildID discord.GuildID) tcell.Style {
	indication := discordState.GuildIsUnread(guildID, ningen.GuildUnreadOpts{UnreadOpts: ningen.UnreadOpts{IncludeMutedCategories: true}})
	return gt.unreadStyle(indication)
}

func (gt *guildsTree) getChannelNodeStyle(channelID discord.ChannelID) tcell.Style {
	indication := discordState.ChannelIsUnread(channelID, ningen.UnreadOpts{IncludeMutedCategories: true})
	return gt.unreadStyle(indication)
}

func (gt *guildsTree) createGuildNode(n *tview.TreeNode, guild discord.Guild) {
	guildNode := tview.NewTreeNode(guild.Name).
		SetReference(guild.ID).
		SetTextStyle(gt.getGuildNodeStyle(guild.ID))
	n.AddChild(guildNode)
}

func (gt *guildsTree) createChannelNode(node *tview.TreeNode, channel discord.Channel) {
	// Log every channel being processed
	slog.Debug("processing channel",
		"channel_id", channel.ID,
		"channel_name", channel.Name,
		"channel_type", channel.Type,
		"guild_id", channel.GuildID)

	// NOTE: Removed permission check because Discord already filters channels at the API level.
	// If a channel appears in Cabinet.Channels(), the user should have access to see it.
	// The permission check was causing legitimate channels to be hidden.

	slog.Debug("creating channel node",
		"channel_id", channel.ID,
		"channel_name", channel.Name,
		"channel_type", channel.Type)

	channelNode := tview.NewTreeNode(ui.ChannelToString(channel)).
		SetReference(channel.ID).
		SetTextStyle(gt.getChannelNodeStyle(channel.ID))
	node.AddChild(channelNode)
}

func (gt *guildsTree) createChannelNodes(node *tview.TreeNode, channels []discord.Channel) {
	for _, channel := range channels {
		if channel.Type != discord.GuildCategory && !channel.ParentID.IsValid() {
			gt.createChannelNode(node, channel)
		}
	}

PARENT_CHANNELS:
	for _, channel := range channels {
		if channel.Type == discord.GuildCategory {
			for _, nested := range channels {
				if nested.ParentID == channel.ID {
					gt.createChannelNode(node, channel)
					continue PARENT_CHANNELS
				}
			}
		}
	}

	for _, channel := range channels {
		if channel.ParentID.IsValid() {
			var parent *tview.TreeNode
			node.Walk(func(node, _ *tview.TreeNode) bool {
				if node.GetReference() == channel.ParentID {
					parent = node
					return false
				}

				return true
			})

			if parent != nil {
				gt.createChannelNode(parent, channel)
			}
		}
	}
}

func (gt *guildsTree) onSelected(node *tview.TreeNode) {
	app.messageInput.reset()

	if len(node.GetChildren()) != 0 {
		node.SetExpanded(!node.IsExpanded())
		return
	}

	switch ref := node.GetReference().(type) {
	case discord.GuildID:
		go discordState.MemberState.Subscribe(ref)

		channels, err := discordState.Cabinet.Channels(ref)
		if err != nil {
			slog.Error("failed to get channels", "err", err, "guild_id", ref)
			return
		}

		slog.Debug("fetched channels for guild", "guild_id", ref, "total_channels", len(channels))

		sort.Slice(channels, func(i, j int) bool {
			return channels[i].Position < channels[j].Position
		})

		gt.createChannelNodes(node, channels)
	case discord.ChannelID:
		channel, err := discordState.Cabinet.Channel(ref)
		if err != nil {
			slog.Error("failed to get channel", "channel_id", ref)
			return
		}

		// Check if the channel type does NOT support messages (explicitly exclude problematic types)
		switch channel.Type {
		case discord.GuildCategory, discord.GuildForum:
			// These channel types definitely do not support messages
			slog.Debug("channel type does not support messages", "channel_type", channel.Type, "channel_name", channel.Name, "channel_id", channel.ID)
			return
		case discord.GuildVoice, discord.GuildStageVoice:
			// Voice channels don't support messages directly, but we should look for associated text channels
			slog.Debug("voice channel selected, looking for associated text channel", "channel_type", channel.Type, "channel_name", channel.Name, "channel_id", channel.ID)

			// Find associated text channel in the same category or with similar name
			textChannelID := gt.findAssociatedTextChannel(*channel)
			if textChannelID.IsValid() {
				slog.Info("found associated text channel", "voice_channel", channel.Name, "text_channel_id", textChannelID, "voice_channel_id", channel.ID)
				// Get the text channel and process it
				textChannel, err := discordState.Cabinet.Channel(textChannelID)
				if err != nil {
					slog.Error("failed to get associated text channel", "text_channel_id", textChannelID, "err", err)
					// Fall back to voice channel behavior
					app.messagesList.reset()
					app.messagesList.setTitle(*channel)
					app.messagesList.drawMessages([]discord.Message{})
					app.messageInput.SetDisabled(true)
					app.messageInput.SetPlaceholder("Voice channel - no associated text channel found")
					gt.selectedChannelID = channel.ID
					gt.selectedGuildID = channel.GuildID
					return
				}

				// Get messages for the text channel and process it
				var messages []discord.Message
				var fetchErr error
				if textChannel.Type != discord.DirectMessage && textChannel.Type != discord.GroupDM {
					hasViewPerm := discordState.HasPermissions(textChannel.ID, discord.PermissionViewChannel)
					slog.Debug("permission check result for associated text channel", "channel_id", textChannel.ID, "channel_name", textChannel.Name, "has_view_permission", hasViewPerm)
					if !hasViewPerm {
						messages = []discord.Message{}
						fetchErr = nil
					} else {
						messages, fetchErr = discordState.Messages(textChannel.ID, uint(gt.cfg.MessagesLimit))
					}
				} else {
					messages, fetchErr = discordState.Messages(textChannel.ID, uint(gt.cfg.MessagesLimit))
				}

				if fetchErr != nil {
					slog.Error("failed to get messages for associated text channel", "err", fetchErr, "channel_id", textChannel.ID)
					cabinetMessages, cabinetErr := discordState.Cabinet.Messages(textChannel.ID)
					if cabinetErr != nil {
						slog.Error("fallback cabinet message fetch also failed for associated text channel", "err", cabinetErr, "channel_id", textChannel.ID)
						messages = []discord.Message{}
					} else {
						messages = cabinetMessages
					}
				}

				gt.processTextChannel(*textChannel, messages)
				return
			} else {
				slog.Debug("no associated text channel found for voice channel", "channel_name", channel.Name, "channel_id", channel.ID)
				// If no associated text channel found, show a message or handle gracefully
				app.messagesList.reset()
				app.messagesList.setTitle(*channel)
				app.messagesList.drawMessages([]discord.Message{})
				app.messageInput.SetDisabled(true)
				app.messageInput.SetPlaceholder("Voice channel - no associated text channel found")
				gt.selectedChannelID = channel.ID
				gt.selectedGuildID = channel.GuildID
				return
			}
		default:
			// All other channel types (including text channels) proceed with normal processing
			slog.Debug("proceeding with message fetch for channel type", "channel_type", channel.Type, "channel_name", channel.Name, "channel_id", channel.ID)

			go discordState.ReadState.MarkRead(channel.ID, channel.LastMessageID)

			messages, err := discordState.Messages(channel.ID, uint(gt.cfg.MessagesLimit))
			if err != nil {
				slog.Error("failed to get messages", "err", err, "channel_id", channel.ID, "limit", gt.cfg.MessagesLimit)
				return
			}

			if guildID := channel.GuildID; guildID.IsValid() {
				app.messagesList.requestGuildMembers(guildID, messages)
			}

			app.messagesList.reset()
			app.messagesList.setTitle(*channel)
			app.messagesList.drawMessages(messages)
			app.messagesList.ScrollToEnd()

			hasNoPerm := channel.Type != discord.DirectMessage && channel.Type != discord.GroupDM && !discordState.HasPermissions(channel.ID, discord.PermissionSendMessages)
			app.messageInput.SetDisabled(hasNoPerm)
			if hasNoPerm {
				app.messageInput.SetPlaceholder("You do not have permission to send messages in this channel.")
			} else {
				app.messageInput.SetPlaceholder("Message...")
				app.SetFocus(app.messageInput)
			}

			gt.selectedChannelID = channel.ID
			gt.selectedGuildID = channel.GuildID
		}
	case nil: // Direct messages
		channels, err := discordState.PrivateChannels()
		if err != nil {
			slog.Error("failed to get private channels", "err", err)
			return
		}

		sort.Slice(channels, func(a, b int) bool {
			msgID := func(ch discord.Channel) discord.MessageID {
				if ch.LastMessageID.IsValid() {
					return ch.LastMessageID
				}
				return discord.MessageID(ch.ID)
			}
			return msgID(channels[a]) > msgID(channels[b])
		})

		for _, c := range channels {
			gt.createChannelNode(node, c)
		}
	}
}

func (gt *guildsTree) collapseParentNode(node *tview.TreeNode) {
	gt.
		GetRoot().
		Walk(func(n, parent *tview.TreeNode) bool {
			if n == node && parent.GetLevel() != 0 {
				parent.Collapse()
				gt.SetCurrentNode(parent)
				return false
			}

			return true
		})
}

func (gt *guildsTree) onInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Name() {
	case gt.cfg.Keys.GuildsTree.CollapseParentNode:
		gt.collapseParentNode(gt.GetCurrentNode())
		return nil
	case gt.cfg.Keys.GuildsTree.MoveToParentNode:
		return tcell.NewEventKey(tcell.KeyRune, 'K', tcell.ModNone)

	case gt.cfg.Keys.GuildsTree.SelectPrevious:
		gt.Move(-1)
	case gt.cfg.Keys.GuildsTree.SelectNext:
		gt.Move(1)
	case gt.cfg.Keys.GuildsTree.SelectFirst:
		gt.Move(gt.GetRowCount() * -1)
	case gt.cfg.Keys.GuildsTree.SelectLast:
		gt.Move(gt.GetRowCount())

	case gt.cfg.Keys.GuildsTree.SelectCurrent:
		return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)

	case gt.cfg.Keys.GuildsTree.YankID:
		gt.yankID()
	}

	return nil
}

func (gt *guildsTree) yankID() {
	node := gt.GetCurrentNode()
	if node == nil {
		return
	}

	// Reference of a tree node in the guilds tree is its ID.
	// discord.Snowflake (discord.GuildID and discord.ChannelID) have the String method.
	if id, ok := node.GetReference().(fmt.Stringer); ok {
		go clipboard.Write(clipboard.FmtText, []byte(id.String()))
	}
}

// processTextChannel handles the display of text channel messages
func (gt *guildsTree) processTextChannel(channel discord.Channel, messages []discord.Message) {
	app.messagesList.reset()
	app.messagesList.setTitle(channel)
	app.messagesList.drawMessages(messages)
	app.messagesList.ScrollToEnd()
	slog.Debug("finished drawing messages", "channel_id", channel.ID, "channel_name", channel.Name, "timestamp", time.Now().UnixNano())

	hasNoPerm := channel.Type != discord.DirectMessage && channel.Type != discord.GroupDM && !discordState.HasPermissions(channel.ID, discord.PermissionSendMessages)
	app.messageInput.SetDisabled(hasNoPerm)
	if hasNoPerm {
		app.messageInput.SetPlaceholder("You do not have permission to send messages in this channel.")
	} else {
		app.messageInput.SetPlaceholder("Message...")
		app.SetFocus(app.messageInput)
	}

	gt.selectedChannelID = channel.ID
	gt.selectedGuildID = channel.GuildID
}

// findAssociatedTextChannel finds an associated text channel for a voice channel
func (gt *guildsTree) findAssociatedTextChannel(voiceChannel discord.Channel) discord.ChannelID {
	// Get all channels in the same guild
	channels, err := discordState.Cabinet.Channels(voiceChannel.GuildID)
	if err != nil {
		slog.Error("failed to get guild channels for voice channel association", "guild_id", voiceChannel.GuildID, "err", err)
		return 0
	}

	// Look for text channels that might be associated with this voice channel
	for _, channel := range channels {
		// Skip non-text channels
		if channel.Type != discord.GuildText && channel.Type != discord.GuildAnnouncement {
			continue
		}

		// Priority 1: Check if the channel is in the same category (parent) - highest priority
		if voiceChannel.ParentID.IsValid() && channel.ParentID == voiceChannel.ParentID {
			slog.Info("found associated text channel in same category", "voice_channel", voiceChannel.Name, "text_channel", channel.Name, "category_id", voiceChannel.ParentID)
			return channel.ID
		}

		// Priority 2: Check for exact name match (case-insensitive)
		if strings.EqualFold(voiceChannel.Name, channel.Name) {
			slog.Info("found associated text channel with exact name match", "voice_channel", voiceChannel.Name, "text_channel", channel.Name)
			return channel.ID
		}

		// Priority 3: Check for common naming patterns with normalization
		voiceName := strings.ToLower(strings.TrimSpace(voiceChannel.Name))
		channelName := strings.ToLower(strings.TrimSpace(channel.Name))

		// Normalize names by removing common prefixes/suffixes and special characters
		normalizedVoice := normalizeChannelName(voiceName)
		normalizedChannel := normalizeChannelName(channelName)

		// Check if normalized names match
		if normalizedVoice == normalizedChannel {
			slog.Info("found associated text channel with normalized name match", "voice_channel", voiceChannel.Name, "text_channel", channel.Name, "normalized_voice", normalizedVoice, "normalized_text", normalizedChannel)
			return channel.ID
		}

		// Check if one name contains the other after normalization
		if strings.Contains(normalizedChannel, normalizedVoice) || strings.Contains(normalizedVoice, normalizedChannel) {
			slog.Info("found associated text channel with partial name match", "voice_channel", voiceChannel.Name, "text_channel", channel.Name)
			return channel.ID
		}

		// Check for common voice/text channel patterns
		if isAssociatedNamePattern(voiceName, channelName) {
			slog.Info("found associated text channel with pattern match", "voice_channel", voiceChannel.Name, "text_channel", channel.Name)
			return channel.ID
		}
	}

	// If no associated channel found, return 0
	return 0
}

// normalizeChannelName normalizes channel names for comparison
func normalizeChannelName(name string) string {
	// Remove common prefixes and suffixes, special characters, and normalize spaces
	name = strings.ToLower(name)
	name = strings.TrimSpace(name)

	// Remove common prefixes/suffixes
	name = strings.ReplaceAll(name, "voice", "")
	name = strings.ReplaceAll(name, "text", "")
	name = strings.ReplaceAll(name, "chat", "")
	name = strings.ReplaceAll(name, "general", "gen")
	name = strings.ReplaceAll(name, "main", "")
	name = strings.ReplaceAll(name, "main-", "")
	name = strings.ReplaceAll(name, "-main", "")
	name = strings.ReplaceAll(name, "vc", "")
	name = strings.ReplaceAll(name, "channel", "")
	name = strings.ReplaceAll(name, "room", "")

	// Remove special characters and normalize spaces/dashes
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, "_", "")
	name = strings.ReplaceAll(name, " ", "")

	return strings.TrimSpace(name)
}

// isAssociatedNamePattern checks for common voice/text channel naming patterns
func isAssociatedNamePattern(voiceName, textName string) bool {
	// Common patterns like "general-voice" <-> "general-text" or "voice-general" <-> "general"
	if strings.Contains(voiceName, "voice") && (strings.Contains(textName, "text") || strings.Contains(textName, "chat")) {
		// Remove voice/text parts and compare the core name
		voiceCore := strings.ReplaceAll(voiceName, "voice", "")
		textCore := strings.ReplaceAll(strings.ReplaceAll(textName, "text", ""), "chat", "")
		voiceCore = strings.ReplaceAll(voiceCore, "-", "")
		voiceCore = strings.ReplaceAll(voiceCore, "_", "")
		textCore = strings.ReplaceAll(textCore, "-", "")
		textCore = strings.ReplaceAll(textCore, "_", "")
		if voiceCore == textCore && voiceCore != "" {
			return true
		}
	}

	// Pattern like "gaming" voice channel and "gaming-chat" text channel
	if strings.Contains(textName, voiceName+"-chat") || strings.Contains(textName, voiceName+"-text") {
		return true
	}

	// Pattern like "music-voice" and "music" or "music-text"
	if strings.Contains(voiceName, "-voice") && (strings.HasPrefix(textName, strings.TrimSuffix(voiceName, "-voice")) ||
		strings.HasPrefix(textName, strings.TrimSuffix(voiceName, "-voice")+"-text")) {
		return true
	}

	return false
}
