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
	"sync"
	"time"

	"github.com/ayn2op/discordo/internal/config"
	http_internal "github.com/ayn2op/discordo/internal/http"
	"github.com/ayn2op/discordo/internal/notifications"
	"github.com/ayn2op/discordo/internal/trivia"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/google/uuid"
)

// TriviaStrategy represents the answer selection strategy for triviadrops
type TriviaStrategy string

const (
	TriviaStrategyRandom TriviaStrategy = "random"
	TriviaStrategyFirst  TriviaStrategy = "first"
	TriviaStrategyA      TriviaStrategy = "a"
	TriviaStrategyB      TriviaStrategy = "b"
	TriviaStrategyC      TriviaStrategy = "c"
	TriviaStrategyD      TriviaStrategy = "d"
	TriviaStrategySmart  TriviaStrategy = "smart"
)

// AnswerChoice represents a selected answer for triviadrops
type AnswerChoice struct {
	ButtonID string
	Label    string
	Index    int
}

type interactionHandler struct {
	cfg            *config.Config
	triviaDatabase *trivia.TriviaDatabase
	dbReady        chan struct{} // Closed when database is ready
	dbReadyOnce    sync.Once     // Ensures we only close the channel once
}

func newInteractionHandler(cfg *config.Config) *interactionHandler {
	h := &interactionHandler{
		cfg:            cfg,
		triviaDatabase: trivia.NewTriviaDatabase(),
		dbReady:        make(chan struct{}), // Initialize the ready channel
	}

	// Initialize the trivia database in a goroutine to avoid blocking startup
	// Only initialize if triviadrop is enabled
	if cfg.TipCC.TriviaDrop {
		go func() {
			slog.Info("Initializing trivia database...")
			if err := h.triviaDatabase.DownloadOTDBDatabase(cfg); err != nil {
				slog.Error("Failed to download trivia database", "error", err)
			} else {
				slog.Info("Trivia database initialized", "stats", h.triviaDatabase.GetStats())
			}
			// Mark the database as ready
			h.dbReadyOnce.Do(func() {
				close(h.dbReady)
			})
		}()
	} else {
		// If trivia drop is not enabled, mark as immediately ready
		h.dbReadyOnce.Do(func() {
			close(h.dbReady)
		})
	}

	return h
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

	// Check for airdrop and triviadrop message patterns
	isAirdropMessage := h.IsTipCCAirdropMessage(message)
	isTriviaDropMessage := h.IsTipCCTriviaDropMessage(message)

	// Check if message has components
	if len(message.Components) == 0 && !isAirdropMessage && !isTriviaDropMessage {
		return
	}

	// If this is an airdrop message without buttons yet, start watching
	if isAirdropMessage && len(message.Components) == 0 {
		slog.Info("Airdrop message detected, watching for buttons", "message_id", message.ID)
		go h.watchForAirdropButtons(message.ID, channelID)
		return
	}

	// If this is a triviadrop message without buttons yet, start watching
	if isTriviaDropMessage && len(message.Components) == 0 {
		slog.Info("🧩 Triviadrop message detected, watching for buttons", "message_id", message.ID)
		go h.watchForTriviaDropButtons(message.ID, channelID)
		return
	}

	// Info level logging for visibility
	if !isTriviaDropMessage && len(message.Components) > 0 {
		slog.Info("Message has components but doesn't match triviadrop pattern", "message_id", message.ID)
	}

	// Check if triviadrop is enabled in config
	if !h.cfg.TipCC.TriviaDrop {
		slog.Warn("Trivia drop is disabled in config - enable it with triviadrop_enabled = true")
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

					slog.Debug("Processing button component",
						"button_label", label,
						"is_triviadrop_button", h.isTriviaDropAnswerButton(buttonID, button.Label))

					// Store clickable button info
					h.storeClickableButton(message.ID, channelID, buttonID, label)

					buttonCategory := "confirmation"
					if h.isTipCCDropButton(buttonID) {
						buttonCategory = "drop"
					} else if h.isTriviaDropAnswerButton(buttonID, label) {
						buttonCategory = "triviadrop"
					}

					slog.Info("Detected tip.cc button",
						"button_id", buttonID,
						"label", label,
						"message_id", message.ID,
						"category", buttonCategory,
						"is_airdrop", isAirdropMessage,
						"is_triviadrop", isTriviaDropMessage,
						"auto_claim_enabled", h.cfg.TipCC.AutoClaim,
						"triviadrop_enabled", h.cfg.TipCC.TriviaDrop,
					)

					// Auto-click logic - confirmation dialogs always work, drops only if auto_claim is enabled
					shouldAutoClick := false
					buttonType := "drop"

					if h.isTipCCDropButton(buttonID) {
						if h.cfg.TipCC.AutoClaim {
							shouldAutoClick = true
							buttonType = "drop"
						}
					} else if h.isTriviaDropAnswerButton(buttonID, label) {
						if h.cfg.TipCC.TriviaDrop && isTriviaDropMessage {
							shouldAutoClick = true
							buttonType = "triviadrop"
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

// IsTipCCAirdropMessage checks if the message contains airdrop indicators
func (h *interactionHandler) IsTipCCAirdropMessage(message discord.Message) bool {
	// Primary airdrop patterns - most specific first
	primaryPatterns := []string{
		"$airdrop",
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

// IsTipCCTriviaDropMessage checks if the message contains triviadrop indicators
func (h *interactionHandler) IsTipCCTriviaDropMessage(message discord.Message) bool {
	// Primary triviadrop patterns - most specific first
	primaryPatterns := []string{
		"$triviadrop",
		"trivia drop",
		"triviadrop",
		"trivia drop appears",
		"a trivia drop",
		"trivia drop created",
		"trivia drop started",
	}

	// Secondary patterns - broader matching with game context
	secondaryPatterns := []string{
		"trivia drop",
		"trivia",
	}

	// Debug: Log initial check
	slog.Debug("Checking for triviadrop message",
		"author_id", message.Author.ID,
		"content_length", len(message.Content),
		"embeds_count", len(message.Embeds))

	// Check message content for primary patterns first
	contentLower := strings.ToLower(message.Content)
	for _, pattern := range primaryPatterns {
		if strings.Contains(contentLower, pattern) {
			slog.Debug("Primary triviadrop pattern detected", "pattern", pattern, "content", message.Content)
			return true
		}
	}

	// Check embeds for primary patterns
	for embedIdx, embed := range message.Embeds {
		// Debug: Log embed details
		slog.Debug("Checking embed for triviadrop",
			"embed_index", embedIdx,
			"title", embed.Title,
			"description_length", len(embed.Description))

		if embed.Title != "" {
			titleLower := strings.ToLower(embed.Title)
			for _, pattern := range primaryPatterns {
				if strings.Contains(titleLower, pattern) {
					slog.Debug("Primary triviadrop pattern detected in embed title", "pattern", pattern, "title", embed.Title)
					return true
				}
			}
		}

		if embed.Description != "" {
			descLower := strings.ToLower(embed.Description)
			for _, pattern := range primaryPatterns {
				if strings.Contains(descLower, pattern) {
					slog.Debug("Primary triviadrop pattern detected in embed description", "pattern", pattern, "description", embed.Description)
					return true
				}
			}
		}

		// Check embed fields
		for fieldIdx, field := range embed.Fields {
			fieldNameLower := strings.ToLower(field.Name)
			fieldValueLower := strings.ToLower(field.Value)

			// Debug: Log field being checked
			slog.Debug("Checking embed field",
				"embed_index", embedIdx,
				"field_index", fieldIdx,
				"field_name", field.Name,
				"field_value_length", len(field.Value))

			for _, pattern := range primaryPatterns {
				if strings.Contains(fieldNameLower, pattern) || strings.Contains(fieldValueLower, pattern) {
					slog.Debug("Primary triviadrop pattern detected in embed field", "pattern", pattern, "field_name", field.Name, "field_value", field.Value)
					return true
				}
			}
		}
	}

	// Check secondary patterns only if they have game context
	for _, pattern := range secondaryPatterns {
		if strings.Contains(contentLower, pattern) {
			if h.hasGameContext(message) {
				slog.Debug("Secondary triviadrop pattern with game context", "pattern", pattern, "content", message.Content)
				return true
			}
		}
	}

	// Check embeds for secondary patterns with game context
	for embedIdx, embed := range message.Embeds {
		if embed.Title != "" {
			titleLower := strings.ToLower(embed.Title)
			for _, pattern := range secondaryPatterns {
				if strings.Contains(titleLower, pattern) && h.hasGameContext(message) {
					slog.Debug("Secondary triviadrop pattern in embed title with game context", "pattern", pattern, "embed_index", embedIdx, "title", embed.Title)
					return true
				}
			}
		}

		if embed.Description != "" {
			descLower := strings.ToLower(embed.Description)
			for _, pattern := range secondaryPatterns {
				if strings.Contains(descLower, pattern) && h.hasGameContext(message) {
					slog.Debug("Secondary triviadrop pattern in embed description with game context", "pattern", pattern, "embed_index", embedIdx, "description_length", len(embed.Description))
					return true
				}
			}
		}

		// Check embed fields
		for fieldIdx, field := range embed.Fields {
			fieldNameLower := strings.ToLower(field.Name)
			fieldValueLower := strings.ToLower(field.Value)

			for _, pattern := range secondaryPatterns {
				if (strings.Contains(fieldNameLower, pattern) || strings.Contains(fieldValueLower, pattern)) && h.hasGameContext(message) {
					slog.Debug("Secondary triviadrop pattern in embed field with game context", "pattern", pattern, "embed_index", embedIdx, "field_index", fieldIdx)
					return true
				}
			}
		}
	}

	// Debug: If we reached here, no patterns were found
	slog.Debug("No triviadrop patterns found in message", "message_id", message.ID)
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

// isAirdropButton checks if the button ID is specifically for airdrops
func (h *interactionHandler) isAirdropButton(buttonID string) bool {
	// Primary tip.cc airdrop button patterns - most specific first
	airdropPatterns := []string{
		"claim_airdrop",
		"grab_airdrop",
		"collect_airdrop",
		"receive_airdrop",
	}

	buttonIDLower := strings.ToLower(buttonID)

	// Check for specific airdrop patterns
	for _, pattern := range airdropPatterns {
		if strings.Contains(buttonIDLower, pattern) {
			slog.Debug("Airdrop button pattern detected", "pattern", pattern, "button_id", buttonID)
			return true
		}
	}

	return false
}

// isTriviaDropAnswerButton checks if the button is specifically for triviadrop answers
func (h *interactionHandler) isTriviaDropAnswerButton(buttonID, label string) bool {
	buttonIDLower := strings.ToLower(buttonID)
	labelLower := strings.ToLower(label)

	// Primary triviadrop answer button patterns
	answerPatterns := []string{
		"answer_a", "answer_b", "answer_c", "answer_d",
		"option_a", "option_b", "option_c", "option_d",
		"trivia_a", "trivia_b", "trivia_c", "trivia_d",
		"quiz_a", "quiz_b", "quiz_c", "quiz_d",
		"true", "false",
		"a)", "b)", "c)", "d)",
		"(a)", "(b)", "(c)", "(d)",
	}

	// Check button ID patterns
	for _, pattern := range answerPatterns {
		if strings.Contains(buttonIDLower, pattern) {
			slog.Debug("Triviadrop answer button pattern detected in ID", "pattern", pattern, "button_id", buttonID)
			return true
		}
	}

	// Check button label patterns
	for _, pattern := range answerPatterns {
		if strings.Contains(labelLower, pattern) {
			slog.Debug("Triviadrop answer button pattern detected in label", "pattern", pattern, "button_label", label)
			return true
		}
	}

	// Check for single letter labels (A, B, C, D)
	singleLetterPatterns := []string{"a", "b", "c", "d"}
	if len(labelLower) > 0 {
		for _, pattern := range singleLetterPatterns {
			if strings.TrimSpace(labelLower) == pattern {
				slog.Debug("Triviadrop single letter answer detected", "letter", pattern, "button_label", label)
				return true
			}
		}
	}

	// Check for true/false specifically
	trueFalsePatterns := []string{"true", "false"}
	for _, pattern := range trueFalsePatterns {
		if strings.TrimSpace(labelLower) == pattern {
			slog.Debug("Triviadrop true/false answer detected", "answer", pattern, "button_label", label)
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

// selectTriviaAnswer selects an answer based on the configured strategy
func (h *interactionHandler) selectTriviaAnswer(message discord.Message) (AnswerChoice, error) {
	// Get the configured strategy
	strategy := TriviaStrategy(strings.ToLower(h.cfg.TipCC.TriviaStrategy))

	slog.Debug("Selecting trivia answer",
		"strategy", strategy,
		"message_id", message.ID)

	// Handle strategy-specific logic
	switch strategy {
	case TriviaStrategySmart:
		// Use database lookup for 95%+ accuracy
		return h.selectSmartAnswer(message)
	case TriviaStrategyRandom:
		return h.selectRandomAnswerStrategy(message)
	case TriviaStrategyFirst:
		return h.selectFirstAnswer(message)
	case TriviaStrategyA, TriviaStrategyB, TriviaStrategyC, TriviaStrategyD:
		return h.selectSpecificAnswer(message, strategy)
	default:
		// Default to smart strategy if unknown strategy
		return h.selectSmartAnswer(message)
	}
}

// selectSmartAnswer implements the database-driven answer selection
func (h *interactionHandler) selectSmartAnswer(message discord.Message) (AnswerChoice, error) {
	// Wait for the database to be ready (with timeout)
	select {
	case <-h.dbReady:
		// Database is ready, continue
	case <-time.After(10 * time.Second):
		// Timeout waiting for database, proceed anyway but log warning
		slog.Warn("Timeout waiting for trivia database to be ready", "message_id", message.ID)
	}

	// First try database lookup for 95%+ accuracy
	category, question, err := trivia.ExtractTriviaQuestion(message)
	if err != nil {
		slog.Warn("Failed to extract trivia question for smart strategy", "error", err, "message_id", message.ID)
		// Fall back to heuristics if extraction fails
		return h.heuristicBasedAnswer(message)
	}

	// Try exact database match first
	if answer, found := h.triviaDatabase.GetAnswer(question); found {
		slog.Info("Found exact match in trivia database",
			"category", category,
			"question", question,
			"answer", answer,
		)
		return h.findMatchingButton(message, answer)
	}

	// Try fuzzy matching for variations
	if answer, found := h.triviaDatabase.FuzzyMatch(question); found {
		slog.Info("Found fuzzy match in trivia database",
			"category", category,
			"question", question,
			"answer", answer,
		)
		return h.findMatchingButton(message, answer)
	}

	// If no database match, fall back to heuristics
	slog.Info("No database match, falling back to heuristics",
		"category", category,
		"question", question,
	)

	return h.heuristicBasedAnswer(message)
}

// selectRandomAnswerStrategy implements random answer selection
func (h *interactionHandler) selectRandomAnswerStrategy(message discord.Message) (AnswerChoice, error) {
	var answerButtons []AnswerChoice
	if len(message.Components) > 0 {
		for componentIdx, component := range message.Components {
			if actionRow, ok := component.(*discord.ActionRowComponent); ok {
				for subComponentIdx, subComponent := range *actionRow {
					if button, ok := subComponent.(*discord.ButtonComponent); ok {
						buttonID := string(button.ID())
						if h.isTriviaDropAnswerButton(buttonID, button.Label) {
							answerButtons = append(answerButtons, AnswerChoice{
								ButtonID: buttonID,
								Label:    button.Label,
								Index:    componentIdx*100 + subComponentIdx, // Unique index
							})
						}
					}
				}
			}
		}
	}

	if len(answerButtons) == 0 {
		slog.Warn("No answer buttons found for random strategy", "message_id", message.ID)
		return AnswerChoice{}, fmt.Errorf("no answer buttons found")
	}

	// Simple random selection using time
	randomIndex := time.Now().Nanosecond() % len(answerButtons)
	if randomIndex < 0 {
		randomIndex = -randomIndex
	}

	selected := answerButtons[randomIndex]
	slog.Debug("Selected random answer",
		"answer", selected.Label,
		"button_id", selected.ButtonID,
		"total_options", len(answerButtons),
		"selected_index", randomIndex)

	return selected, nil
}

// selectFirstAnswer implements first answer selection
func (h *interactionHandler) selectFirstAnswer(message discord.Message) (AnswerChoice, error) {
	if len(message.Components) > 0 {
		for componentIdx, component := range message.Components {
			if actionRow, ok := component.(*discord.ActionRowComponent); ok {
				for subComponentIdx, subComponent := range *actionRow {
					if button, ok := subComponent.(*discord.ButtonComponent); ok {
						buttonID := string(button.ID())
						if h.isTriviaDropAnswerButton(buttonID, button.Label) {
							selected := AnswerChoice{
								ButtonID: buttonID,
								Label:    button.Label,
								Index:    componentIdx*100 + subComponentIdx, // Unique index
							}

							slog.Debug("Selected first answer",
								"answer", selected.Label,
								"button_id", selected.ButtonID)

							return selected, nil
						}
					}
				}
			}
		}
	}

	slog.Warn("No answer buttons found for first strategy", "message_id", message.ID)
	return AnswerChoice{}, fmt.Errorf("no answer buttons found")
}

// selectSpecificAnswer implements A, B, C, D specific answer selection
func (h *interactionHandler) selectSpecificAnswer(message discord.Message, strategy TriviaStrategy) (AnswerChoice, error) {
	// Convert strategy to target letter - extract the last character as string (a, b, c, d)
	var targetLetter string
	switch strategy {
	case TriviaStrategyA:
		targetLetter = "a"
	case TriviaStrategyB:
		targetLetter = "b"
	case TriviaStrategyC:
		targetLetter = "c"
	case TriviaStrategyD:
		targetLetter = "d"
	default:
		// For other strategies, just return first answer
		return h.selectFirstAnswer(message)
	}

	var answerButtons []AnswerChoice
	if len(message.Components) > 0 {
		for componentIdx, component := range message.Components {
			if actionRow, ok := component.(*discord.ActionRowComponent); ok {
				for subComponentIdx, subComponent := range *actionRow {
					if button, ok := subComponent.(*discord.ButtonComponent); ok {
						buttonID := string(button.ID())
						if h.isTriviaDropAnswerButton(buttonID, button.Label) {
							answerButtons = append(answerButtons, AnswerChoice{
								ButtonID: buttonID,
								Label:    button.Label,
								Index:    componentIdx*100 + subComponentIdx, // Unique index
							})
						}
					}
				}
			}
		}
	}

	if len(answerButtons) == 0 {
		slog.Warn("No answer buttons found for specific strategy", "strategy", strategy, "message_id", message.ID)
		return AnswerChoice{}, fmt.Errorf("no answer buttons found")
	}

	// Find the button matching the specified letter
	for _, answer := range answerButtons {
		buttonLabel := strings.ToLower(strings.TrimSpace(answer.Label))
		target := strings.ToLower(targetLetter)

		if buttonLabel == target ||
			buttonLabel == target+")" ||
			buttonLabel == "("+target+")" {
			slog.Debug("Selected specific answer",
				"strategy", strategy,
				"answer", answer.Label,
				"button_id", answer.ButtonID)
			return answer, nil
		}
	}

	// If exact match not found, return the first answer
	slog.Warn("Specific answer not found, returning first available",
		"strategy", strategy,
		"target_letter", targetLetter,
		"available_options", len(answerButtons))

	return answerButtons[0], nil
}

// selectRandomAnswer randomly selects an answer from available options
func (h *interactionHandler) selectRandomAnswer(answers []AnswerChoice) AnswerChoice {
	if len(answers) == 0 {
		return AnswerChoice{}
	}

	// Simple random selection using time
	randomIndex := time.Now().Nanosecond() % len(answers)
	if randomIndex < 0 {
		randomIndex = -randomIndex
	}

	selected := answers[randomIndex]
	slog.Debug("Selected random answer",
		"answer", selected.Label,
		"button_id", selected.ButtonID,
		"total_options", len(answers),
		"selected_index", randomIndex)

	return selected
}

// heuristicBasedAnswer provides fallback logic when database lookup fails
func (h *interactionHandler) heuristicBasedAnswer(message discord.Message) (AnswerChoice, error) {
	// Extract all answer buttons from the message
	var answerButtons []AnswerChoice
	if len(message.Components) > 0 {
		for componentIdx, component := range message.Components {
			if actionRow, ok := component.(*discord.ActionRowComponent); ok {
				for subComponentIdx, subComponent := range *actionRow {
					if button, ok := subComponent.(*discord.ButtonComponent); ok {
						buttonID := string(button.ID())
						if h.isTriviaDropAnswerButton(buttonID, button.Label) {
							answerButtons = append(answerButtons, AnswerChoice{
								ButtonID: buttonID,
								Label:    button.Label,
								Index:    componentIdx*100 + subComponentIdx, // Unique index
							})
						}
					}
				}
			}
		}
	}

	if len(answerButtons) == 0 {
		return AnswerChoice{}, fmt.Errorf("no answer buttons found")
	}

	// Extract trivia question for potential collection
	category, question, err := trivia.ExtractTriviaQuestion(message)
	if err != nil {
		slog.Debug("Could not extract trivia question for collection", "error", err)
	} else {
		// Log unknown question for future database expansion
		slog.Info("Unknown trivia question, potentially for database expansion",
			"category", category,
			"question", question,
		)
	}

	// Use existing random selection as fallback
	return h.selectRandomAnswer(answerButtons), nil
}

// findMatchingButton finds the button that matches the given answer
func (h *interactionHandler) findMatchingButton(message discord.Message, targetAnswer string) (AnswerChoice, error) {
	if len(message.Components) == 0 {
		return AnswerChoice{}, fmt.Errorf("no components found in message")
	}

	// Search through action rows for buttons
	componentIdx := 0
	for _, component := range message.Components {
		if actionRow, ok := component.(*discord.ActionRowComponent); ok {
			subComponentIdx := 0
			for _, subComponent := range *actionRow {
				if button, ok := subComponent.(*discord.ButtonComponent); ok {
					buttonLabel := strings.TrimSpace(button.Label)

					// Exact match
					if buttonLabel == targetAnswer {
						slog.Debug("Found exact match for answer",
							"target_answer", targetAnswer,
							"button_label", buttonLabel)
						return AnswerChoice{
							ButtonID: string(button.ID()),
							Label:    buttonLabel,
							Index:    componentIdx*100 + subComponentIdx, // Unique index
						}, nil
					}

					// Case-insensitive match
					if strings.EqualFold(buttonLabel, targetAnswer) {
						slog.Debug("Found case-insensitive match for answer",
							"target_answer", targetAnswer,
							"button_label", buttonLabel)
						return AnswerChoice{
							ButtonID: string(button.ID()),
							Label:    buttonLabel,
							Index:    componentIdx*100 + subComponentIdx, // Unique index
						}, nil
					}

					// Normalize both strings for comparison
					normalizedTarget := normalizeText(targetAnswer)
					normalizedLabel := normalizeText(buttonLabel)

					// Normalized match
					if normalizedTarget == normalizedLabel {
						slog.Debug("Found normalized match for answer",
							"target_answer", targetAnswer,
							"button_label", buttonLabel,
							"normalized_target", normalizedTarget,
							"normalized_label", normalizedLabel)
						return AnswerChoice{
							ButtonID: string(button.ID()),
							Label:    buttonLabel,
							Index:    componentIdx*100 + subComponentIdx, // Unique index
						}, nil
					}

					// Partial match (for cases where answer might be truncated)
					if strings.Contains(strings.ToLower(buttonLabel), strings.ToLower(targetAnswer)) {
						slog.Debug("Found partial match for answer",
							"target_answer", targetAnswer,
							"button_label", buttonLabel)
						return AnswerChoice{
							ButtonID: string(button.ID()),
							Label:    buttonLabel,
							Index:    componentIdx*100 + subComponentIdx, // Unique index
						}, nil
					}

					// Check if the button label contains the target answer with common prefixes/suffixes
					buttonLower := strings.ToLower(buttonLabel)
					targetLower := strings.ToLower(targetAnswer)
					if strings.Contains(buttonLower, " "+targetLower+" ") ||
						strings.HasPrefix(buttonLower, targetLower+" ") ||
						strings.HasSuffix(buttonLower, " "+targetLower) ||
						buttonLower == targetLower+"!" ||
						buttonLower == targetLower+"." {
						slog.Debug("Found contextual match for answer",
							"target_answer", targetAnswer,
							"button_label", buttonLabel)
						return AnswerChoice{
							ButtonID: string(button.ID()),
							Label:    buttonLabel,
							Index:    componentIdx*100 + subComponentIdx, // Unique index
						}, nil
					}
				}
				subComponentIdx++
			}
		}
		componentIdx++
	}

	slog.Warn("No matching button found for answer",
		"target_answer", targetAnswer,
		"message_id", message.ID)
	return AnswerChoice{}, fmt.Errorf("no matching button found for answer: %s", targetAnswer)
}

// normalizeText normalizes text for comparison by removing extra spaces, punctuation, etc.
func normalizeText(text string) string {
	// Remove leading/trailing whitespace
	text = strings.TrimSpace(text)

	// Convert to lowercase
	text = strings.ToLower(text)

	// Remove common punctuation at the end
	text = strings.TrimRight(text, ".!?:;")

	// Normalize multiple spaces to single space
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	return text
}

// findAnswerByLetter finds an answer by its letter (A, B, C, D)
func (h *interactionHandler) findAnswerByLetter(answers []AnswerChoice, letter string) (AnswerChoice, error) {
	letter = strings.ToLower(letter)

	for _, answer := range answers {
		answerLabel := strings.ToLower(strings.TrimSpace(answer.Label))
		if answerLabel == letter || answerLabel == letter+")" || answerLabel == "("+letter+")" {
			slog.Debug("Selected answer by letter",
				"letter", letter,
				"answer", answer.Label,
				"button_id", answer.ButtonID)
			return answer, nil
		}
	}

	// If exact match not found, try partial match
	for _, answer := range answers {
		answerLabel := strings.ToLower(answer.Label)
		if strings.Contains(answerLabel, letter) {
			slog.Debug("Selected answer by partial letter match",
				"letter", letter,
				"answer", answer.Label,
				"button_id", answer.ButtonID)
			return answer, nil
		}
	}

	// Fallback to first answer if requested letter not found
	if len(answers) > 0 {
		slog.Debug("Requested letter not found, falling back to first answer",
			"requested_letter", letter,
			"fallback_answer", answers[0].Label)
		return answers[0], nil
	}

	return AnswerChoice{}, fmt.Errorf("no answers available for letter %s", letter)
}

func (h *interactionHandler) autoClickButton(messageID discord.MessageID, channelID discord.ChannelID, buttonID, label string, isAirdrop bool, buttonType ...string) {
	btnType := "button"
	if isAirdrop {
		btnType = "airdrop button"
	} else if len(buttonType) > 0 {
		btnType = buttonType[0]
	}

	// Add delay based on button type
	if btnType == "triviadrop" {
		// Use triviadrop-specific delay (much faster)
		delay := h.cfg.TipCC.TriviaDelay
		if delay == 0 {
			delay = 200 // Default 200ms for triviadrops
		}
		if delay > 0 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	} else {
		// Use regular delay for other button types
		if h.cfg.TipCC.Delay > 0 {
			time.Sleep(time.Duration(h.cfg.TipCC.Delay) * time.Millisecond)
		}
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

// watchForTriviaDropButtons monitors triviadrop messages for button updates
func (h *interactionHandler) watchForTriviaDropButtons(messageID discord.MessageID, channelID discord.ChannelID) {
	// Watch for updates to this message for a few seconds (triviadrops appear quickly)
	ticker := time.NewTicker(200 * time.Millisecond) // Faster polling for triviadrops
	defer ticker.Stop()

	timeout := time.After(5 * time.Second) // Shorter timeout for triviadrops

	for {
		select {
		case <-ticker.C:
			// Check if message has been updated with buttons
			message, err := discordState.Cabinet.Message(channelID, messageID)
			if err != nil {
				continue
			}

			if len(message.Components) > 0 {
				slog.Info("🧩 Triviadrop buttons detected", "message_id", messageID)
				h.detectButtonsInMessage(*message, channelID)
				return
			}

		case <-timeout:
			slog.Info("🧩 Triviadrop button watch timeout", "message_id", messageID)
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
func (h *interactionHandler) handleTipCCDropMessage(message discord.Message, channelID discord.ChannelID) error {
	// Check if this is a tip.cc drop message
	const tipCCBotID = 617037497574359050
	if message.Author.ID != discord.UserID(tipCCBotID) {
		return nil
	}

	// Info level logging for visibility
	slog.Info("Processing tip.cc message",
		"message_id", message.ID,
		"channel_id", channelID,
		"embeds_count", len(message.Embeds),
		"components_count", len(message.Components),
		"auto_claim", h.cfg.TipCC.AutoClaim,
		"triviadrop_enabled", h.cfg.TipCC.TriviaDrop,
	)

	if !h.cfg.TipCC.TriviaDrop {
		slog.Warn("Trivia drop autoplay is disabled in configuration.")
	}

	// Process different types of drops
	for i, embed := range message.Embeds {
		embedTitle := strings.ToLower(embed.Title)
		embedDesc := strings.ToLower(embed.Description)

		slog.Info("Processing embed for drop types",
			"embed_index", i,
			"title", embed.Title,
			"description_present", len(embed.Description) > 0,
		)

		// Handle different drop types
		if strings.Contains(embedTitle, "airdrop") {
			slog.Info("Detected airdrop", "embed_index", i)
			return h.handleAirdrop(message, channelID)
		} else if strings.Contains(embedTitle, "phrase") {
			slog.Info("Detected phrase drop", "embed_index", i)
			return h.handlePhraseDrop(message, channelID, embed)
		} else if strings.Contains(embedTitle, "math") {
			slog.Info("Detected math drop", "embed_index", i)
			return h.handleMathDrop(message, channelID, embed)
		} else if strings.Contains(embedTitle, "trivia") || h.IsTipCCTriviaDropMessage(message) {
			slog.Info("🧩 Detected trivia/triviadrop", "embed_index", i, "title", embed.Title)
			return h.handleTriviaDrop(message, channelID, embed)
		} else if strings.Contains(embedTitle, "appeared") && strings.Contains(embedDesc, "envelope") {
			slog.Info("Detected redpacket", "embed_index", i)
			return h.handleRedpacket(message, channelID)
		}
	}

	// Log if no drop types matched
	slog.Info("No matching drop type found", "title", message.Embeds[0].Title, "description_length", len(message.Embeds[0].Description))

	// Log any remaining buttons for manual attention
	if len(message.Components) > 0 {
		for _, component := range message.Components {
			if actionRow, ok := component.(*discord.ActionRowComponent); ok {
				for _, subComponent := range *actionRow {
					if button, ok := subComponent.(*discord.ButtonComponent); ok {
						slog.Info("Tip.cc button available",
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
	slog.Info("Detected tip.cc airdrop", "message_id", message.ID)

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

	// Check if this was an airdrop button and show notification
	if h.isAirdropButton(buttonID) {
		channel, err := discordState.Cabinet.Channel(channelID)
		channelName := "unknown"
		if err == nil {
			if channel.Name != "" {
				channelName = channel.Name
			} else {
				channelName = channelID.String()
			}
		}
		msg := fmt.Sprintf("Airdrop claimed in #%s", channelName)
		app.ShowNotification(msg)
		if h.cfg.Notifications.Enabled {
			go func() {
				_ = notifications.Send("Airdrop Claimed", msg, true, h.cfg.Notifications.Duration)
			}()
		}
		slog.Info("Airdrop claimed notification shown", "channel", channelName, "button_id", buttonID)
	}

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
	slog.Info("🧩 Detected tip.cc trivia drop", "message_id", message.ID)

	// Check if triviadrop autoclaim is enabled
	if !h.cfg.TipCC.TriviaDrop {
		slog.Debug("Triviadrop autoclaim is disabled, logging buttons only")
		return h.logTriviaButtons(message, channelID)
	}

	// Select answer based on strategy
	selectedAnswer, err := h.selectTriviaAnswer(message)
	if err != nil {
		slog.Error("Failed to select trivia answer", "error", err, "message_id", message.ID)
		return err
	}

	slog.Info("🧩 Selected triviadrop answer",
		"answer", selectedAnswer.Label,
		"button_id", selectedAnswer.ButtonID,
		"strategy", h.cfg.TipCC.TriviaStrategy,
		"message_id", message.ID)

	// Apply triviadrop-specific delay (much faster than regular drops)
	delay := h.cfg.TipCC.TriviaDelay
	if delay == 0 {
		delay = 200 // Default 200ms for triviadrops
	}

	// Auto-click the selected answer button with triviadrop delay
	go func() {
		time.Sleep(time.Duration(delay) * time.Millisecond)
		h.autoClickButton(message.ID, channelID, selectedAnswer.ButtonID, selectedAnswer.Label, false, "triviadrop")
	}()

	return nil
}

// logTriviaButtons logs available trivia buttons when autoclaim is disabled
func (h *interactionHandler) logTriviaButtons(message discord.Message, channelID discord.ChannelID) error {
	if len(message.Components) > 0 {
		for _, component := range message.Components {
			if actionRow, ok := component.(*discord.ActionRowComponent); ok {
				for _, subComponent := range *actionRow {
					if button, ok := subComponent.(*discord.ButtonComponent); ok {
						slog.Info("Trivia button available (manual click required)",
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
	slog.Info("Detected tip.cc redpacket", "message_id", message.ID)

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
