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

func TestEncodeCodeCellRequiredFields(t *testing.T) {
	nb := NewEmpty()
	raw, err := Encode(nb)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	// nbformat v4 code cells require outputs (array) and execution_count (null|int).
	if !strings.Contains(s, `"outputs": []`) && !strings.Contains(s, `"outputs":[]`) {
		t.Fatalf("code cell missing outputs array:\n%s", s)
	}
	if !strings.Contains(s, `"execution_count": null`) && !strings.Contains(s, `"execution_count":null`) {
		t.Fatalf("code cell missing execution_count null:\n%s", s)
	}

	// Empty Outputs slice (as ProjectNotebook emits) must not be dropped by omitempty.
	nb.Cells[0].Outputs = []Output{}
	nb.Cells[0].ExecutionCount = nil
	raw, err = Encode(nb)
	if err != nil {
		t.Fatal(err)
	}
	s = string(raw)
	if !strings.Contains(s, `"outputs"`) {
		t.Fatalf("empty outputs omitted:\n%s", s)
	}
	if !strings.Contains(s, `"execution_count"`) {
		t.Fatalf("nil execution_count omitted:\n%s", s)
	}
}

func TestEncodeMarkdownOmitsCodeFields(t *testing.T) {
	nb := &Notebook{
		NBFormat:      4,
		NBFormatMinor: 5,
		Metadata:      map[string]any{},
		Cells: []Cell{
			{
				ID:       "md1",
				CellType: CellMarkdown,
				Metadata: map[string]any{},
				Source:   NewMultiline("# hi"),
				// Accidentally set; must not appear on markdown cells.
				Outputs:        []Output{{OutputType: "stream"}},
				ExecutionCount: func() *int { n := 1; return &n }(),
			},
		},
	}
	raw, err := Encode(nb)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, `"outputs"`) {
		t.Fatalf("markdown cell should not emit outputs:\n%s", s)
	}
	if strings.Contains(s, `"execution_count"`) {
		t.Fatalf("markdown cell should not emit execution_count:\n%s", s)
	}
	if !strings.Contains(s, `"# hi"`) {
		t.Fatalf("missing source:\n%s", s)
	}
}

func TestEncodeCodeCellPreservesOutputsAndCount(t *testing.T) {
	ec := 3
	nb := NewEmpty()
	nb.Cells[0].Source = NewMultiline("1+1")
	nb.Cells[0].ExecutionCount = &ec
	nb.Cells[0].Outputs = []Output{
		{OutputType: "stream", Name: "stdout", Text: NewMultiline("2\n")},
	}
	raw, err := Encode(nb)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Cells[0].ExecutionCount == nil || *got.Cells[0].ExecutionCount != 3 {
		t.Fatalf("execution_count=%v", got.Cells[0].ExecutionCount)
	}
	if len(got.Cells[0].Outputs) != 1 || got.Cells[0].Outputs[0].OutputType != "stream" {
		t.Fatalf("outputs=%#v", got.Cells[0].Outputs)
	}
	if got.Cells[0].Outputs[0].Text.String() != "2\n" {
		t.Fatalf("stream text %q", got.Cells[0].Outputs[0].Text.String())
	}
}
