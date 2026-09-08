package core

import "strings"

// TSVColumns are the four readable-view columns.
var TSVColumns = [4]string{"title", "index", "start", "path"}

// ToTSV serializes blocks as a four-column TSV (with header), for self-check
// and optional distribution/debugging.
func ToTSV(blocks []Block) string {
	var b strings.Builder
	b.WriteString(strings.Join(TSVColumns[:], "\t"))
	b.WriteByte('\n')
	for _, e := range blocks {
		b.WriteString(tsvEscape(e.Title))
		b.WriteByte('\t')
		b.WriteString(tsvEscape(e.IndexText()))
		b.WriteByte('\t')
		b.WriteString(itoa(e.Start))
		b.WriteByte('\t')
		b.WriteString(tsvEscape(e.Path))
		b.WriteByte('\n')
	}
	return b.String()
}

// tsvEscape escapes tab/newline/backslash so each field stays single-line.
func tsvEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\t", "\\t")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [24]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
