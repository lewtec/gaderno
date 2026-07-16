package document

import (
	"strings"
	"testing"
)

func TestRoundTripEmpty(t *testing.T) {
	nb := NewEmpty()
	raw, err := Encode(nb)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.NBFormat != 4 {
		t.Fatalf("nbformat %d", got.NBFormat)
	}
	if len(got.Cells) != 1 {
		t.Fatalf("cells %d", len(got.Cells))
	}
	if got.Cells[0].ID == "" {
		t.Fatal("expected cell id")
	}
}

func TestDecodeMultilineArray(t *testing.T) {
	raw := []byte(`{
  "nbformat": 4,
  "nbformat_minor": 5,
  "metadata": {},
  "cells": [
    {
      "id": "abc12345",
      "cell_type": "code",
      "metadata": {},
      "source": ["print(", "1)\n"],
      "outputs": [],
      "execution_count": null
    }
  ]
}`)
	nb, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	src := nb.Cells[0].SourceString()
	if src != "print(1)\n" {
		t.Fatalf("source %q", src)
	}
}

func TestEnsureCellIDsStable(t *testing.T) {
	nb := NewEmpty()
	id := nb.Cells[0].ID
	EnsureCellIDs(nb)
	if nb.Cells[0].ID != id {
		t.Fatalf("id changed %s -> %s", id, nb.Cells[0].ID)
	}
}

func TestDecodeAssignsMissingIDs(t *testing.T) {
	raw := []byte(`{
  "nbformat": 4,
  "nbformat_minor": 4,
  "metadata": {},
  "cells": [{"cell_type": "markdown", "metadata": {}, "source": "# hi"}]
}`)
	nb, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if nb.Cells[0].ID == "" {
		t.Fatal("expected assigned id")
	}
	out, err := Encode(nb)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"id"`) {
		t.Fatal("expected id in encode")
	}
}
