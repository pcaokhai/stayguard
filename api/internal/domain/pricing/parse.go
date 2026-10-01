package pricing

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"

	"github.com/pcaokhai/stayguard/api/internal/domain/money"
)

// ParseRatePlan validates raw JSON against the rate plan contract and reports every
// problem at once. The returned error is a *ValidationError. Callers must bound the body
// size (http.MaxBytesReader) because the domain does not.
func ParseRatePlan(raw []byte) (RatePlan, error) {
	doc, err := decode(raw)
	if err != nil {
		return RatePlan{}, &ValidationError{Errors: []FieldError{{Path: "", Code: CodeInvalidJSON}}}
	}
	var c collector
	plan := c.plan(doc)
	if len(c.errs) > 0 {
		return RatePlan{}, &ValidationError{Errors: c.sorted()}
	}
	return plan, nil
}

// decode keeps numbers as text so fractions stay distinguishable from integers.
func decode(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("trailing data")
	}
	return doc, nil
}

func (c *collector) plan(doc any) RatePlan {
	m := c.object("", doc, "version", "currency", "graceMinutes", "hourly", "overnight", "daily")
	p := RatePlan{
		Version:      c.integer("", m, "version", MinVersion, math.MaxInt64),
		GraceMinutes: int(c.integer("", m, "graceMinutes", MinGraceMinutes, MaxGraceMinutes)),
	}
	if cur, present := m["currency"]; present {
		if cur == CurrencyVND {
			p.Currency = CurrencyVND
		} else {
			c.add("currency", CodeConst)
		}
	}
	if h, present := m["hourly"]; present {
		hm := c.object("hourly", h, "firstHour", "extraHour")
		p.Hourly = Hourly{FirstHour: c.money("hourly", hm, "firstHour"), ExtraHour: c.money("hourly", hm, "extraHour")}
	}
	if o, present := m["overnight"]; present {
		p.Overnight = c.window("overnight", o)
	}
	if d, present := m["daily"]; present {
		p.Daily = c.window("daily", d)
	}
	return p
}

func (c *collector) money(path string, m map[string]any, key string) money.Vnd {
	return money.Vnd(c.integer(path, m, key, 0, math.MaxInt64))
}

func (c *collector) window(path string, v any) Window {
	m := c.object(path, v, "price", "windowStart", "windowEnd")
	return Window{
		Price: c.money(path, m, "price"),
		Start: c.clock(path, m, "windowStart"),
		End:   c.clock(path, m, "windowEnd"),
	}
}
