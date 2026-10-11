package calibreweb

import (
	"encoding/xml"
	"testing"
)

// The series index is read from the entry's content, as a real Calibre-Web writes it.
func TestSeriesIndex(t *testing.T) {
	tests := []struct {
		content string // the XHTML of the entry's content
		series  string
		want    float64
		found   bool
	}{
		{"\n    TAGS: Epic, Fantasy<br/>\n    SERIES: Eragon [5]<br/>\n    <p>&lt;div&gt;Résumé [2]&lt;/div&gt;</p>", "Eragon", 5, true},
		{"SERIES: Le Trône de Fer [3]<br/>", "Le Trône de Fer", 3, true},
		{"SERIES: Discworld [2.5]<br/>", "Discworld", 2.5, true},
		{"SERIES: L&#39;Assassin [royal] [12]<br/>", "L'Assassin [royal]", 12, true},
		{"SERIES: Héritage [4]<br/>", "Eragon", 0, false}, // another series
		{"TAGS: Fantasy<br/>", "Eragon", 0, false},
		{"", "Eragon", 0, false},
	}
	for _, test := range tests {
		var item entry
		data := `<entry xmlns="http://www.w3.org/2005/Atom"><title>A book</title><content type="xhtml"><div xmlns="http://www.w3.org/1999/xhtml">` + test.content + `</div></content></entry>`
		if err := xml.Unmarshal([]byte(data), &item); err != nil {
			t.Fatal(err)
		}
		got, found := item.seriesIndex(test.series)
		if got != test.want || found != test.found {
			t.Errorf("%q in %s: %v, %v; want %v, %v", test.content, test.series, got, found, test.want, test.found)
		}
	}
}
