package git

import (
	"fmt"
	"time"

	"github.com/yourorg/secret-manager/pkg/logger"
)

// PushWithRetry attempts to push with exponential backoff retry logic
func (c *GitClient) PushWithRetry(maxRetries int) error {
	if maxRetries <= 0 {
		maxRetries = 3 // Default
	}

	var lastErr error
	backoff := 1 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		logger.Info("Attempting push", "attempt", attempt, "max_retries", maxRetries)

		err := c.Push()
		if err == nil {
			if attempt > 1 {
				logger.Info("Push succeeded after retry", "attempt", attempt)
			}
			return nil
		}

		lastErr = err
		logger.Warn("Push failed", "error", err, "attempt", attempt, "max_retries", maxRetries)

		// Don't sleep after the last attempt
		if attempt < maxRetries {
			logger.Info("Retrying after backoff", "backoff_seconds", backoff.Seconds())
			time.Sleep(backoff)
			// Exponential backoff: 1s, 2s, 4s, 8s, ...
			backoff *= 2
		}
	}

	return fmt.Errorf("push failed after %d retries: %w", maxRetries, lastErr)
}
