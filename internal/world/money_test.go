package world

import "testing"

func TestVATAndCHFRoundTrip(t *testing.T) {
	net := Money(10000)
	vat := VAT(net, StdVAT)
	if vat != 810 {
		t.Fatalf("8.1%% of 100.00 is 8.10, got %s", vat.CHFString())
	}
	if View(net+vat).CHF != "108.10" {
		t.Fatalf("gross %s", View(net+vat).CHF)
	}
	got, err := MoneyFromCHF(12.345)
	if err != nil || got != 1235 {
		t.Fatalf("round %v %v", got, err)
	}
	if Money(-101).CHFString() != "-1.01" {
		t.Fatalf("neg %s", Money(-101).CHFString())
	}
}
