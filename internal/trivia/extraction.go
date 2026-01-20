package trivia

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
)

// ExtractTriviaQuestion extracts question and category from Discord message
func ExtractTriviaQuestion(message discord.Message) (string, string, error) {
	// Check if message has embeds
	if len(message.Embeds) == 0 {
		return "", "", fmt.Errorf("no embeds found in message")
	}

	embed := message.Embeds[0]

	// Check if this is a triviadrop
	if !strings.Contains(embed.Title, "Trivia time - ") {
		return "", "", fmt.Errorf("not a triviadrop message")
	}

	// Extract category from embed title
	category := strings.TrimSpace(strings.TrimPrefix(embed.Title, "Trivia time - "))
	if category == "" {
		return "", "", fmt.Errorf("could not extract category from title: %s", embed.Title)
	}

	// Extract question from embed description
	// Format: **User** asked: *question*
	// We need to extract the question after "asked:" which is wrapped in single asterisks
	questionPattern := regexp.MustCompile(`asked:\s*\*([^*]+)\*`)
	matches := questionPattern.FindStringSubmatch(embed.Description)

	// Fallback: try extracting question without asterisks (plain text after "asked:")
	if len(matches) < 2 {
		questionPattern = regexp.MustCompile(`asked:\s*(.+?)(?:\n|$)`)
		matches = questionPattern.FindStringSubmatch(embed.Description)
	}

	// Fallback: if no "asked:" pattern, try to get the main text (could be just the question)
	if len(matches) < 2 {
		// Try to extract any text that looks like a question (ends with ?)
		questionPattern = regexp.MustCompile(`([^*\n]+\?)`)
		matches = questionPattern.FindStringSubmatch(embed.Description)
	}

	if len(matches) < 2 {
		return "", "", fmt.Errorf("could not extract question from description: %s", embed.Description)
	}

	question := strings.TrimSpace(matches[1])

	slog.Debug("Extracted trivia question",
		"category", category,
		"question", question,
		"message_id", message.ID,
	)

	return category, question, nil
}

// FindMatchingButton finds the button that matches the given answer
func FindMatchingButton(message discord.Message, targetAnswer string) (discord.ButtonComponent, error) {
	if len(message.Components) == 0 {
		return discord.ButtonComponent{}, fmt.Errorf("no components found in message")
	}

	// Search through action rows for buttons
	for _, component := range message.Components {
		if actionRow, ok := component.(*discord.ActionRowComponent); ok {
			for _, subComponent := range *actionRow {
				if button, ok := subComponent.(*discord.ButtonComponent); ok {
					buttonLabel := strings.TrimSpace(button.Label)

					// Exact match
					if buttonLabel == targetAnswer {
						return *button, nil
					}

					// Case-insensitive match
					if strings.EqualFold(buttonLabel, targetAnswer) {
						return *button, nil
					}

					// Partial match (for cases where answer might be truncated)
					if strings.Contains(strings.ToLower(buttonLabel), strings.ToLower(targetAnswer)) {
						return *button, nil
					}
				}
			}
		}
	}

	return discord.ButtonComponent{}, fmt.Errorf("no matching button found for answer: %s", targetAnswer)
}
