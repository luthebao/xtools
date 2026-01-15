package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"xtools/internal/domain"
	"xtools/internal/ports"
)

// ExcelExporter implements ExcelExporter interface
type ExcelExporter struct {
	baseDir string
}

// NewExcelExporter creates a new Excel exporter
func NewExcelExporter(baseDir string) (*ExcelExporter, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create export directory: %w", err)
	}

	return &ExcelExporter{baseDir: baseDir}, nil
}

// ExportTweets exports tweets to an Excel file
func (e *ExcelExporter) ExportTweets(accountID string, tweets []domain.Tweet, path string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Tweets"
	f.SetSheetName("Sheet1", sheet)

	// Set headers
	headers := []string{"Tweet ID", "Author", "Author Username", "Text", "Matched Keywords",
		"Likes", "Retweets", "Replies", "Views", "Created At", "Discovered At"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// Set header style
	style, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	f.SetCellStyle(sheet, "A1", "K1", style)

	// Add tweets
	for i, tweet := range tweets {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), tweet.ID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), tweet.AuthorName)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), tweet.AuthorUsername)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), tweet.Text)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), joinStrings(tweet.MatchedKeywords))
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), tweet.LikeCount)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), tweet.RetweetCount)
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row), tweet.ReplyCount)
		f.SetCellValue(sheet, fmt.Sprintf("I%d", row), tweet.ViewCount)
		f.SetCellValue(sheet, fmt.Sprintf("J%d", row), tweet.CreatedAt.Format(time.RFC3339))
		f.SetCellValue(sheet, fmt.Sprintf("K%d", row), tweet.DiscoveredAt.Format(time.RFC3339))
	}

	// Auto-fit columns
	for i := 1; i <= len(headers); i++ {
		col, _ := excelize.ColumnNumberToName(i)
		f.SetColWidth(sheet, col, col, 15)
	}
	f.SetColWidth(sheet, "D", "D", 50) // Text column wider

	return f.SaveAs(path)
}

// AppendTweets appends tweets to existing Excel file
func (e *ExcelExporter) AppendTweets(accountID string, tweets []domain.Tweet) error {
	path := e.GetExportPath(accountID)

	var f *excelize.File
	var err error

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create new file if doesn't exist
		return e.ExportTweets(accountID, tweets, path)
	}

	f, err = excelize.OpenFile(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	sheet := "Tweets"

	// Find last row
	rows, err := f.GetRows(sheet)
	if err != nil {
		return err
	}
	startRow := len(rows) + 1

	// Append tweets
	for i, tweet := range tweets {
		row := startRow + i
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), tweet.ID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), tweet.AuthorName)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), tweet.AuthorUsername)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), tweet.Text)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), joinStrings(tweet.MatchedKeywords))
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), tweet.LikeCount)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), tweet.RetweetCount)
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row), tweet.ReplyCount)
		f.SetCellValue(sheet, fmt.Sprintf("I%d", row), tweet.ViewCount)
		f.SetCellValue(sheet, fmt.Sprintf("J%d", row), tweet.CreatedAt.Format(time.RFC3339))
		f.SetCellValue(sheet, fmt.Sprintf("K%d", row), tweet.DiscoveredAt.Format(time.RFC3339))
	}

	return f.Save()
}

// ExportReplies exports replies to Excel
func (e *ExcelExporter) ExportReplies(accountID string, replies []domain.Reply, path string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Replies"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{"Reply ID", "Tweet ID", "Reply Text", "Status",
		"Generated At", "Posted At", "Posted Reply ID", "Tokens Used", "Error"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})
	f.SetCellStyle(sheet, "A1", "I1", style)

	for i, reply := range replies {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), reply.ID)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), reply.TweetID)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), reply.Text)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), string(reply.Status))
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), reply.GeneratedAt.Format(time.RFC3339))
		if reply.PostedAt != nil {
			f.SetCellValue(sheet, fmt.Sprintf("F%d", row), reply.PostedAt.Format(time.RFC3339))
		}
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), reply.PostedReplyID)
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row), reply.LLMTokensUsed)
		f.SetCellValue(sheet, fmt.Sprintf("I%d", row), reply.ErrorMessage)
	}

	f.SetColWidth(sheet, "C", "C", 50)

	return f.SaveAs(path)
}

// ExportMetrics exports metrics report to Excel
func (e *ExcelExporter) ExportMetrics(report domain.MetricsReport, path string) error {
	f := excelize.NewFile()
	defer f.Close()

	// Summary sheet
	summary := "Summary"
	f.SetSheetName("Sheet1", summary)

	f.SetCellValue(summary, "A1", "Metrics Report")
	f.SetCellValue(summary, "A2", "Generated At:")
	f.SetCellValue(summary, "B2", report.GeneratedAt.Format(time.RFC3339))
	f.SetCellValue(summary, "A3", "Account:")
	f.SetCellValue(summary, "B3", report.AccountID)

	f.SetCellValue(summary, "A5", "Profile Growth")
	f.SetCellValue(summary, "A6", "Followers Gained:")
	f.SetCellValue(summary, "B6", report.ProfileGrowth.FollowersGained)
	f.SetCellValue(summary, "A7", "Followers Lost:")
	f.SetCellValue(summary, "B7", report.ProfileGrowth.FollowersLost)
	f.SetCellValue(summary, "A8", "Net Change:")
	f.SetCellValue(summary, "B8", report.ProfileGrowth.NetChange)

	f.SetCellValue(summary, "A10", "Reply Performance")
	f.SetCellValue(summary, "A11", "Total Replies:")
	f.SetCellValue(summary, "B11", report.ReplyPerformance.TotalReplies)
	f.SetCellValue(summary, "A12", "Avg Likes:")
	f.SetCellValue(summary, "B12", report.ReplyPerformance.AvgLikesPerReply)

	// Top tweets sheet
	if len(report.TopTweets) > 0 {
		topTweets := "Top Tweets"
		f.NewSheet(topTweets)

		headers := []string{"Tweet ID", "Likes", "Retweets", "Replies", "Impressions"}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(topTweets, cell, h)
		}

		for i, t := range report.TopTweets {
			row := i + 2
			f.SetCellValue(topTweets, fmt.Sprintf("A%d", row), t.TweetID)
			f.SetCellValue(topTweets, fmt.Sprintf("B%d", row), t.LikeCount)
			f.SetCellValue(topTweets, fmt.Sprintf("C%d", row), t.RetweetCount)
			f.SetCellValue(topTweets, fmt.Sprintf("D%d", row), t.ReplyCount)
			f.SetCellValue(topTweets, fmt.Sprintf("E%d", row), t.Impressions)
		}
	}

	return f.SaveAs(path)
}

// GetExportPath returns the export path for an account
func (e *ExcelExporter) GetExportPath(accountID string) string {
	return filepath.Join(e.baseDir, accountID+"_tweets.xlsx")
}

// LoadTweets loads tweets from an Excel file
func (e *ExcelExporter) LoadTweets(path string) ([]domain.Tweet, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := f.GetRows("Tweets")
	if err != nil {
		return nil, err
	}

	var tweets []domain.Tweet
	for i, row := range rows {
		if i == 0 { // Skip header
			continue
		}
		if len(row) < 11 {
			continue
		}

		tweet := domain.Tweet{
			ID:             row[0],
			AuthorName:     row[1],
			AuthorUsername: row[2],
			Text:           row[3],
		}

		if createdAt, err := time.Parse(time.RFC3339, row[9]); err == nil {
			tweet.CreatedAt = createdAt
		}
		if discoveredAt, err := time.Parse(time.RFC3339, row[10]); err == nil {
			tweet.DiscoveredAt = discoveredAt
		}

		tweets = append(tweets, tweet)
	}

	return tweets, nil
}

func joinStrings(strs []string) string {
	return strings.Join(strs, ", ")
}

// Ensure ExcelExporter implements ExcelExporter interface
var _ ports.ExcelExporter = (*ExcelExporter)(nil)
