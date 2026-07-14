package scaffold

import (
	"io/fs"
	"testing"
)

func TestEmbeddedTemplate(t *testing.T) {
	if _, err := fs.Stat(assets, "_assets/template/go.mod.tmpl"); err != nil {
		t.Fatal(err)
	}
}
