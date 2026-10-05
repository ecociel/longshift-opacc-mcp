package world

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Money int64

func (m Money) Rappen() int64 { return int64(m) }

func (m Money) CHFString() string {
	neg := ""
	v := m
	if v < 0 {
		neg = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", neg, v/100, v%100)
}

func View(m Money) MoneyView {
	return MoneyView{Rappen: m.Rappen(), CHF: m.CHFString(), Currency: "CHF"}
}

func MoneyFromCHF(f float64) (Money, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("invalid amount")
	}
	return Money(math.Round(f * 100)), nil
}

func ParseCHF(s string) (Money, error) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, err
	}
	return MoneyFromCHF(f)
}

func VAT(net Money, permille int) Money {
	if net < 0 {
		return -VAT(-net, permille)
	}
	return (net*Money(permille) + 500) / 1000
}
