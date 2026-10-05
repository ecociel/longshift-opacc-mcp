package world

import (
	"encoding/json"
	"testing"
	"time"
)

func testWorld(t *testing.T, now time.Time) *World {
	t.Helper()
	w := New(Options{
		Seed:      1,
		Window:    90 * 24 * time.Hour,
		TimeScale: time.Hour,
		Clock:     NewManualClock(now),
	})
	if p := w.Problems(); len(p) > 0 {
		t.Fatalf("fresh world invariants: %v", p)
	}
	return w
}

func TestSeededWorldHasActivityInsideWindow(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	w := testWorld(t, now)

	st := w.Status()
	if st.Counts.Customers < 10 || st.Counts.Articles < 20 {
		t.Fatalf("master data: %+v", st.Counts)
	}
	if st.Counts.Orders == 0 || st.Counts.Invoices == 0 || st.Counts.Payments == 0 {
		t.Fatalf("expected generated activity: %+v", st.Counts)
	}
	if !st.Now.Equal(now) {
		t.Fatalf("clock now %v want %v", st.Now, now)
	}
	if !st.WindowStart.Equal(now.Add(-90 * 24 * time.Hour)) {
		t.Fatalf("window start %v", st.WindowStart)
	}

	orders := w.ListOrders(ListFilter{Limit: 200})
	if orders.Total == 0 {
		t.Fatal("no orders")
	}
	for _, o := range orders.Items {
		if o.OrderedAt.Before(st.WindowStart) || o.OrderedAt.After(now) {
			t.Fatalf("order %s at %v outside window", o.OrderNo, o.OrderedAt)
		}
	}
}

func TestCreateOrderAndPaymentStayConsistent(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	w := testWorld(t, now)

	stock := w.ListStock(ListFilter{Limit: 200})
	var article string
	var before int
	for _, s := range stock.Items {
		if s.Available >= 4 {
			article = s.ArticleNo
			before = s.Available
			break
		}
	}
	if article == "" {
		t.Fatal("no article with stock")
	}

	customers := w.ListCustomers(ListFilter{Limit: 1})
	ord, err := w.CreateSalesOrder(customers.Items[0].CustomerNo, []LineInput{{ArticleNo: article, Qty: 3}})
	if err != nil {
		t.Fatal(err)
	}
	if ord.Status != StatusInvoiced || ord.InvoiceNo == "" || ord.Source != SourceMCP {
		t.Fatalf("%+v", ord)
	}
	if ord.Gross.Rappen <= 0 || ord.VAT.Rappen <= 0 {
		t.Fatalf("totals %+v", ord)
	}

	got, err := w.GetOrder(ord.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if !sameJSON(t, ord, got) {
		t.Fatalf("re-read order changed\n%+v\n%+v", ord, got)
	}

	after, err := w.GetStock(article)
	if err != nil {
		t.Fatal(err)
	}
	if after.Available != before-3 {
		t.Fatalf("stock available %d want %d", after.Available, before-3)
	}

	inv, err := w.GetInvoice(ord.InvoiceNo)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Open.Rappen != inv.Gross.Rappen || inv.Status != StatusOpen {
		t.Fatalf("invoice %+v", inv)
	}

	pay, err := w.PostPayment(inv.InvoiceNo, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if pay.Amount.Rappen != inv.Gross.Rappen {
		t.Fatalf("paid %v of %v", pay.Amount, inv.Gross)
	}
	again, err := w.GetPayment(pay.PaymentNo)
	if err != nil {
		t.Fatal(err)
	}
	if !sameJSON(t, pay, again) {
		t.Fatalf("re-read payment changed")
	}

	inv2, err := w.GetInvoice(inv.InvoiceNo)
	if err != nil {
		t.Fatal(err)
	}
	if inv2.Status != StatusPaid || inv2.Open.Rappen != 0 {
		t.Fatalf("after pay %+v", inv2)
	}
	if _, err := w.PostPayment(inv.InvoiceNo, 1, ""); err == nil {
		t.Fatal("expected reject of payment on closed invoice")
	}
	if p := w.Problems(); len(p) > 0 {
		t.Fatalf("after writes: %v", p)
	}
}

func TestWritesSurviveAdvanceThenDropOutOfWindow(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	w := testWorld(t, now)

	oldest := w.ListOrders(ListFilter{Limit: 200})
	var oldestNo string
	var oldestAt time.Time
	for offset := 0; offset < oldest.Total; offset += 200 {
		page := w.ListOrders(ListFilter{Limit: 200, Offset: offset})
		for _, o := range page.Items {
			if oldestNo == "" || o.OrderedAt.Before(oldestAt) {
				oldestNo = o.OrderNo
				oldestAt = o.OrderedAt
			}
		}
	}
	if oldestNo == "" {
		t.Fatal("no orders")
	}

	stock := w.ListStock(ListFilter{Limit: 1, ArticleNo: ""})
	var article string
	for _, s := range w.ListStock(ListFilter{Limit: 200}).Items {
		if s.Available >= 2 {
			article = s.ArticleNo
			break
		}
	}
	cust := w.ListCustomers(ListFilter{Limit: 1}).Items[0]
	ord, err := w.CreateSalesOrder(cust.CustomerNo, []LineInput{{ArticleNo: article, Qty: 1}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.PostPayment(ord.InvoiceNo, 0, "Überweisung")
	if err != nil {
		t.Fatal(err)
	}

	w.Advance(36 * time.Hour)
	if p := w.Problems(); len(p) > 0 {
		t.Fatalf("after +36h: %v", p)
	}
	got, err := w.GetOrder(ord.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if got.Gross != ord.Gross || got.InvoiceNo != ord.InvoiceNo || !got.OrderedAt.Equal(ord.OrderedAt) {
		t.Fatalf("write mutated after advance: %+v vs %+v", got, ord)
	}
	if w.Now().Sub(now) != 36*time.Hour {
		t.Fatalf("clock %v", w.Now())
	}

	later := w.Status()
	if later.Counts.Orders == 0 {
		t.Fatal("world went empty after 36h")
	}

	w.Advance(90 * 24 * time.Hour)
	if p := w.Problems(); len(p) > 0 {
		t.Fatalf("after +90d: %v", p)
	}
	if _, err := w.GetOrder(ord.OrderNo); err == nil {
		t.Fatal("written order should have dropped after leaving the 3-month window")
	}
	if _, err := w.GetOrder(oldestNo); err == nil {
		t.Fatal("oldest seeded order should have dropped")
	}

	st := w.Status()
	if !st.Now.Equal(now.Add(36*time.Hour + 90*24*time.Hour)) {
		t.Fatalf("clock after roll %v", st.Now)
	}
	if st.Counts.Orders == 0 {
		t.Fatal("rolling world should keep generating new orders")
	}
	orders := w.ListOrders(ListFilter{Limit: 200})
	for _, o := range orders.Items {
		if o.OrderedAt.Before(st.WindowStart) {
			t.Fatalf("order %s still older than window", o.OrderNo)
		}
	}
}

func TestRejectOversellAndOverpay(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	w := testWorld(t, now)
	cust := w.ListCustomers(ListFilter{Limit: 1}).Items[0]

	if _, err := w.CreateSalesOrder(cust.CustomerNo, []LineInput{{ArticleNo: "A-20001", Qty: 1_000_000}}); err == nil {
		t.Fatal("expected insufficient stock")
	}
	if _, err := w.CreateSalesOrder("K-99999", []LineInput{{ArticleNo: "A-20001", Qty: 1}}); err == nil {
		t.Fatal("expected missing customer")
	}

	invs := w.ListOpenItems(ListFilter{Limit: 1})
	if invs.Total == 0 {
		t.Fatal("need an open item")
	}
	item := invs.Items[0]
	tooMuch := float64(item.Open.Rappen)/100 + 50
	if _, err := w.PostPayment(item.InvoiceNo, tooMuch, ""); err == nil {
		t.Fatal("expected overpay reject")
	}
	if p := w.Problems(); len(p) > 0 {
		t.Fatalf("%v", p)
	}
}

func TestScaledClockMapping(t *testing.T) {
	sim0 := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	wall0 := time.Now().Add(-2 * time.Second)
	c := NewScaledClockAt(sim0, wall0, time.Hour)
	got := c.Now()
	elapsed := time.Since(wall0).Seconds()
	want := sim0.Add(time.Duration(elapsed * float64(time.Hour)))
	if d := got.Sub(want); d > 50*time.Millisecond || d < -50*time.Millisecond {
		t.Fatalf("scaled now %v want ~%v (delta %v)", got, want, d)
	}
	if got.Before(sim0.Add(90*time.Minute)) || got.After(sim0.Add(3*time.Hour)) {
		t.Fatalf("2 wall seconds should be about 2 simulated hours, got %v (origin %v)", got, sim0)
	}
}

func TestPartialPaymentOpenItem(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	w := testWorld(t, now)
	var article string
	for _, s := range w.ListStock(ListFilter{Limit: 200}).Items {
		if s.Available >= 1 {
			article = s.ArticleNo
			break
		}
	}
	cust := w.ListCustomers(ListFilter{Limit: 1}).Items[0]
	ord, err := w.CreateSalesOrder(cust.CustomerNo, []LineInput{{ArticleNo: article, Qty: 1}})
	if err != nil {
		t.Fatal(err)
	}
	inv, err := w.GetInvoice(ord.InvoiceNo)
	if err != nil {
		t.Fatal(err)
	}
	half := float64(inv.Open.Rappen/2) / 100
	if _, err := w.PostPayment(inv.InvoiceNo, half, "Überweisung"); err != nil {
		t.Fatal(err)
	}
	inv2, err := w.GetInvoice(inv.InvoiceNo)
	if err != nil {
		t.Fatal(err)
	}
	if inv2.Status != StatusPartial || inv2.Open.Rappen == 0 || inv2.Paid.Rappen == 0 {
		t.Fatalf("%+v", inv2)
	}
	if inv2.Paid.Rappen+inv2.Open.Rappen != inv2.Gross.Rappen {
		t.Fatalf("paid+open != gross: %+v", inv2)
	}
	if p := w.Problems(); len(p) > 0 {
		t.Fatalf("%v", p)
	}
}

func sameJSON(t *testing.T, a, b any) bool {
	t.Helper()
	x, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	y, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return string(x) == string(y)
}
