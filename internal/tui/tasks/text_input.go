package tasks

import "wydo/internal/tui/shared"

// TextInputModel is an alias for shared.TextInputModel.
type TextInputModel = shared.TextInputModel

// TextInputResultMsg is an alias for shared.TextInputResultMsg.
type TextInputResultMsg = shared.TextInputResultMsg

// NewTextInput creates a new text input component.
var NewTextInput = shared.NewTextInput

// NewDateInput creates a text input configured for date entry.
var NewDateInput = shared.NewDateInput

// ValidateDateFormat validates that the input is in yyyy-MM-dd format.
var ValidateDateFormat = shared.ValidateDateFormat
