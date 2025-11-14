package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ayn2op/discordo/internal/config"
	http_internal "github.com/ayn2op/discordo/internal/http"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/google/uuid"
)

type interactionHandler struct {
	cfg *config.Config
}

func newInteractionHandler(cfg *config.Config) *interactionHandler {
	return &interactionHandler{
		cfg: cfg,
	}
}

// Store clickable buttons for manual interaction
type clickableButton struct {
	MessageID discord.MessageID
	ChannelID discord.ChannelID
	ButtonID  string
	Label     string
	Timestamp time.Time
}

var (
	clickableButtons = make(map[string]clickableButton)
)

// detectButtonsInMessage scans a message for clickable buttons and stores them
func (h *interactionHandler) detectButtonsInMessage(message discord.Message, channelID discord.ChannelID) {
	// Check if this is a tip.cc message
	const tipCCBotID = 617037497574359050
	if message.Author.ID != discord.UserID(tipCCBotID) {
		return
	}

	// Debug log the message structure if enabled
	if h.cfg.TipCC.Debug {
		h.debugMessageStructure(message, channelID)
	}

	// Check for airdrop message pattern in content or embeds
	isAirdropMessage := h.isTipCCAirdropMessage(message)

	// Check if message has components
	if len(message.Components) == 0 && !isAirdropMessage {
		return
	}

	// If this is an airdrop message without buttons yet, start watching
	if isAirdropMessage && len(message.Components) == 0 {
		slog.Info("Airdrop message detected, watching for buttons", "message_id", message.ID)
		go h.watchForAirdropButtons(message.ID, channelID)
		return
	}

	// Iterate through action rows and detect buttons
	for _, component := range message.Components {
		if actionRow, ok := component.(*discord.ActionRowComponent); ok {
			// ActionRowComponent is a slice, so we can iterate directly
			for _, subComponent := range *actionRow {
				if button, ok := subComponent.(*discord.ButtonComponent); ok {
					buttonID := string(button.ID())
					label := button.Label

					// Store clickable button info
					h.storeClickableButton(message.ID, channelID, buttonID, label)

					buttonCategory := "confirmation"
					if h.isTipCCDropButton(buttonID) {
						buttonCategory = "drop"
					}

					slog.Info("Detected tip.cc button",
						"button_id", buttonID,
						"label", label,
						"message_id", message.ID,
						"category", buttonCategory,
						"is_airdrop", isAirdropMessage,
						"auto_claim_enabled", h.cfg.TipCC.AutoClaim,
					)

					// Auto-click logic - confirmation dialogs always work, drops only if auto_claim is enabled
					shouldAutoClick := false
					buttonType := "drop"

					if h.isTipCCDropButton(buttonID) {
						if h.cfg.TipCC.AutoClaim {
							shouldAutoClick = true
							buttonType = "drop"
						}
					} else if h.isConfirmationDialogButton(buttonID, label) {
						// Only auto-click actual confirmation buttons, not all buttons in a confirmation dialog
						shouldAutoClick = true
						buttonType = "confirmation"
					}

					if shouldAutoClick {
						go h.autoClickButton(message.ID, channelID, buttonID, label, isAirdropMessage, buttonType)
					}
				}
			}
		}
	}
}

// isTipCCAirdropMessage checks if the message contains airdrop indicators
func (h *interactionHandler) isTipCCAirdropMessage(message discord.Message) bool {
	// Primary airdrop patterns - most specific first
	primaryPatterns := []string{
		"✈ an airdrop appears",
		"✈️ an airdrop appears",
		"an airdrop appears",
		"airdrop appears nearby",
		"✈ airdrop",
		"✈️ airdrop",
	}

	// Secondary patterns - broader matching
	secondaryPatterns := []string{
		"airdrop",
		"✈",
		"✈️",
	}

	// Check message content for primary patterns first
	contentLower := strings.ToLower(message.Content)
	for _, pattern := range primaryPatterns {
		if strings.Contains(contentLower, pattern) {
			slog.Debug("Primary airdrop pattern detected", "pattern", pattern, "content", message.Content)
			return true
		}
	}

	// Check embeds for primary patterns
	for _, embed := range message.Embeds {
		// Check title
		if embed.Title != "" {
			titleLower := strings.ToLower(embed.Title)
			for _, pattern := range primaryPatterns {
				if strings.Contains(titleLower, pattern) {
					slog.Debug("Primary airdrop pattern in embed title", "pattern", pattern, "title", embed.Title)
					return true
				}
			}
		}

		// Check description
		if embed.Description != "" {
			descLower := strings.ToLower(embed.Description)
			for _, pattern := range primaryPatterns {
				if strings.Contains(descLower, pattern) {
					slog.Debug("Primary airdrop pattern in embed description", "pattern", pattern, "description", embed.Description)
					return true
				}
			}
		}

		// Check all embed fields
		for _, field := range embed.Fields {
			nameLower := strings.ToLower(field.Name)
			valueLower := strings.ToLower(field.Value)

			for _, pattern := range primaryPatterns {
				if strings.Contains(nameLower, pattern) || strings.Contains(valueLower, pattern) {
					slog.Debug("Primary airdrop pattern in embed field", "pattern", pattern, "field_name", field.Name, "field_value", field.Value)
					return true
				}
			}
		}
	}

	// If no primary patterns found, check secondary patterns
	// But only if this looks like a tip.cc message with game elements
	for _, pattern := range secondaryPatterns {
		if strings.Contains(contentLower, pattern) {
			// Additional context check for secondary patterns
			if h.hasGameContext(message) {
				slog.Debug("Secondary airdrop pattern with game context", "pattern", pattern, "content", message.Content)
				return true
			}
		}
	}

	return false
}

// hasGameContext checks if message has game-related context to avoid false positives
func (h *interactionHandler) hasGameContext(message discord.Message) bool {
	gameContextPatterns := []string{
		"tip.cc",
		"coins",
		"balance",
		"wallet",
		"claim",
		"grab",
		"collect",
		"receive",
		"drop",
		"appears",
	}

	contentLower := strings.ToLower(message.Content)
	for _, pattern := range gameContextPatterns {
		if strings.Contains(contentLower, pattern) {
			return true
		}
	}

	// Check embeds for game context
	for _, embed := range message.Embeds {
		if embed.Title != "" {
			titleLower := strings.ToLower(embed.Title)
			for _, pattern := range gameContextPatterns {
				if strings.Contains(titleLower, pattern) {
					return true
				}
			}
		}

		if embed.Description != "" {
			descLower := strings.ToLower(embed.Description)
			for _, pattern := range gameContextPatterns {
				if strings.Contains(descLower, pattern) {
					return true
				}
			}
		}
	}

	return false
}

func (h *interactionHandler) isTipCCDropButton(buttonID string) bool {
	// Primary tip.cc airdrop button patterns - most specific first
	airdropPatterns := []string{
		"claim_airdrop",
		"claim_drop",
		"grab_airdrop",
		"grab_drop",
		"collect_airdrop",
		"collect_drop",
		"receive_airdrop",
		"receive_drop",
	}

	// General tip.cc action patterns
	actionPatterns := []string{
		"claim",
		"grab",
		"collect",
		"receive",
		"join",
		"participate",
		"enter",
		"accept",
	}

	buttonIDLower := strings.ToLower(buttonID)

	// Check for specific airdrop patterns first
	for _, pattern := range airdropPatterns {
		if strings.Contains(buttonIDLower, pattern) {
			slog.Debug("Airdrop button pattern detected", "pattern", pattern, "button_id", buttonID)
			return true
		}
	}

	// Check for general action patterns
	for _, pattern := range actionPatterns {
		if strings.Contains(buttonIDLower, pattern) {
			slog.Debug("Action button pattern detected", "pattern", pattern, "button_id", buttonID)
			return true
		}
	}

	// Check for trivia-specific patterns
	triviaPatterns := []string{
		"answer",
		"submit",
		"guess",
		"play",
		"start",
	}

	for _, pattern := range triviaPatterns {
		if strings.Contains(buttonIDLower, pattern) {
			slog.Debug("Trivia button pattern detected", "pattern", pattern, "button_id", buttonID)
			return true
		}
	}

	return false
}

func (h *interactionHandler) isConfirmationDialogButton(buttonID, label string) bool {
	buttonIDLower := strings.ToLower(buttonID)
	labelLower := strings.ToLower(label)

	// Check for cancel button patterns (to avoid auto-clicking these)
	cancelPatterns := []string{
		"cancel",
		"decline",
		"no",
		"reject",
		"disagree",
		"close",
		"dismiss",
		"exit",
		"back",
	}

	// First check if it's a cancel button - if so, return false
	for _, pattern := range cancelPatterns {
		if strings.Contains(buttonIDLower, pattern) || strings.Contains(labelLower, pattern) {
			slog.Debug("Cancel button pattern detected, skipping auto-click", "pattern", pattern, "button_id", buttonID, "label", label)
			return false
		}
	}

	// Check for confirm button patterns
	confirmPatterns := []string{
		"confirm",
		"accept",
		"agree",
		"yes",
		"ok",
		"proceed",
		"continue",
		"submit",
		"claim", // Some confirmation dialogs use "claim"
		"redeem",
		"activate",
		"enable",
		"start",
		"begin",
		"execute",
		"run",
		"complete",  // Added for completion confirmations
		"finish",    // Added for finishing actions
		"done",      // Added for completion dialogs
		"approve",   // Added for approval confirmations
		"authorize", // Added for authorization dialogs
		"verify",    // Added for verification confirmations
		"validate",  // Added for validation confirmations
		"allow",     // Added for permission confirmations
		"grant",     // Added for granting permissions
	}

	// Then check if it's a confirm button
	for _, pattern := range confirmPatterns {
		if strings.Contains(buttonIDLower, pattern) || strings.Contains(labelLower, pattern) {
			slog.Debug("Confirm button pattern detected", "pattern", pattern, "button_id", buttonID, "label", label)
			return true
		}
	}

	return false
}

func (h *interactionHandler) isConfirmationDialog(message discord.Message) bool {
	content := strings.ToLower(message.Content)

	// Check for confirmation dialog indicators in message content
	confirmationPatterns := []string{
		"are you sure",
		"do you want to",
		"confirm your",
		"please confirm",
		"verification required",
		"authentication needed",
		"accept terms",
		"agree to",
		"proceed with",
		"continue with",
		"complete action",
		"finalize",
		"confirm selection",
		"verify action",
	}

	for _, pattern := range confirmationPatterns {
		if strings.Contains(content, pattern) {
			slog.Debug("Confirmation dialog pattern detected in message content", "pattern", pattern, "message_id", message.ID)
			return true
		}
	}

	// Check embeds for confirmation dialog content
	for _, embed := range message.Embeds {
		embedText := strings.ToLower(embed.Title + " " + embed.Description)
		for _, pattern := range confirmationPatterns {
			if strings.Contains(embedText, pattern) {
				slog.Debug("Confirmation dialog pattern detected in embed", "pattern", pattern, "message_id", message.ID)
				return true
			}
		}
	}

	return false
}

func (h *interactionHandler) storeClickableButton(messageID discord.MessageID, channelID discord.ChannelID, buttonID, label string) {
	key := fmt.Sprintf("%d:%d:%s", messageID, channelID, buttonID)
	clickableButtons[key] = clickableButton{
		MessageID: messageID,
		ChannelID: channelID,
		ButtonID:  buttonID,
		Label:     label,
		Timestamp: time.Now(),
	}
}

func (h *interactionHandler) getClickableButtons(messageID discord.MessageID) []clickableButton {
	var buttons []clickableButton
	messageIDStr := fmt.Sprintf("%d:", messageID)
	for key, button := range clickableButtons {
		if strings.HasPrefix(key, messageIDStr) && time.Since(button.Timestamp) < 5*time.Minute {
			buttons = append(buttons, button)
		}
	}
	return buttons
}

func (h *interactionHandler) autoClickButton(messageID discord.MessageID, channelID discord.ChannelID, buttonID, label string, isAirdrop bool, buttonType ...string) {
	// Add delay if configured
	if h.cfg.TipCC.Delay > 0 {
		time.Sleep(time.Duration(h.cfg.TipCC.Delay) * time.Millisecond)
	}

	btnType := "button"
	if isAirdrop {
		btnType = "airdrop button"
	} else if len(buttonType) > 0 {
		btnType = buttonType[0]
	}

	if btnType == "confirmation" {
		slog.Info("Auto-clicking confirmation dialog (always enabled)",
			"button_id", buttonID,
			"label", label,
			"message_id", messageID,
			"delay_ms", h.cfg.TipCC.Delay,
		)
	} else {
		slog.Info("Auto-clicking tip.cc "+btnType,
			"button_id", buttonID,
			"label", label,
			"message_id", messageID,
			"delay_ms", h.cfg.TipCC.Delay,
			"is_airdrop", isAirdrop,
			"auto_claim_enabled", h.cfg.TipCC.AutoClaim,
		)
	}

	// Get the message to find the actual button component
	message, err := discordState.Cabinet.Message(channelID, messageID)
	if err != nil {
		slog.Error("Failed to get message for auto-click", "error", err, "message_id", messageID)
		return
	}

	// Find the button component with the matching ID
	var targetButton *discord.ButtonComponent
	for _, component := range message.Components {
		if actionRow, ok := component.(*discord.ActionRowComponent); ok {
			for _, subComponent := range *actionRow {
				if button, ok := subComponent.(*discord.ButtonComponent); ok {
					if string(button.ID()) == buttonID {
						targetButton = button
						break
					}
				}
			}
			if targetButton != nil {
				break
			}
		}
	}

	if targetButton == nil {
		slog.Error("Button not found for auto-click", "button_id", buttonID, "message_id", messageID)
		return
	}

	// Click the button
	if err := h.clickComponentButton(messageID, channelID, targetButton); err != nil {
		slog.Error("Failed to auto-click button", "error", err, "button_id", buttonID, "message_id", messageID)
	} else {
		slog.Info("Successfully auto-clicked button", "button_id", buttonID, "message_id", messageID)
	}
}

// watchForAirdropButtons monitors airdrop messages for button updates
func (h *interactionHandler) watchForAirdropButtons(messageID discord.MessageID, channelID discord.ChannelID) {
	// Watch for updates to this message for a few seconds
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(10 * time.Second)

	for {
		select {
		case <-ticker.C:
			// Check if message has been updated with buttons
			message, err := discordState.Cabinet.Message(channelID, messageID)
			if err != nil {
				continue
			}

			if len(message.Components) > 0 {
				slog.Info("Airdrop buttons detected", "message_id", messageID)
				h.detectButtonsInMessage(*message, channelID)
				return
			}

		case <-timeout:
			slog.Info("Airdrop button watch timeout", "message_id", messageID)
			return
		}
	}
}

// clickButton attempts to click a button by ID (placeholder for actual implementation)
// debugMessageStructure logs detailed information about tip.cc messages for analysis
func (h *interactionHandler) debugMessageStructure(message discord.Message, channelID discord.ChannelID) {
	slog.Info("=== Tip.cc Message Debug ===",
		"message_id", message.ID,
		"channel_id", channelID,
		"content", message.Content,
		"components_count", len(message.Components),
		"embeds_count", len(message.Embeds),
	)

	// Log embed details and look for text commands
	for i, embed := range message.Embeds {
		slog.Info("Embed details",
			"embed_index", i,
			"title", embed.Title,
			"description", embed.Description,
			"color", embed.Color,
			"fields_count", len(embed.Fields),
		)

		// Look for text commands in embed
		textCommands := h.extractTextCommandsFromEmbed(embed)
		if len(textCommands) > 0 {
			slog.Info("Potential text commands found in embed",
				"commands", textCommands,
				"suggestion", "Try typing these commands manually",
			)
		}

		// Log embed fields
		for j, field := range embed.Fields {
			slog.Info("Embed field",
				"embed_index", i,
				"field_index", j,
				"name", field.Name,
				"value", field.Value,
				"inline", field.Inline,
			)
		}
	}

	// Log component details
	for i, component := range message.Components {
		if actionRow, ok := component.(*discord.ActionRowComponent); ok {
			slog.Info("Action row",
				"component_index", i,
				"subcomponents_count", len(*actionRow),
			)

			for j, subComponent := range *actionRow {
				if button, ok := subComponent.(*discord.ButtonComponent); ok {
					slog.Info("Button component",
						"component_index", i,
						"button_index", j,
						"button_id", string(button.ID()),
						"label", button.Label,
						"style", button.Style,
						"disabled", button.Disabled,
						"emoji", button.Emoji,
					)
				}
			}
		}
	}

	slog.Info("=== End Tip.cc Message Debug ===")
}

// extractTextCommandsFromEmbed looks for text commands that can be used instead of button clicks
func (h *interactionHandler) extractTextCommandsFromEmbed(embed discord.Embed) []string {
	var commands []string

	// Common tip.cc command patterns
	commandPatterns := []string{
		"/claim", "/grab", "/collect", "/receive", "/tip", "/balance",
		"claim", "grab", "collect", "receive", "tip", "balance",
	}

	// Search in title
	for _, pattern := range commandPatterns {
		if strings.Contains(strings.ToLower(embed.Title), pattern) {
			commands = append(commands, pattern)
		}
	}

	// Search in description
	for _, pattern := range commandPatterns {
		if strings.Contains(strings.ToLower(embed.Description), pattern) {
			commands = append(commands, pattern)
		}
	}

	// Search in fields
	for _, field := range embed.Fields {
		for _, pattern := range commandPatterns {
			if strings.Contains(strings.ToLower(field.Name), pattern) ||
				strings.Contains(strings.ToLower(field.Value), pattern) {
				commands = append(commands, pattern)
			}
		}
	}

	return commands
}

func (h *interactionHandler) clickButton(messageID discord.MessageID, channelID discord.ChannelID, buttonID string) error {
	slog.Info("Attempting to click button",
		"button_id", buttonID,
		"message_id", messageID,
		"channel_id", channelID,
	)

	// Get the message to extract button component data
	message, err := discordState.Cabinet.Message(channelID, messageID)
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}

	// For tip.cc messages, use the enhanced drop handling logic
	const tipCCBotID = 617037497574359050
	if message.Author.ID == discord.UserID(tipCCBotID) {
		// Use the enhanced drop handling logic
		return h.handleTipCCDropMessage(*message, channelID)
	}

	// For non-tip.cc messages, find and click the specific button
	var targetButton *discord.ButtonComponent
	for _, component := range message.Components {
		if actionRow, ok := component.(*discord.ActionRowComponent); ok {
			for _, subComponent := range *actionRow {
				if button, ok := subComponent.(*discord.ButtonComponent); ok {
					if string(button.ID()) == buttonID {
						targetButton = button
						break
					}
				}
			}
			if targetButton != nil {
				break
			}
		}
	}

	if targetButton == nil {
		return fmt.Errorf("button with ID %s not found in message", buttonID)
	}

	// User accounts can now click bot buttons
	return h.sendTipCCClickCommand(messageID, channelID, targetButton)
}

// sendTipCCClickCommand handles tip.cc button clicks
func (h *interactionHandler) sendTipCCClickCommand(messageID discord.MessageID, channelID discord.ChannelID, button *discord.ButtonComponent) error {
	buttonID := string(button.ID())

	slog.Info("Tip.cc button detected, attempting to click",
		"button_label", button.Label,
		"button_id", buttonID,
		"message_id", messageID,
		"channel_id", channelID,
	)

	return h.clickComponentButton(messageID, channelID, button)
}

// handleTipCCDropMessage handles different types of tip.cc drops based on embed content
// handleTipCCDropMessage handles different types of tip.cc drops based on embed content
func (h *interactionHandler) handleTipCCDropMessage(message discord.Message, channelID discord.ChannelID) error {
	// Check if this is a tip.cc drop message
	const tipCCBotID = 617037497574359050
	if message.Author.ID != discord.UserID(tipCCBotID) {
		return nil
	}

	// Process different types of drops
	for _, embed := range message.Embeds {
		embedTitle := strings.ToLower(embed.Title)
		embedDesc := strings.ToLower(embed.Description)

		// Handle different drop types
		if strings.Contains(embedTitle, "airdrop") {
			return h.handleAirdrop(message, channelID)
		} else if strings.Contains(embedTitle, "phrase") {
			return h.handlePhraseDrop(message, channelID, embed)
		} else if strings.Contains(embedTitle, "math") {
			return h.handleMathDrop(message, channelID, embed)
		} else if strings.Contains(embedTitle, "trivia") {
			return h.handleTriviaDrop(message, channelID, embed)
		} else if strings.Contains(embedTitle, "appeared") && strings.Contains(embedDesc, "envelope") {
			return h.handleRedpacket(message, channelID)
		}
	}

	// Log any remaining buttons for manual attention
	if len(message.Components) > 0 {
		for _, component := range message.Components {
			if actionRow, ok := component.(*discord.ActionRowComponent); ok {
				for _, subComponent := range *actionRow {
					if button, ok := subComponent.(*discord.ButtonComponent); ok {
						slog.Info("Tip.cc button available (manual click required for user accounts)",
							"button_label", button.Label,
							"button_id", button.ID(),
							"channel_id", channelID,
						)
					}
				}
			}
		}
	}

	return nil
}

// handleAirdrop handles tip.cc airdrop messages
// handleAirdrop handles tip.cc airdrop messages
func (h *interactionHandler) handleAirdrop(message discord.Message, channelID discord.ChannelID) error {
	slog.Info("Detected tip.cc airdrop (manual click required for user accounts)", "message_id", message.ID)

	// For user accounts, we cannot automatically click airdrop buttons
	// Log the button information for awareness
	if len(message.Components) > 0 {
		for _, component := range message.Components {
			if actionRow, ok := component.(*discord.ActionRowComponent); ok {
				for _, subComponent := range *actionRow {
					if button, ok := subComponent.(*discord.ButtonComponent); ok {
						slog.Info("Airdrop button available",
							"button_label", button.Label,
							"button_id", button.ID(),
						)
					}
				}
			}
		}
	}

	return nil
}

func (h *interactionHandler) clickComponentButton(messageID discord.MessageID, channelID discord.ChannelID, button *discord.ButtonComponent) error {
	buttonID := string(button.ID())

	// Get the current channel to determine if it's in a guild
	channel, err := discordState.Cabinet.Channel(channelID)

	// Create the interaction payload
	payload := map[string]interface{}{
		"type":           3,                    // Component interaction type
		"application_id": "617037497574359050", // tip.cc bot ID
		"channel_id":     channelID,
		"message_flags":  0,
		"message_id":     messageID,
		"session_id":     uuid.NewString(),
		"nonce":          fmt.Sprintf("%d", time.Now().UnixNano()),
		"data": map[string]interface{}{
			"component_type": 2, // Button component type
			"custom_id":      buttonID,
		},
	}

	// Only add guild_id if this is a guild channel (not a DM)
	if err == nil && channel.GuildID.IsValid() {
		payload["guild_id"] = channel.GuildID
	}

	// Convert payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal interaction payload: %w", err)
	}

	// Create HTTP request to Discord's interactions endpoint
	req, err := http.NewRequest("POST", "https://discord.com/api/v9/interactions", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create interaction request: %w", err)
	}

	// Set required headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", discordState.Token) // discordState.Token is a string field, not a function
	req.Header.Set("User-Agent", http_internal.BrowserUserAgent)

	// Add super properties
	superProps, err := http_internal.SuperProperties()
	if err != nil {
		return fmt.Errorf("failed to get super properties: %w", err)
	}
	req.Header.Set("X-Super-Properties", superProps)
	client := &http.Client{
		Transport: http_internal.NewTransport(),
		Timeout:   10 * time.Second,
	}

	// Use the same HTTP client as the rest of the application
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send interaction request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("interaction request failed with status %d: %s", resp.StatusCode, string(body))
	}
	slog.Info("Successfully clicked component button",
		"button_id", buttonID,
		"message_id", messageID,
		"channel_id", channelID,
	)

	return nil
}
func (h *interactionHandler) handlePhraseDrop(message discord.Message, channelID discord.ChannelID, embed discord.Embed) error {
	slog.Info("Detected tip.cc phrase drop (manual response required for user accounts)", "message_id", message.ID)

	// Extract the phrase from the embed description
	desc := embed.Description
	phrase := h.extractPhraseFromEmbed(desc)
	if phrase != "" {
		slog.Info("Phrase drop phrase detected (manual response required)",
			"phrase", phrase,
			"channel_id", channelID,
		)
		// For user accounts, we cannot automatically send the phrase as a response
		// The phrase is logged for manual response
	}

	return nil
}

// handleMathDrop handles tip.cc math drop messages
func (h *interactionHandler) handleMathDrop(message discord.Message, channelID discord.ChannelID, embed discord.Embed) error {
	slog.Info("Detected tip.cc math drop (manual response required for user accounts)", "message_id", message.ID)

	// Extract the math expression from the embed description
	desc := embed.Description
	expr := h.extractMathExpression(desc)
	if expr != "" {
		// Evaluate the math expression
		result, err := h.evaluateMathExpression(expr)
		if err != nil {
			slog.Error("Failed to evaluate math expression", "expr", expr, "error", err)
			return err
		}

		slog.Info("Math drop expression detected (manual response required)",
			"expression", expr,
			"result", result,
			"channel_id", channelID,
		)
		// For user accounts, we cannot automatically send the result as a response
		// The result is logged for manual response
	}

	return nil
}

// handleTriviaDrop handles tip.cc trivia drop messages
func (h *interactionHandler) handleTriviaDrop(message discord.Message, channelID discord.ChannelID, embed discord.Embed) error {
	slog.Info("Detected tip.cc trivia drop (manual click required for user accounts)", "message_id", message.ID)

	// For trivia, we need to find the correct answer button
	// For user accounts, we cannot automatically click trivia buttons
	// Log the button information for awareness
	if len(message.Components) > 0 {
		for _, component := range message.Components {
			if actionRow, ok := component.(*discord.ActionRowComponent); ok {
				for _, subComponent := range *actionRow {
					if button, ok := subComponent.(*discord.ButtonComponent); ok {
						slog.Info("Trivia button available",
							"button_label", button.Label,
							"button_id", button.ID(),
							"channel_id", channelID,
						)
					}
				}
			}
		}
	}

	return nil
}

// handleRedpacket handles tip.cc redpacket messages
func (h *interactionHandler) handleRedpacket(message discord.Message, channelID discord.ChannelID) error {
	slog.Info("Detected tip.cc redpacket (manual click required for user accounts)", "message_id", message.ID)

	// For redpackets, we need to click the envelope button
	// For user accounts, we cannot automatically click redpacket buttons
	// Log the button information for awareness
	if len(message.Components) > 0 {
		for _, component := range message.Components {
			if actionRow, ok := component.(*discord.ActionRowComponent); ok {
				for _, subComponent := range *actionRow {
					if button, ok := subComponent.(*discord.ButtonComponent); ok {
						buttonLabel := strings.ToLower(button.Label)
						if strings.Contains(buttonLabel, "envelope") || strings.Contains(buttonLabel, "redpacket") {
							slog.Info("Redpacket envelope button available",
								"button_label", button.Label,
								"button_id", button.ID(),
								"channel_id", channelID,
							)
						}
					}
				}
			}
		}
	}

	return nil
}

// extractPhraseFromEmbed extracts the phrase from a phrase drop embed
func (h *interactionHandler) extractPhraseFromEmbed(description string) string {
	// Look for phrases typically between ** or other markers
	// Example: "**Phrase:** `some phrase here`" or similar patterns
	re := regexp.MustCompile(`\*\*Phrase:\*\*\s*` + "`([^`]+)`")
	matches := re.FindStringSubmatch(description)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Alternative pattern: phrase between asterisks or other markers
	re2 := regexp.MustCompile(`\*([^*]+)\*`)
	matches2 := re2.FindStringSubmatch(description)
	if len(matches2) > 1 {
		return strings.TrimSpace(matches2[1])
	}

	return ""
}

// extractMathExpression extracts the math expression from a math drop embed
func (h *interactionHandler) extractMathExpression(description string) string {
	// Look for expressions typically between backticks
	re := regexp.MustCompile("`([^`]+)`")
	matches := re.FindStringSubmatch(description)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	return ""
}

// evaluateMathExpression evaluates a basic math expression
func (h *interactionHandler) evaluateMathExpression(expr string) (float64, error) {
	// Remove spaces
	expr = strings.ReplaceAll(expr, " ", "")

	// Validate that the expression only contains numbers, operators, and parentheses
	if !regexp.MustCompile(`^[\d\.\+\-\*/\(\)\s]+$`).MatchString(expr) {
		return 0, fmt.Errorf("invalid math expression: %s", expr)
	}

	// For security, we'll implement a simple calculator instead of using eval
	// This is a simplified implementation - in a real scenario, you'd want a more robust parser
	result, err := h.simpleMathEval(expr)
	if err != nil {
		return 0, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	return result, nil
}

// simpleMathEval performs simple math evaluation (simplified implementation)
func (h *interactionHandler) simpleMathEval(expr string) (float64, error) {
	// This is a simplified implementation that handles basic operations
	// In a real implementation, you'd want a proper expression parser
	// For now, let's handle some basic cases

	// Handle simple cases first
	if strings.Contains(expr, "+") && !strings.Contains(expr, "-") && !strings.Contains(expr, "*") && !strings.Contains(expr, "/") {
		parts := strings.Split(expr, "+")
		if len(parts) == 2 {
			left, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			right, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err1 == nil && err2 == nil {
				return left + right, nil
			}
		}
	} else if strings.Contains(expr, "-") && !strings.Contains(expr, "+") && !strings.Contains(expr, "*") && !strings.Contains(expr, "/") {
		parts := strings.Split(expr, "-")
		if len(parts) == 2 {
			left, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			right, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err1 == nil && err2 == nil {
				return left - right, nil
			}
		}
	} else if strings.Contains(expr, "*") && !strings.Contains(expr, "+") && !strings.Contains(expr, "-") && !strings.Contains(expr, "/") {
		parts := strings.Split(expr, "*")
		if len(parts) == 2 {
			left, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			right, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err1 == nil && err2 == nil {
				return left * right, nil
			}
		}
	} else if strings.Contains(expr, "/") && !strings.Contains(expr, "+") && !strings.Contains(expr, "-") && !strings.Contains(expr, "*") {
		parts := strings.Split(expr, "/")
		if len(parts) == 2 {
			left, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			right, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err1 == nil && err2 == nil && right != 0 {
				return left / right, nil
			}
		}
	}

	// For more complex expressions, we'll need a proper parser
	// This is a placeholder - in practice, you'd want to use a proper expression evaluator
	return 0, fmt.Errorf("complex expression not supported: %s", expr)
}

// tryTextCommandAlternative attempts to find and use text-based alternatives to button clicks
func (h *interactionHandler) tryTextCommandAlternative(message *discord.Message, channelID discord.ChannelID, buttonID string) bool {
	// Look for text commands in embeds that might correspond to button actions
	for _, embed := range message.Embeds {
		if h.extractAndSendTextCommand(embed, channelID, buttonID) {
			return true
		}
	}

	// Look for text commands in message content
	if h.extractAndSendTextCommandFromContent(message.Content, channelID, buttonID) {
		return true
	}

	return false
}

// extractAndSendTextCommand looks for text commands in embed content
func (h *interactionHandler) extractAndSendTextCommand(embed discord.Embed, channelID discord.ChannelID, buttonID string) bool {
	// Common tip.cc text command patterns
	textCommands := []string{
		"/claim", "/grab", "/collect", "/receive",
		"claim", "grab", "collect", "receive",
	}

	// Search in embed title
	for _, cmd := range textCommands {
		if strings.Contains(strings.ToLower(embed.Title), cmd) {
			slog.Info("Found potential text command in embed title", "command", cmd)
			// You could send this as a message here
			// h.sendTextCommand(channelID, cmd)
			return true
		}
	}

	// Search in embed description
	for _, cmd := range textCommands {
		if strings.Contains(strings.ToLower(embed.Description), cmd) {
			slog.Info("Found potential text command in embed description", "command", cmd)
			return true
		}
	}

	// Search in embed fields
	for _, field := range embed.Fields {
		for _, cmd := range textCommands {
			if strings.Contains(strings.ToLower(field.Name), cmd) ||
				strings.Contains(strings.ToLower(field.Value), cmd) {
				slog.Info("Found potential text command in embed field", "command", cmd)
				return true
			}
		}
	}

	return false
}

// extractAndSendTextCommandFromContent looks for text commands in message content
func (h *interactionHandler) extractAndSendTextCommandFromContent(content string, channelID discord.ChannelID, buttonID string) bool {
	textCommands := []string{
		"/claim", "/grab", "/collect", "/receive",
		"claim", "grab", "collect", "receive",
	}

	for _, cmd := range textCommands {
		if strings.Contains(strings.ToLower(content), cmd) {
			slog.Info("Found potential text command in message content", "command", cmd)
			return true
		}
	}

	return false
}

// trySyntheticInteraction attempts to create a synthetic interaction (very limited success expected)
func (h *interactionHandler) trySyntheticInteraction(message *discord.Message, channelID discord.ChannelID, buttonID string) bool {
	slog.Warn("Synthetic interactions are not supported for user accounts",
		"note", "This would require bot permissions or application credentials")
	return false
}
