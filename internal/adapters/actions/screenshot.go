package actions

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"xtools/internal/ports"
)

// ScreenshotCapture captures webpage screenshots using go-rod
type ScreenshotCapture struct {
	browser   *rod.Browser
	dataDir   string
	mu        sync.Mutex
	isClosing bool
}

// NewScreenshotCapture creates a new screenshot capture instance
func NewScreenshotCapture(dataDir string) (*ScreenshotCapture, error) {
	// Create screenshots directory
	screenshotDir := filepath.Join(dataDir, "screenshots")
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create screenshots directory: %w", err)
	}

	return &ScreenshotCapture{
		dataDir: screenshotDir,
	}, nil
}

// ensureBrowser lazily initializes the browser when needed
func (c *ScreenshotCapture) ensureBrowser() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosing {
		return fmt.Errorf("screenshot capture is closing")
	}

	if c.browser != nil {
		return nil
	}

	log.Println("[ScreenshotCapture] Launching browser...")

	// Launch browser (headless)
	l := launcher.New().
		Headless(true).
		NoSandbox(true).
		Leakless(false) // Disable leakless to avoid antivirus false positives

	url, err := l.Launch()
	if err != nil {
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("failed to connect to browser: %w", err)
	}

	c.browser = browser
	log.Println("[ScreenshotCapture] Browser launched successfully")
	return nil
}

// CaptureMarket captures the Polymarket market page
func (c *ScreenshotCapture) CaptureMarket(ctx context.Context, marketSlug string) (string, error) {
	if marketSlug == "" {
		return "", fmt.Errorf("market slug is required")
	}

	url := fmt.Sprintf("https://polymarket.com/event/%s", marketSlug)
	return c.capture(ctx, url, "market", marketSlug)
}

// CaptureProfile captures the Polymarket wallet profile page
func (c *ScreenshotCapture) CaptureProfile(ctx context.Context, walletAddress string) (string, error) {
	if walletAddress == "" {
		return "", fmt.Errorf("wallet address is required")
	}

	url := fmt.Sprintf("https://polymarket.com/profile/%s", walletAddress)
	return c.capture(ctx, url, "profile", shortenAddressForFilename(walletAddress))
}

func (c *ScreenshotCapture) capture(ctx context.Context, url, prefix, identifier string) (string, error) {
	if err := c.ensureBrowser(); err != nil {
		return "", err
	}

	log.Printf("[ScreenshotCapture] Capturing %s: %s", prefix, url)

	// Create page
	page, err := c.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return "", fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// Set viewport size for consistent screenshots
	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:  1280,
		Height: 800,
	}); err != nil {
		log.Printf("[ScreenshotCapture] Failed to set viewport: %v", err)
	}

	// Navigate to page
	if err := page.Navigate(url); err != nil {
		return "", fmt.Errorf("failed to navigate: %w", err)
	}

	// Wait for page to load
	if err := page.WaitLoad(); err != nil {
		log.Printf("[ScreenshotCapture] Wait load error (continuing): %v", err)
	}

	// Additional wait for dynamic content
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(3 * time.Second):
	}

	// Generate filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s.png", prefix, identifier, timestamp)
	filepath := filepath.Join(c.dataDir, filename)

	// Capture screenshot
	data, err := page.Screenshot(false, nil)
	if err != nil {
		return "", fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// Save to file
	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to save screenshot: %w", err)
	}

	log.Printf("[ScreenshotCapture] Screenshot saved: %s", filepath)
	return filepath, nil
}

// Close cleans up browser resources
func (c *ScreenshotCapture) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.isClosing = true

	if c.browser != nil {
		log.Println("[ScreenshotCapture] Closing browser...")
		if err := c.browser.Close(); err != nil {
			log.Printf("[ScreenshotCapture] Error closing browser: %v", err)
			return err
		}
		c.browser = nil
	}
	return nil
}

// CleanupOldScreenshots removes screenshots older than the specified duration
func (c *ScreenshotCapture) CleanupOldScreenshots(maxAge time.Duration) error {
	entries, err := os.ReadDir(c.dataDir)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(c.dataDir, entry.Name())
			if err := os.Remove(path); err != nil {
				log.Printf("[ScreenshotCapture] Failed to remove old screenshot %s: %v", entry.Name(), err)
			} else {
				log.Printf("[ScreenshotCapture] Removed old screenshot: %s", entry.Name())
			}
		}
	}
	return nil
}

func shortenAddressForFilename(addr string) string {
	if len(addr) <= 12 {
		return addr
	}
	return addr[:6] + "_" + addr[len(addr)-4:]
}

// Ensure ScreenshotCapture implements ports.ScreenshotCapture
var _ ports.ScreenshotCapture = (*ScreenshotCapture)(nil)
