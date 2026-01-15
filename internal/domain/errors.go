package domain

import "errors"

// Domain errors
var (
	// Account errors
	ErrAccountNotFound      = errors.New("account not found")
	ErrAccountAlreadyExists = errors.New("account already exists")
	ErrAccountNotEnabled    = errors.New("account is not enabled")
	ErrInvalidAuthType      = errors.New("invalid authentication type")

	// Authentication errors
	ErrNotAuthenticated    = errors.New("not authenticated")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrTokenExpired        = errors.New("authentication token expired")
	ErrCookiesExpired      = errors.New("browser cookies expired")

	// Twitter API errors
	ErrRateLimited         = errors.New("rate limit exceeded")
	ErrTweetNotFound       = errors.New("tweet not found")
	ErrUserNotFound        = errors.New("user not found")
	ErrReplyFailed         = errors.New("failed to post reply")
	ErrSearchFailed        = errors.New("search failed")
	ErrAlreadyReplied      = errors.New("already replied to this tweet")

	// LLM errors
	ErrLLMFailed           = errors.New("LLM request failed")
	ErrLLMRateLimited      = errors.New("LLM rate limit exceeded")
	ErrLLMInvalidResponse  = errors.New("invalid LLM response")
	ErrLLMContextTooLong   = errors.New("context too long for LLM")

	// Storage errors
	ErrConfigNotFound      = errors.New("configuration not found")
	ErrConfigInvalid       = errors.New("invalid configuration")
	ErrStorageWrite        = errors.New("failed to write to storage")
	ErrStorageRead         = errors.New("failed to read from storage")

	// Worker errors
	ErrWorkerAlreadyRunning = errors.New("worker already running")
	ErrWorkerNotRunning     = errors.New("worker not running")

	// Validation errors
	ErrInvalidKeyword      = errors.New("invalid keyword")
	ErrInvalidUsername     = errors.New("invalid username")
	ErrEmptyReply          = errors.New("reply text is empty")
	ErrReplyTooLong        = errors.New("reply exceeds character limit")
)

// AppError wraps domain errors with additional context
type AppError struct {
	Err       error
	Message   string
	AccountID string
	Code      string
}

func (e *AppError) Error() string {
	if e.Message != "" {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Err.Error()
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new application error
func NewAppError(err error, message, accountID, code string) *AppError {
	return &AppError{
		Err:       err,
		Message:   message,
		AccountID: accountID,
		Code:      code,
	}
}
