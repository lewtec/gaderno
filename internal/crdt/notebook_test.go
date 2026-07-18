package crdt

import (
	"testing"

	"github.com/lucasew/gaderno/internal/document"
)

func TestLoadProjectRoundTrip(t *testing.T) {
	nb := document.NewEmpty()
	nb.Cells[0].Source = document.NewMultiline("print(42)")
	d := New()
	if err := d.LoadFromNotebook(nb); err != nil {
		t.Fatal(err)
	}
	ids := d.CellIDs()
	if len(ids) != 1 {
		t.Fatalf("ids %v", ids)
	}
	if got := d.Source(ids[0]); got != "print(42)" {
		t.Fatalf("source %q", got)
	}
	out := d.ProjectNotebook()
	if out.Cells[0].SourceString() != "print(42)" {
		t.Fatalf("project %q", out.Cells[0].SourceString())
	}
}

func TestLoadProjectPreservesOutputsAndMeta(t *testing.T) {
	ec := 7
	nb := document.NewEmpty()
	nb.Cells[0].Source = document.NewMultiline("print(1)")
	nb.Cells[0].ExecutionCount = &ec
	nb.Cells[0].Outputs = []document.Output{
		{
			OutputType: "stream",
			Name:       "stdout",
			Text:       document.NewMultiline("1\n"),
		},
		{
			OutputType: "execute_result",
			Data:       map[string]any{"text/plain": "1"},
			ExecutionCount: func() *int {
				n := 7
				return &n
			}(),
		},
	}
	// Nested kernelspec must survive flatten → unflatten (not kernelspec_json on disk).
	nb.Metadata = map[string]any{
		"kernelspec": map[string]any{
			"name":         "python3",
			"display_name": "Python 3",
			"language":     "python",
		},
		"language_info": map[string]any{"name": "python"},
	}

	d := New()
	if err := d.LoadFromNotebook(nb); err != nil {
		t.Fatal(err)
	}
	out := d.ProjectNotebook()

	if out.Cells[0].ExecutionCount == nil || *out.Cells[0].ExecutionCount != 7 {
		t.Fatalf("execution_count = %v, want 7", out.Cells[0].ExecutionCount)
	}
	if len(out.Cells[0].Outputs) != 2 {
		t.Fatalf("outputs len = %d, want 2", len(out.Cells[0].Outputs))
	}
	if out.Cells[0].Outputs[0].OutputType != "stream" {
		t.Fatalf("output[0].type = %q", out.Cells[0].Outputs[0].OutputType)
	}
	if got := out.Cells[0].Outputs[0].Text.String(); got != "1\n" {
		t.Fatalf("stream text %q", got)
	}
	ks, ok := out.Metadata["kernelspec"].(map[string]any)
	if !ok {
		t.Fatalf("kernelspec type %T value %#v (should not be kernelspec_json)", out.Metadata["kernelspec"], out.Metadata)
	}
	if ks["name"] != "python3" {
		t.Fatalf("kernelspec.name = %v", ks["name"])
	}
	if _, hasJSON := out.Metadata["kernelspec_json"]; hasJSON {
		t.Fatal("projected metadata still has kernelspec_json")
	}
	li, ok := out.Metadata["language_info"].(map[string]any)
	if !ok || li["name"] != "python" {
		t.Fatalf("language_info = %#v", out.Metadata["language_info"])
	}
}

func TestProjectEmptyCodeCellOutputs(t *testing.T) {
	nb := document.NewEmpty()
	d := New()
	if err := d.LoadFromNotebook(nb); err != nil {
		t.Fatal(err)
	}
	out := d.ProjectNotebook()
	if out.Cells[0].Outputs == nil {
		t.Fatal("code cell outputs should be empty slice, not nil")
	}
	if len(out.Cells[0].Outputs) != 0 {
		t.Fatalf("outputs %v", out.Cells[0].Outputs)
	}
}

func TestSetSourceServer(t *testing.T) {
	nb := document.NewEmpty()
	d := New()
	if err := d.LoadFromNotebook(nb); err != nil {
		t.Fatal(err)
	}
	id := d.CellIDs()[0]
	if err := d.SetSourceServer(id, "hello"); err != nil {
		t.Fatal(err)
	}
	if d.Source(id) != "hello" {
		t.Fatalf("source %q", d.Source(id))
	}
}
