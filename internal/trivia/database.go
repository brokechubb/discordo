package trivia

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ayn2op/discordo/internal/config"
)

// TriviaQuestion represents a single trivia question with its answer
type TriviaQuestion struct {
	Question string
	Answer   string
	Category string
}

// TriviaDatabase holds the complete trivia database with efficient lookup structures
type TriviaDatabase struct {
	Questions     map[string]string           // Question -> Answer (fast O(1) lookup)
	Categories    map[string][]TriviaQuestion // Category -> Questions
	QuestionCount int
	LastUpdated   time.Time
	mutex         sync.RWMutex
}

// NewTriviaDatabase creates a new trivia database instance
func NewTriviaDatabase() *TriviaDatabase {
	return &TriviaDatabase{
		Questions:  make(map[string]string),
		Categories: make(map[string][]TriviaQuestion),
		mutex:      sync.RWMutex{},
	}
}

// OTDBCategories represents all available OTDB categories
var OTDBCategories = []string{
	"General Knowledge", "Geography", "History", "Science & Nature",
	"Science: Mathematics", "Science: Computers", "Science: Gadgets",
	"Science & Nature", "Animals", "Celebrities", "Entertainment: Board Games",
	"Entertainment: Books", "Entertainment: Cartoon & Animations", "Entertainment: Comics",
	"Entertainment: Film", "Entertainment: Japanese Anime & Manga", "Entertainment: Music",
	"Entertainment: Musicals & Theatres", "Entertainment: Television", "Entertainment: Video Games",
	"Politics", "Mythology", "Sports", "Vehicles",
}

// DownloadOTDBDatabase downloads and parses the complete Open Trivia Database
// It first tries to load from local file, then supplements with OTDB download
func (db *TriviaDatabase) DownloadOTDBDatabase(cfg *config.Config) error {
	startTime := time.Now()

	// Try to load local trivia database first (JakeySelfBot export)
	localPaths := []string{
		"data/trivia.csv",
		"/home/chubb/src/discordo/data/trivia.csv",
	}

	localLoaded := false
	for _, path := range localPaths {
		if _, err := os.Stat(path); err == nil {
			slog.Info("Loading local trivia database...", "path", path)
			if err := db.LoadFromLocalCSV(path); err != nil {
				slog.Warn("Failed to load local trivia database", "path", path, "error", err)
			} else {
				localLoaded = true
				slog.Info("Local trivia database loaded successfully",
					"path", path,
					"questions", len(db.Questions),
				)
				break
			}
		}
	}

	// If local database has enough questions, skip OTDB download
	if localLoaded && len(db.Questions) >= 1000 {
		db.mutex.Lock()
		db.QuestionCount = len(db.Questions)
		db.LastUpdated = time.Now()
		db.mutex.Unlock()

		slog.Info("Using local trivia database (skipping OTDB download)",
			"total_questions", len(db.Questions),
			"categories", len(db.Categories),
			"load_time_ms", time.Since(startTime).Milliseconds(),
		)
		return nil
	}

	// Supplement with OTDB download
	slog.Info("Downloading OTDB database to supplement local...",
		"categories", len(OTDBCategories),
		"existing_questions", len(db.Questions),
	)

	for _, category := range OTDBCategories {
		if err := db.downloadCategory(category); err != nil {
			slog.Debug("Failed to download OTDB category",
				"category", category,
				"error", err,
			)
			continue
		}
	}

	db.mutex.Lock()
	db.QuestionCount = len(db.Questions)
	db.LastUpdated = time.Now()
	db.mutex.Unlock()

	slog.Info("Trivia database initialization completed",
		"total_questions", len(db.Questions),
		"categories", len(db.Categories),
		"load_time_ms", time.Since(startTime).Milliseconds(),
	)

	return nil
}

// LoadFromLocalCSV loads trivia questions from a pipe-delimited CSV file
// Format: category|question|answer
func (db *TriviaDatabase) LoadFromLocalCSV(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open local trivia file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '|' // Use pipe delimiter
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // Allow variable fields

	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read local trivia CSV: %w", err)
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	loadedCount := 0
	for _, record := range records {
		if len(record) < 3 {
			continue
		}

		category := strings.TrimSpace(record[0])
		question := strings.TrimSpace(record[1])
		answer := strings.TrimSpace(record[2])

		if question == "" || answer == "" {
			continue
		}

		// Add to fast lookup map (case-insensitive key for better matching)
		db.Questions[question] = answer
		db.Questions[strings.ToLower(question)] = answer

		// Add to category
		triviaQuestion := TriviaQuestion{
			Question: question,
			Answer:   answer,
			Category: category,
		}
		db.Categories[category] = append(db.Categories[category], triviaQuestion)
		loadedCount++
	}

	slog.Debug("Loaded local trivia questions",
		"file", filePath,
		"loaded", loadedCount,
		"total_in_db", len(db.Questions),
	)

	return nil
}

// downloadCategory downloads and parses a single category from OTDB
func (db *TriviaDatabase) downloadCategory(category string) error {
	// Construct URL for the category CSV file
	url := fmt.Sprintf("https://raw.githubusercontent.com/QuartzWarrior/OTDB-Source/main/%s.csv", url.QueryEscape(category))

	slog.Debug("Downloading category",
		"category", category,
		"url", url,
	)

	// Download the file
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download category %s: %w", category, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error downloading category %s: %d", category, resp.StatusCode)
	}

	// Parse CSV content
	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to parse CSV for category %s: %w", category, err)
	}

	// Process records
	var questions []TriviaQuestion
	for _, record := range records {
		if len(record) < 2 {
			continue // Skip invalid records
		}

		// URL decode: question and answer
		question := strings.TrimSpace(record[0])
		answer := strings.TrimSpace(record[1])

		// Skip empty records
		if question == "" || answer == "" {
			continue
		}

		questions = append(questions, TriviaQuestion{
			Question: question,
			Answer:   answer,
			Category: category,
		})

		// Add to fast lookup map
		db.mutex.Lock()
		db.Questions[question] = answer
		db.mutex.Unlock()
	}

	// Store category questions
	db.mutex.Lock()
	db.Categories[category] = questions
	db.mutex.Unlock()

	slog.Debug("Processed category",
		"category", category,
		"questions", len(questions),
	)

	return nil
}

// GetAnswer performs fast O(1) lookup for exact question match
func (db *TriviaDatabase) GetAnswer(question string) (string, bool) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	// Try exact match first
	if answer, exists := db.Questions[question]; exists {
		return answer, true
	}

	return "", false
}

// FuzzyMatch performs fuzzy matching for question variations
func (db *TriviaDatabase) FuzzyMatch(question string) (string, bool) {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	originalQuestion := question
	question = strings.ToLower(strings.TrimSpace(question))

	// Try exact match first (both original and lowercase)
	if answer, exists := db.Questions[originalQuestion]; exists {
		return answer, true
	}
	if answer, exists := db.Questions[question]; exists {
		return answer, true
	}

	// Try common variations
	variations := []string{
		question,
		strings.TrimPrefix(question, "what is "),
		strings.TrimPrefix(question, "who is "),
		strings.TrimPrefix(question, "which "),
		strings.TrimPrefix(question, "where is "),
		strings.TrimPrefix(question, "when did "),
		strings.TrimPrefix(question, "how many "),
		strings.TrimSuffix(question, "?"),
		strings.TrimSuffix(question, "."),
		strings.TrimSuffix(question, " ?"),
		strings.ReplaceAll(question, "\u2019", "'"),
		strings.ReplaceAll(question, "\u201c", "\""),
		strings.ReplaceAll(question, "\u201d", "\""),
	}

	for _, variation := range variations {
		if answer, exists := db.Questions[variation]; exists {
			return answer, true
		}
	}

	// Try substring matching for partial questions
	for dbQuestion, answer := range db.Questions {
		dbQuestionLower := strings.ToLower(dbQuestion)
		// Check if our question contains the DB question or vice versa
		if strings.Contains(question, dbQuestionLower) || strings.Contains(dbQuestionLower, question) {
			if len(question) > 20 && len(dbQuestionLower) > 20 { // Only for substantial matches
				slog.Debug("Fuzzy substring match found",
					"input_question", question,
					"matched_question", dbQuestion,
					"answer", answer,
				)
				return answer, true
			}
		}
	}

	return "", false
}

// GetStats returns database statistics
func (db *TriviaDatabase) GetStats() map[string]interface{} {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	return map[string]interface{}{
		"total_questions": db.QuestionCount,
		"categories":      len(db.Categories),
		"last_updated":    db.LastUpdated,
		"coverage":        fmt.Sprintf("%.1f%%", float64(len(db.Questions))/4146.0*100),
	}
}

// SaveToFile saves the database to a local file for offline use
func (db *TriviaDatabase) SaveToFile(filePath string) error {
	db.mutex.RLock()
	defer db.mutex.RUnlock()

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create database file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"Question", "Answer", "Category"}); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write all questions
	for _, questions := range db.Categories {
		for _, q := range questions {
			record := []string{q.Question, q.Answer, q.Category}
			if err := writer.Write(record); err != nil {
				return fmt.Errorf("failed to write CSV record: %w", err)
			}
		}
	}

	slog.Info("Database saved to file",
		"path", filePath,
		"questions", db.QuestionCount,
	)

	return nil
}

// LoadFromFile loads the database from a local file
func (db *TriviaDatabase) LoadFromFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open database file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read CSV file: %w", err)
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	// Clear existing data
	db.Questions = make(map[string]string)
	db.Categories = make(map[string][]TriviaQuestion)

	// Process records
	for _, record := range records {
		if len(record) < 3 {
			continue // Skip invalid records
		}

		question := strings.TrimSpace(record[0])
		answer := strings.TrimSpace(record[1])
		category := ""
		if len(record) >= 3 {
			category = strings.TrimSpace(record[2])
		}

		// Skip empty records
		if question == "" || answer == "" {
			continue
		}

		triviaQuestion := TriviaQuestion{
			Question: question,
			Answer:   answer,
			Category: category,
		}

		// Add to fast lookup map
		db.Questions[question] = answer

		// Add to category
		db.Categories[category] = append(db.Categories[category], triviaQuestion)
		db.QuestionCount++
	}

	db.LastUpdated = time.Now()

	slog.Info("Database loaded from file",
		"path", filePath,
		"questions", db.QuestionCount,
		"categories", len(db.Categories),
	)

	return nil
}
