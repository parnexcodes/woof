package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestUploadCommand_NoFlagsError(t *testing.T) {
	// Test that running upload without any flags shows the expected error
	root := rootCmd
	root.SetArgs([]string{"upload"})

	// Capture output
	output := &bytes.Buffer{}
	root.SetErr(output)

	// Execute the command
	err := root.Execute()

	if err == nil {
		t.Errorf("expected error about missing files/folders, but got none")
		return
	}

	// Check that we get the expected error message
	if !strings.Contains(err.Error(), "no files or folders specified") {
		t.Errorf("expected error containing 'no files or folders specified', but got: %v", err)
	}
}

func TestUploadCommand_HelpText(t *testing.T) {
	// Test that help text contains our new flags
	buf := new(bytes.Buffer)
	root := rootCmd
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"upload", "--help"})

	err := root.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	helpText := buf.String()
	expectedFlags := []string{
		"--all",
		"--file", "-f",
		"--folder", "-d",
		"glob patterns",
	}

	for _, flag := range expectedFlags {
		if !strings.Contains(helpText, flag) {
			t.Errorf("help text should contain %s, but didn't. Full help:\n%s", flag, helpText)
		}
	}
}

func TestExpandGlobPatterns(t *testing.T) {
	// Create test files
	testFiles := []string{"test1.txt", "test2.txt", "other.log"}
	for _, file := range testFiles {
		_, err := os.Create(file)
		if err != nil {
			t.Fatalf("failed to create test file %s: %v", file, err)
		}
	}
	defer func() {
		for _, file := range testFiles {
			os.Remove(file)
		}
	}()

	tests := []struct {
		name     string
		patterns []string
		expected []string
	}{
		{
			name:     "no glob patterns",
			patterns: []string{"test1.txt", "other.log"},
			expected: []string{"test1.txt", "other.log"},
		},
		{
			name:     "simple glob",
			patterns: []string{"test*.txt"},
			expected: []string{"test1.txt", "test2.txt"},
		},
		{
			name:     "mixed patterns",
			patterns: []string{"test*.txt", "other.log"},
			expected: []string{"test1.txt", "test2.txt", "other.log"},
		},
		{
			name:     "non-matching glob",
			patterns: []string{"*.nonexistent"},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandGlobPatterns(tt.patterns)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Sort both slices for comparison
			resultStr := strings.Join(result, ",")
			expectedStr := strings.Join(tt.expected, ",")

			if resultStr != expectedStr {
				t.Errorf("expected %s, got %s", expectedStr, resultStr)
			}
		})
	}
}

func TestValidatePaths(t *testing.T) {
	// Create test file and directory
	testFile := "test_validation.txt"
	testDir := "test_validation_dir"

	_, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	err = os.Mkdir(testDir, 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}
	defer os.RemoveAll(testDir)

	tests := []struct {
		name        string
		files       []string
		folders     []string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid file and directory",
			files:       []string{testFile},
			folders:     []string{testDir},
			expectError: false,
		},
		{
			name:        "nonexistent file",
			files:       []string{"/nonexistent/file.txt"},
			folders:     []string{},
			expectError: true,
			errorMsg:    "file does not exist",
		},
		{
			name:        "directory as file",
			files:       []string{testDir},
			folders:     []string{},
			expectError: true,
			errorMsg:    "is a directory, but --file flag requires a file",
		},
		{
			name:        "file as directory",
			files:       []string{},
			folders:     []string{testFile},
			expectError: true,
			errorMsg:    "is a file, but --folder/-d flag requires a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePaths(tt.files, tt.folders)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing '%s', but got none", tt.errorMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing '%s', but got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, but got: %v", err)
				}
			}
		})
	}
}