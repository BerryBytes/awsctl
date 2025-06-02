package sso

import (
	"errors"
	"strings"
	"testing"

	mock_sso "github.com/BerryBytes/awsctl/tests/mock/sso"
	promptUtils "github.com/BerryBytes/awsctl/utils/prompt"
	"github.com/golang/mock/gomock"
	"github.com/manifoldco/promptui"
	"github.com/stretchr/testify/assert"
)

func mockValidateStartURL(input string) error {
	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		return errors.New("invalid URL format")
	}
	return nil
}

func TestPromptUI_PromptWithDefault(t *testing.T) {
	tests := []struct {
		name         string
		label        string
		defaultValue string
		input        string
		inputErr     error
		wantResult   string
		wantErr      bool
		errContains  string
	}{
		{
			name:         "Valid input",
			label:        "Enter name",
			defaultValue: "default",
			input:        "test-name",
			wantResult:   "test-name",
			wantErr:      false,
		},
		{
			name:         "Empty input uses default",
			label:        "Enter name",
			defaultValue: "default",
			input:        "",
			wantResult:   "default",
			wantErr:      false,
		},
		{
			name:         "Whitespace input uses default",
			label:        "Enter name",
			defaultValue: "default",
			input:        "   ",
			wantResult:   "default",
			wantErr:      false,
		},
		{
			name:         "Empty input with empty default fails",
			label:        "Enter name",
			defaultValue: "",
			input:        "",
			inputErr:     errors.New("input cannot be empty"),
			wantErr:      true,
			errContains:  "prompt failed: input cannot be empty",
		},
		{
			name:         "Interrupted prompt",
			label:        "Enter name",
			defaultValue: "default",
			inputErr:     promptui.ErrInterrupt,
			wantErr:      true,
			errContains:  promptUtils.ErrInterrupted.Error(),
		},
		{
			name:         "EOF prompt",
			label:        "Enter name",
			defaultValue: "default",
			inputErr:     promptui.ErrEOF,
			wantErr:      true,
			errContains:  promptUtils.ErrInterrupted.Error(),
		},
		{
			name:         "Generic prompt error",
			label:        "Enter name",
			defaultValue: "default",
			inputErr:     errors.New("prompt error"),
			wantErr:      true,
			errContains:  "prompt failed: prompt error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRunner := mock_sso.NewMockPromptRunner(ctrl)
			mockRunner.EXPECT().
				RunPrompt(tt.label, tt.defaultValue, gomock.Any()).
				Return(tt.input, tt.inputErr)

			p := &PromptUI{runner: mockRunner}
			result, err := p.PromptWithDefault(tt.label, tt.defaultValue)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantResult, result)
		})
	}
}

func TestPromptUI_SelectFromList(t *testing.T) {
	tests := []struct {
		name        string
		label       string
		items       []string
		input       string
		inputErr    error
		wantResult  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "Valid selection",
			label:      "Select an option",
			items:      []string{"option1", "option2"},
			input:      "option1",
			wantResult: "option1",
			wantErr:    false,
		},
		{
			name:        "Interrupted selection",
			label:       "Select an option",
			items:       []string{"option1", "option2"},
			inputErr:    promptui.ErrInterrupt,
			wantErr:     true,
			errContains: promptUtils.ErrInterrupted.Error(),
		},
		{
			name:        "EOF selection",
			label:       "Select an option",
			items:       []string{"option1", "option2"},
			inputErr:    promptui.ErrEOF,
			wantErr:     true,
			errContains: promptUtils.ErrInterrupted.Error(),
		},
		{
			name:        "Generic prompt error",
			label:       "Select an option",
			items:       []string{"option1", "option2"},
			inputErr:    errors.New("select error"),
			wantErr:     true,
			errContains: "prompt failed: select error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRunner := mock_sso.NewMockPromptRunner(ctrl)
			mockRunner.EXPECT().
				RunSelect(tt.label, tt.items).
				Return(tt.input, tt.inputErr)

			p := &PromptUI{runner: mockRunner}
			result, err := p.SelectFromList(tt.label, tt.items)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantResult, result)
		})
	}
}

func TestPromptUI_PromptYesNo(t *testing.T) {
	tests := []struct {
		name         string
		label        string
		defaultValue bool
		input        string
		inputErr     error
		wantResult   bool
		wantErr      bool
		errContains  string
	}{
		{
			name:         "Input 'y' returns true",
			label:        "Confirm?",
			defaultValue: false,
			input:        "y",
			wantResult:   true,
			wantErr:      false,
		},
		{
			name:         "Input 'n' returns false",
			label:        "Confirm?",
			defaultValue: true,
			input:        "n",
			wantResult:   false,
			wantErr:      false,
		},
		{
			name:         "Empty input with default true",
			label:        "Confirm?",
			defaultValue: true,
			input:        "",
			wantResult:   true,
			wantErr:      false,
		},
		{
			name:         "Empty input with default false",
			label:        "Confirm?",
			defaultValue: false,
			input:        "",
			wantResult:   false,
			wantErr:      false,
		},
		{
			name:         "Invalid input fails",
			label:        "Confirm?",
			defaultValue: false,
			input:        "x",
			inputErr:     errors.New("input must be 'y' or 'n'"),
			wantErr:      true,
			errContains:  "prompt failed: input must be 'y' or 'n'",
		},
		{
			name:         "Interrupted prompt",
			label:        "Confirm?",
			defaultValue: false,
			inputErr:     promptui.ErrInterrupt,
			wantErr:      true,
			errContains:  promptUtils.ErrInterrupted.Error(),
		},
		{
			name:         "EOF prompt",
			label:        "Confirm?",
			defaultValue: false,
			inputErr:     promptui.ErrEOF,
			wantErr:      true,
			errContains:  promptUtils.ErrInterrupted.Error(),
		},
		{
			name:         "Generic prompt error",
			label:        "Confirm?",
			defaultValue: false,
			inputErr:     errors.New("prompt error"),
			wantErr:      true,
			errContains:  "prompt failed: prompt error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRunner := mock_sso.NewMockPromptRunner(ctrl)
			defaultStr := "n"
			if tt.defaultValue {
				defaultStr = "y"
			}
			mockRunner.EXPECT().
				RunPrompt(tt.label, defaultStr, gomock.Any()).
				Return(tt.input, tt.inputErr)

			p := &PromptUI{runner: mockRunner}
			result, err := p.PromptYesNo(tt.label, tt.defaultValue)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantResult, result)
		})
	}
}

func TestPromptUI_PromptRequired(t *testing.T) {
	originalValidateStartURL := validateStartURLFunc
	validateStartURLFunc = mockValidateStartURL
	defer func() { validateStartURLFunc = originalValidateStartURL }()

	tests := []struct {
		name        string
		label       string
		input       string
		inputErr    error
		wantResult  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "Valid URL input",
			label:      "Enter SSO URL",
			input:      "https://test.awsapps.com/start",
			wantResult: "https://test.awsapps.com/start",
			wantErr:    false,
		},
		{
			name:        "Empty input fails",
			label:       "Enter SSO URL",
			input:       "",
			inputErr:    errors.New("input is required"),
			wantErr:     true,
			errContains: "prompt failed: input is required",
		},
		{
			name:        "Invalid URL fails",
			label:       "Enter SSO URL",
			input:       "invalid-url",
			inputErr:    errors.New("invalid URL format"),
			wantErr:     true,
			errContains: "prompt failed: invalid URL format",
		},
		{
			name:        "Interrupted prompt",
			label:       "Enter SSO URL",
			inputErr:    promptui.ErrInterrupt,
			wantErr:     true,
			errContains: promptUtils.ErrInterrupted.Error(),
		},
		{
			name:        "EOF prompt",
			label:       "Enter SSO URL",
			inputErr:    promptui.ErrEOF,
			wantErr:     true,
			errContains: promptUtils.ErrInterrupted.Error(),
		},
		{
			name:        "Generic prompt error",
			label:       "Enter SSO URL",
			inputErr:    errors.New("prompt error"),
			wantErr:     true,
			errContains: "prompt failed: prompt error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockRunner := mock_sso.NewMockPromptRunner(ctrl)
			mockRunner.EXPECT().
				RunPrompt(tt.label, "", gomock.Any()).
				Return(tt.input, tt.inputErr)

			p := &PromptUI{runner: mockRunner}
			result, err := p.PromptRequired(tt.label)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantResult, result)
		})
	}
}

func TestNewPrompter(t *testing.T) {
	prompter := NewPrompter()
	assert.NotNil(t, prompter)
	_, ok := prompter.(*PromptUI)
	assert.True(t, ok, "NewPrompter should return a *PromptUI")
}
