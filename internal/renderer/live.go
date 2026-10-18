package renderer

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
)

// LiveURL opens the diagram in the Mermaid Live Editor's viewer. The
// diagram travels in the URL fragment, which browsers don't send to the
// server.
func LiveURL(src string) string {
	state, _ := json.Marshal(map[string]any{
		"code":    src,
		"mermaid": `{"theme":"default"}`,
	})
	var buf bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	zw.Write(state)
	zw.Close()
	return "https://mermaid.live/view#pako:" + base64.RawURLEncoding.EncodeToString(buf.Bytes())
}
