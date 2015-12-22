package refs

import (
	"clive/app"
	"clive/app/nsutil"
	"fmt"
	"io"
	"strings"
)

func (b *Bib) loadBib(fn string) error {
	lnc := nsutil.GetLines(fn)
	ln := <-lnc
	if !strings.Contains(ln, "bib2ref ok") {
		close(lnc, "not for me")
		return nil
	}
	app.Dprintf("add file %s\n", fn)
	return b.loadLines(bib2ref(lnc))
}

func bib2ref(lnc <-chan string) chan string {
	rc := make(chan string)
	go parseBib(lnc, rc)
	return rc
}

func parseBib(lnc <-chan string, outc chan<- string) {
	for r, err := parse1Bib(lnc); err != io.EOF; r, err = parse1Bib(lnc) {
		for _, ln := range r {
			outc <- ln
		}
		outc <- "\n"
	}
	close(outc, cerror(lnc))
}

// trim space and no final ,
func cleanLn(ln string) string {
	ln = strings.TrimSpace(ln)
	if len(ln) > 0 && ln[len(ln)-1] == ',' {
		ln = ln[:len(ln)-1]
	}
	return ln
}

func parse1Bib(lnc <-chan string) ([]string, error) {
	for {
		ln, ok := <-lnc
		if !ok {
			return nil, io.EOF
		}
		ln = cleanLn(ln)
		if ln == "" {
			continue
		}
		if ln[0] != '@' {
			continue
		}
		out := []string{}
		brace := strings.IndexRune(ln, '{')
		if brace < 0 {
			continue
		}
		ln = ln[brace+1:]
		toks := strings.Fields(ln)
		if len(toks) > 0 {
			out = append(out, "%K "+toks[0]+"\n")
		}
		for fld, err := parseField(lnc); err != io.EOF && fld != nil; fld, err = parseField(lnc) {
			out = append(out, fld...)
		}
		return out, nil
	}
}

var keys = map[string]string{
	"author":       "A",
	"title":        "T",
	"note":         "O",
	"booktitle":    "B",
	"series":       "S",
	"year":         "D",
	"location":     "C",
	"address":      "C",
	"pages":        "P",
	"url":          "O",
	"publisher":    "I",
	"key":          "K",
	"volume":       "V",
	"journal":      "J",
	"keywords":     "K",
	"organization": "I",
}

func parseField(lnc <-chan string) ([]string, error) {
	for {
		ln, ok := <-lnc
		if !ok {
			return nil, io.EOF
		}
		ln = cleanLn(ln)
		if ln == "" {
			return nil, nil
		}
		if ln[0] == '%' {
			continue
		}
		toks := strings.SplitN(ln, "=", 2)
		if len(toks) != 2 {
			continue
		}
		fname := strings.ToLower(strings.TrimSpace(toks[0]))
		if k := keys[fname]; k != "" {
			val := parseValue(k, toks[1])
			for i, v := range val {
				val[i] = fmt.Sprintf("%%%s %s\n", k, strings.TrimSpace(v))
			}
			return val, nil
		}
	}
}

func parseValue(key, ln string) []string {
	ln = strings.TrimSpace(ln)
	if len(ln) == 0 {
		return nil
	}
	if ln[0] == '"' {
		ln = ln[1:]
		if len(ln) > 0 && ln[len(ln)-1] == '"' {
			ln = ln[:len(ln)-1]
		}
	} else if len(ln) > 0 && ln[0] == '{' {
		ln = ln[1:]
		if len(ln) > 0 && ln[len(ln)-1] == '}' {
			ln = ln[:len(ln)-1]
		}
	}
	nln := ""
	incaps := 0
	for _, r := range ln {
		switch r {
		case '\\', '\'':
			continue
		case '{':
			incaps++
		case '}':
			incaps--
		default:
			if incaps <= 0 || key == "A" {
				nln += string(r)
			} else {
				nln += strings.ToUpper(string(r))
			}
		}
	}
	ln = nln
	if key == "A" {
		return strings.Split(ln, " and ")
	}
	if key == "K" {
		return strings.Split(ln, ",")
	}
	return []string{strings.TrimSpace(ln)}
}
