package crdt

import (
	"encoding/json"
	"fmt"

	"github.com/lucasew/gaderno/internal/document"
	ycrdt "github.com/reearth/ygo/crdt"
)

// Root type names in the shared Y.Doc.
const (
	RootCells    = "cells"    // Y.Array of cell id strings
	RootCellData = "cellData" // Y.Map cell_id -> scalar fields (JSON for complex)
	RootMeta     = "meta"     // Y.Map notebook metadata
)

// Origins used for Transact so observers can distinguish writers.
const (
	OriginServer = "gaderno.server"
	OriginLoad   = "gaderno.load"
)

// NotebookDoc wraps a ygo document with notebook helpers.
type NotebookDoc struct {
	Doc *ycrdt.Doc
}

// New empty collaborative notebook document.
func New() *NotebookDoc {
	return &NotebookDoc{Doc: ycrdt.New()}
}

// ApplyUpdate applies a Yjs binary update.
func (n *NotebookDoc) ApplyUpdate(update []byte) error {
	return n.Doc.ApplyUpdate(update)
}

// EncodeStateAsUpdate returns full document state as a Yjs update.
func (n *NotebookDoc) EncodeStateAsUpdate() []byte {
	return n.Doc.EncodeStateAsUpdate()
}

// LoadFromNotebook populates the CRDT from an ipynb model.
// Root types are resolved outside Transact (ygo Get* takes the doc lock).
func (n *NotebookDoc) LoadFromNotebook(nb *document.Notebook) error {
	if nb == nil {
		return fmt.Errorf("nil notebook")
	}
	document.EnsureCellIDs(nb)

	meta := n.Doc.GetMap(RootMeta)
	cells := n.Doc.GetArray(RootCells)
	cellData := n.Doc.GetMap(RootCellData)

	// Pre-create source texts outside the transaction.
	sources := make(map[string]*ycrdt.YText, len(nb.Cells))
	for i := range nb.Cells {
		id := nb.Cells[i].ID
		sources[id] = n.Doc.GetText(sourceKey(id))
	}

	return n.Doc.TransactE(func(txn *ycrdt.Transaction) error {
		for k, v := range flattenMeta(nb.Metadata) {
			meta.Set(txn, k, v)
		}
		for i := range nb.Cells {
			c := nb.Cells[i]
			id := c.ID
			cells.Push(txn, []any{id})

			// Flatten cell fields into cellData as "id.field" keys to avoid nested YMap lock issues.
			cellData.Set(txn, id+".type", string(c.CellType))
			cellData.Set(txn, id+".status", "idle")
			if c.ExecutionCount != nil {
				cellData.Set(txn, id+".execution_count", float64(*c.ExecutionCount))
			}
			outs, _ := json.Marshal(c.Outputs)
			cellData.Set(txn, id+".outputs_json", string(outs))

			st := sources[id]
			if s := c.SourceString(); s != "" {
				st.Insert(txn, 0, s, nil)
			}
		}
		return nil
	}, OriginLoad)
}

// Source returns cell source text.
func (n *NotebookDoc) Source(cellID string) string {
	return n.Doc.GetText(sourceKey(cellID)).ToString()
}

// CellIDs returns ordered cell ids.
func (n *NotebookDoc) CellIDs() []string {
	arr := n.Doc.GetArray(RootCells)
	out := make([]string, 0, arr.Len())
	for i := 0; i < arr.Len(); i++ {
		v := arr.Get(i)
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// ProjectNotebook builds an ipynb snapshot from CRDT state.
func (n *NotebookDoc) ProjectNotebook() *document.Notebook {
	nb := &document.Notebook{
		NBFormat:      4,
		NBFormatMinor: 5,
		Metadata:      map[string]any{},
	}
	meta := n.Doc.GetMap(RootMeta)
	for _, k := range meta.Keys() {
		if v, ok := meta.Get(k); ok {
			nb.Metadata[k] = v
		}
	}
	cellData := n.Doc.GetMap(RootCellData)
	for _, id := range n.CellIDs() {
		c := document.Cell{
			ID:       id,
			CellType: document.CellCode,
			Metadata: map[string]any{},
			Source:   document.NewMultiline(n.Source(id)),
		}
		if t, ok := cellData.Get(id + ".type"); ok {
			if s, ok := t.(string); ok {
				c.CellType = document.CellType(s)
			}
		}
		nb.Cells = append(nb.Cells, c)
	}
	if len(nb.Cells) == 0 {
		return document.NewEmpty()
	}
	return nb
}

// SetSourceServer replaces cell source (server-side writer).
func (n *NotebookDoc) SetSourceServer(cellID, source string) error {
	st := n.Doc.GetText(sourceKey(cellID))
	return n.Doc.TransactE(func(txn *ycrdt.Transaction) error {
		if n := st.Len(); n > 0 {
			st.Delete(txn, 0, n)
		}
		if source != "" {
			st.Insert(txn, 0, source, nil)
		}
		return nil
	}, OriginServer)
}

func sourceKey(cellID string) string {
	return "source:" + cellID
}

func flattenMeta(m map[string]any) map[string]any {
	out := map[string]any{}
	if m == nil {
		return out
	}
	for k, v := range m {
		switch v.(type) {
		case string, bool, float64, int, int64, nil:
			out[k] = v
		default:
			b, err := json.Marshal(v)
			if err != nil {
				continue
			}
			out[k+"_json"] = string(b)
		}
	}
	return out
}
