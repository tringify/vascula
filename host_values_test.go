package vascula

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

// price is a host decimal kept as stored and printed for a locale.
type price struct{ text, shown string }

func (p price) Float64() (float64, error)    { return strconv.ParseFloat(p.text, 64) }
func (p price) String() string               { return p.shown }
func (p price) MarshalJSON() ([]byte, error) { return []byte(p.text), nil }

// day is a host date printed for a locale.
type day struct{ t time.Time }

func (d day) Time() time.Time { return d.t }
func (d day) String() string  { return d.t.Format("2 January 2006") }

func renderHost(t *testing.T, src string, settings map[string]interface{}) string {
	t.Helper()
	tpl, err := Compile(src)
	if err != nil {
		t.Fatal(err)
	}
	out, err := tpl.Render(Options{Settings: settings})
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return out
}

func TestHostNumbersPrintTheirTextAndComputeAsNumbers(t *testing.T) {
	s := map[string]interface{}{"p": price{"19.90", "19,90"}, "q": price{"5", "5"}, "n": json.Number("2.50")}
	for src, want := range map[string]string{
		`{{ settings.p }}`:                                  "19,90",
		`{{ settings.n }}`:                                  "2.50",
		`{{ settings.p | plus: 0.1 }}`:                      "20",
		`{{ settings.p | times: settings.q }}`:              "99.5",
		`{% if settings.p > 10 %}big{% endif %}`:            "big",
		`{% if settings.p > settings.q %}more{% endif %}`:   "more",
		`{% if settings.p == 19.9 %}same{% endif %}`:        "same",
		`{{ settings.p | json }}`:                           "19.90",
		`{{ settings.p | round: 0 }}`:                       "20",
		`{% assign l = settings.p | append: " €" %}{{ l }}`: "19,90 €",
	} {
		if got := renderHost(t, src, s); got != want {
			t.Errorf("%s = %q, want %q", src, got, want)
		}
	}
}

func TestHostDatesPrintTheirTextAndFormatAsDates(t *testing.T) {
	when := time.Date(2026, 10, 12, 18, 30, 0, 0, time.FixedZone("IST", 5*3600+1800))
	s := map[string]interface{}{"d": day{when}, "e": day{when.Add(48 * time.Hour)}}
	for src, want := range map[string]string{
		`{{ settings.d }}`:                                 "12 October 2026",
		`{{ settings.d | date: "%Y-%m-%d %H:%M" }}`:        "2026-10-12 18:30",
		`{% if settings.e > settings.d %}later{% endif %}`: "later",
	} {
		if got := renderHost(t, src, s); got != want {
			t.Errorf("%s = %q, want %q", src, got, want)
		}
	}
}

func TestHostDatesCompareByInstant(t *testing.T) {
	when := time.Date(2026, 10, 12, 18, 30, 0, 0, time.FixedZone("IST", 5*3600+1800))
	s := map[string]interface{}{
		"d":    day{when},
		"min":  day{when.Add(time.Minute)},
		"same": day{when.UTC()},
		"day":  day{time.Date(2026, 10, 12, 0, 0, 0, 0, time.FixedZone("IST", 5*3600+1800))},
	}
	for src, want := range map[string]string{
		// Same display text, different moments.
		`{% if settings.d == settings.min %}eq{% else %}ne{% endif %}`: "ne",
		`{% if settings.d < settings.min %}lt{% endif %}`:              "lt",
		// Same moment, different clocks.
		`{% if settings.d == settings.same %}eq{% endif %}`: "eq",
		// ISO strings on either side.
		`{% if settings.d > "2026-10-01" %}after{% endif %}`:                "after",
		`{% if "2026-12-01" > settings.d %}before{% endif %}`:               "before",
		`{% if settings.d == "2026-10-12T13:00:00Z" %}eq{% endif %}`:        "eq",
		`{% if settings.d == "2026-10-12T18:30:00" %}eq{% endif %}`:         "eq",
		`{% if settings.d >= "2026-10-12T18:30" %}ge{% endif %}`:            "ge",
		`{% if settings.day == "2026-10-12" %}eq{% endif %}`:                "eq",
		`{% if settings.day != "2026-10-13" %}ne{% endif %}`:                "ne",
		`{% if settings.d == "12 October 2026" %}eq{% else %}ne{% endif %}`: "ne",
	} {
		if got := renderHost(t, src, s); got != want {
			t.Errorf("%s = %q, want %q", src, got, want)
		}
	}
}
