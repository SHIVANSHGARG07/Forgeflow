package executors

import "testing"

// RunShell pure function hai (koi DB/network nahi), isliye unit test
// seedha payload de ke result check karta hai — DATABASE_URL ki zaroorat nahi.

func TestRunShell_Success(t *testing.T) {
	err := RunShell([]byte(`{"command": "echo hello"}`))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRunShell_CommandFails(t *testing.T) {
	err := RunShell([]byte(`{"command": "exit 1"}`))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRunShell_MissingCommand(t *testing.T) {
	err := RunShell([]byte(`{"command": ""}`))
	if err == nil {
		t.Fatal("expected error for missing command, got nil")
	}
}

func TestRunShell_InvalidJSON(t *testing.T) {
	err := RunShell([]byte(`not valid json`))
	if err == nil {
		t.Fatal("expected error for invalid json, got nil")
	}
}
