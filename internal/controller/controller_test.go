package controller

import "testing"

func TestSelectGateID(t *testing.T) {
	locations := []map[string]any{
		{"id": float64(1), "shortname": "NL", "name": "Netherlands"},
		{"id": float64(10), "shortname": "BST", "name": "Лучший сервер"},
	}
	got, err := SelectGateID(locations, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != 10 {
		t.Fatalf("gate = %d", got)
	}
	got, err = SelectGateID(locations, "NL")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("gate = %d", got)
	}
	got, err = SelectGateID(locations, "42")
	if err != nil {
		t.Fatal(err)
	}
	if got != 42 {
		t.Fatalf("gate = %d", got)
	}
}

func TestRedact(t *testing.T) {
	raw := map[string]any{"uuid": "1234567890abcdef", "nested": []any{map[string]any{"short_id": "abcdef123456"}}}
	redacted := Redact(raw).(map[string]any)
	if redacted["uuid"] == raw["uuid"] {
		t.Fatal("uuid was not redacted")
	}
}
