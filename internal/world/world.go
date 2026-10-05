package world

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	StatusOpen     = "open"
	StatusInvoiced = "invoiced"
	StatusPartial  = "partial"
	StatusPaid     = "paid"

	SourceGenerator = "generator"
	SourceMCP       = "mcp"

	WarehouseCode = "LG-ROTH"
	WarehouseName = "Hauptlager Rothenburg"
	StdVAT        = 81
)

type Options struct {
	Seed      int64
	Window    time.Duration
	TimeScale time.Duration
	Clock     Clock
	Now       time.Time
}

type articleRec struct {
	no, name, nameDE, unit, size, barcode, group string
	listPrice                                    Money
	vat                                          int
}

type stockRec struct {
	onHand   int
	reserved int
}

type orderRec struct {
	no, customerNo, customerName, status, invoiceNo, source string
	orderedAt                                               time.Time
	deliveredAt                                             *time.Time
	lines                                                   []OrderLine
	net, vat, gross                                         Money
}

type invoiceRec struct {
	no, orderNo, customerNo, customerName, qr string
	invoicedAt, dueAt                         time.Time
	lines                                     []OrderLine
	net, vat, gross, paid                     Money
}

type paymentRec struct {
	no, invoiceNo, customerNo, method string
	paidAt                            time.Time
	amount                            Money
}

type World struct {
	mu        sync.Mutex
	clock     Clock
	window    time.Duration
	seed      int64
	timeScale time.Duration
	lastTick  time.Time

	customers map[string]Customer
	articles  map[string]*articleRec
	stock     map[string]*stockRec
	orders    map[string]*orderRec
	invoices  map[string]*invoiceRec
	payments  map[string]*paymentRec

	customerNos []string
	articleNos  []string

	orderSeq   int
	invoiceSeq int
	paymentSeq int
}

func New(opt Options) *World {
	if opt.Window <= 0 {
		opt.Window = 90 * 24 * time.Hour
	}
	if opt.Seed == 0 {
		opt.Seed = 1
	}
	if opt.TimeScale <= 0 {
		opt.TimeScale = time.Hour
	}
	clock := opt.Clock
	if clock == nil {
		origin := opt.Now
		if origin.IsZero() {
			origin = time.Now()
		}
		clock = NewScaledClock(origin, opt.TimeScale)
	}

	w := &World{
		clock:     clock,
		window:    opt.Window,
		seed:      opt.Seed,
		timeScale: opt.TimeScale,
		customers: make(map[string]Customer),
		articles:  make(map[string]*articleRec),
		stock:     make(map[string]*stockRec),
		orders:    make(map[string]*orderRec),
		invoices:  make(map[string]*invoiceRec),
		payments:  make(map[string]*paymentRec),
	}
	w.seedMaster()
	now := w.clock.Now()
	w.lastTick = now.Add(-w.window)
	w.catchUpLocked(now)
	return w
}

func (w *World) Run(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = time.Second
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.CatchUp()
		}
	}
}

func (w *World) CatchUp() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.catchUpLocked(w.clock.Now())
}

func (w *World) Advance(d time.Duration) {
	w.clock.Advance(d)
	w.CatchUp()
}

func (w *World) Now() time.Time {
	return w.clock.Now()
}

func (w *World) catchUpLocked(now time.Time) {
	t := w.lastTick
	for {
		next := t.Add(time.Hour)
		if next.After(now) {
			break
		}
		w.generateHour(next)
		t = next
	}
	w.lastTick = t
	w.pruneLocked(now)
}

func (w *World) pruneLocked(now time.Time) {
	start := now.Add(-w.window)
	for no, o := range w.orders {
		if !o.orderedAt.Before(start) {
			continue
		}
		if o.status == StatusOpen {
			for _, ln := range o.lines {
				if st := w.stock[ln.ArticleNo]; st != nil {
					st.reserved -= ln.Qty
					if st.reserved < 0 {
						st.reserved = 0
					}
				}
			}
		}
		delete(w.orders, no)
	}
	for no, inv := range w.invoices {
		if inv.invoicedAt.Before(start) {
			delete(w.invoices, no)
		}
	}
	for no, p := range w.payments {
		if p.paidAt.Before(start) {
			delete(w.payments, no)
		}
	}
}

func (w *World) windowStart(now time.Time) time.Time {
	return now.Add(-w.window)
}

func (w *World) Status() Status {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock.Now()
	open := 0
	for _, inv := range w.invoices {
		if inv.gross-inv.paid > 0 {
			open++
		}
	}
	return Status{
		Now:         now,
		WindowStart: w.windowStart(now),
		Window:      formatWindow(w.window),
		TimeScale:   formatScale(w.timeScale),
		Seed:        w.seed,
		Counts: StatusCounts{
			Customers: len(w.customers),
			Articles:  len(w.articles),
			Orders:    len(w.orders),
			Invoices:  len(w.invoices),
			Payments:  len(w.payments),
			OpenItems: open,
		},
	}
}

func formatWindow(d time.Duration) string {
	days := int(d / (24 * time.Hour))
	if time.Duration(days)*24*time.Hour == d {
		return fmt.Sprintf("%dd", days)
	}
	return d.String()
}

func formatScale(d time.Duration) string {
	return fmt.Sprintf("1s wall = %s simulated", formatWindow(d))
}

func (w *World) ListCustomers(f ListFilter) ListResult[Customer] {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock.Now()
	items := make([]Customer, 0, len(w.customers))
	q := strings.ToLower(strings.TrimSpace(f.Query))
	for _, c := range w.customers {
		if q != "" && !strings.Contains(strings.ToLower(c.Name+c.CustomerNo+c.City), q) {
			continue
		}
		items = append(items, c)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CustomerNo < items[j].CustomerNo })
	return page(now, w.windowStart(now), f, items)
}

func (w *World) GetCustomer(no string) (Customer, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c, ok := w.customers[strings.TrimSpace(no)]
	if !ok {
		return Customer{}, fmt.Errorf("customer %s not found", no)
	}
	return c, nil
}

func (w *World) ListArticles(f ListFilter) ListResult[Article] {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock.Now()
	items := make([]Article, 0, len(w.articles))
	q := strings.ToLower(strings.TrimSpace(f.Query))
	for _, no := range w.articleNos {
		a := w.publicArticle(no)
		if q != "" && !strings.Contains(strings.ToLower(a.Name+a.NameDE+a.ArticleNo+a.ProductGroup), q) {
			continue
		}
		items = append(items, a)
	}
	return page(now, w.windowStart(now), f, items)
}

func (w *World) GetArticle(no string) (Article, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.articles[strings.TrimSpace(no)]; !ok {
		return Article{}, fmt.Errorf("article %s not found", no)
	}
	return w.publicArticle(no), nil
}

func (w *World) ListOrders(f ListFilter) ListResult[Order] {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock.Now()
	items := make([]Order, 0, len(w.orders))
	for _, o := range w.orders {
		if f.CustomerNo != "" && o.customerNo != f.CustomerNo {
			continue
		}
		if f.Status != "" && o.status != f.Status {
			continue
		}
		if f.ArticleNo != "" && !orderHasArticle(o, f.ArticleNo) {
			continue
		}
		items = append(items, w.publicOrder(o))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].OrderedAt.Equal(items[j].OrderedAt) {
			return items[i].OrderNo > items[j].OrderNo
		}
		return items[i].OrderedAt.After(items[j].OrderedAt)
	})
	return page(now, w.windowStart(now), f, items)
}

func (w *World) GetOrder(no string) (Order, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	o, ok := w.orders[strings.TrimSpace(no)]
	if !ok {
		return Order{}, fmt.Errorf("order %s not found", no)
	}
	return w.publicOrder(o), nil
}

func (w *World) ListInvoices(f ListFilter) ListResult[Invoice] {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock.Now()
	items := make([]Invoice, 0, len(w.invoices))
	for _, inv := range w.invoices {
		pub := w.publicInvoice(inv)
		if f.CustomerNo != "" && pub.CustomerNo != f.CustomerNo {
			continue
		}
		if f.Status != "" && pub.Status != f.Status {
			continue
		}
		if f.ArticleNo != "" && !invoiceHasArticle(inv, f.ArticleNo) {
			continue
		}
		items = append(items, pub)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].InvoicedAt.Equal(items[j].InvoicedAt) {
			return items[i].InvoiceNo > items[j].InvoiceNo
		}
		return items[i].InvoicedAt.After(items[j].InvoicedAt)
	})
	return page(now, w.windowStart(now), f, items)
}

func (w *World) GetInvoice(no string) (Invoice, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	inv, ok := w.invoices[strings.TrimSpace(no)]
	if !ok {
		return Invoice{}, fmt.Errorf("invoice %s not found", no)
	}
	return w.publicInvoice(inv), nil
}

func (w *World) ListPayments(f ListFilter) ListResult[Payment] {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock.Now()
	items := make([]Payment, 0, len(w.payments))
	for _, p := range w.payments {
		if f.CustomerNo != "" && p.customerNo != f.CustomerNo {
			continue
		}
		items = append(items, w.publicPayment(p))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].PaidAt.Equal(items[j].PaidAt) {
			return items[i].PaymentNo > items[j].PaymentNo
		}
		return items[i].PaidAt.After(items[j].PaidAt)
	})
	return page(now, w.windowStart(now), f, items)
}

func (w *World) GetPayment(no string) (Payment, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.payments[strings.TrimSpace(no)]
	if !ok {
		return Payment{}, fmt.Errorf("payment %s not found", no)
	}
	return w.publicPayment(p), nil
}

func (w *World) ListStock(f ListFilter) ListResult[Stock] {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock.Now()
	items := make([]Stock, 0, len(w.articleNos))
	q := strings.ToLower(strings.TrimSpace(f.Query))
	for _, no := range w.articleNos {
		if f.ArticleNo != "" && no != f.ArticleNo {
			continue
		}
		s := w.publicStock(no)
		if q != "" && !strings.Contains(strings.ToLower(s.Name+s.ArticleNo), q) {
			continue
		}
		items = append(items, s)
	}
	return page(now, w.windowStart(now), f, items)
}

func (w *World) GetStock(articleNo string) (Stock, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	no := strings.TrimSpace(articleNo)
	if _, ok := w.articles[no]; !ok {
		return Stock{}, fmt.Errorf("article %s not found", articleNo)
	}
	return w.publicStock(no), nil
}

func (w *World) ListOpenItems(f ListFilter) ListResult[OpenItem] {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock.Now()
	items := make([]OpenItem, 0)
	for _, inv := range w.invoices {
		item := w.publicOpenItem(inv)
		if item.Open.Rappen <= 0 {
			continue
		}
		if f.CustomerNo != "" && item.CustomerNo != f.CustomerNo {
			continue
		}
		if f.Status != "" && item.Status != f.Status {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].DueAt.Before(items[j].DueAt) })
	return page(now, w.windowStart(now), f, items)
}

func (w *World) CreateSalesOrder(customerNo string, lines []LineInput) (Order, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock.Now()
	w.catchUpLocked(now)

	c, ok := w.customers[strings.TrimSpace(customerNo)]
	if !ok {
		return Order{}, fmt.Errorf("customer %s not found", customerNo)
	}
	if len(lines) == 0 {
		return Order{}, fmt.Errorf("order needs at least one line")
	}

	built, err := w.buildLines(lines)
	if err != nil {
		return Order{}, err
	}
	for _, ln := range built.lines {
		st := w.stock[ln.ArticleNo]
		if st.onHand-st.reserved < ln.Qty {
			return Order{}, fmt.Errorf("insufficient stock for %s: available %d, requested %d", ln.ArticleNo, st.onHand-st.reserved, ln.Qty)
		}
	}
	for _, ln := range built.lines {
		w.stock[ln.ArticleNo].onHand -= ln.Qty
	}

	o := w.addOrder(c, built, now, StatusInvoiced, SourceMCP)
	delivered := now
	o.deliveredAt = &delivered
	inv := w.addInvoice(c, o, now)
	o.invoiceNo = inv.no
	return w.publicOrder(o), nil
}

func (w *World) PostPayment(invoiceNo string, amountCHF float64, method string) (Payment, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.clock.Now()
	w.catchUpLocked(now)

	inv, ok := w.invoices[strings.TrimSpace(invoiceNo)]
	if !ok {
		return Payment{}, fmt.Errorf("invoice %s not found", invoiceNo)
	}
	open := inv.gross - inv.paid
	if open <= 0 {
		return Payment{}, fmt.Errorf("invoice %s is already paid", invoiceNo)
	}

	var amount Money
	if amountCHF <= 0 {
		amount = open
	} else {
		var err error
		amount, err = MoneyFromCHF(amountCHF)
		if err != nil {
			return Payment{}, err
		}
		if amount <= 0 {
			return Payment{}, fmt.Errorf("amount must be positive")
		}
	}
	if amount > open {
		return Payment{}, fmt.Errorf("payment %s CHF exceeds open item %s CHF", amount.CHFString(), open.CHFString())
	}
	method = strings.TrimSpace(method)
	if method == "" {
		method = "QR-Rechnung"
	}

	inv.paid += amount
	w.paymentSeq++
	p := &paymentRec{
		no:         fmt.Sprintf("ZA-%d-%05d", now.Year(), w.paymentSeq),
		invoiceNo:  inv.no,
		customerNo: inv.customerNo,
		method:     method,
		paidAt:     now,
		amount:     amount,
	}
	w.payments[p.no] = p
	return w.publicPayment(p), nil
}

type builtLines struct {
	lines           []OrderLine
	net, vat, gross Money
}

func (w *World) buildLines(in []LineInput) (builtLines, error) {
	out := builtLines{}
	for i, raw := range in {
		if raw.Qty <= 0 {
			return builtLines{}, fmt.Errorf("line %d: qty must be > 0", i+1)
		}
		a, ok := w.articles[strings.TrimSpace(raw.ArticleNo)]
		if !ok {
			return builtLines{}, fmt.Errorf("article %s not found", raw.ArticleNo)
		}
		net := a.listPrice * Money(raw.Qty)
		vat := VAT(net, a.vat)
		ln := OrderLine{
			LineNo:      i + 1,
			ArticleNo:   a.no,
			Name:        a.name,
			Qty:         raw.Qty,
			Unit:        a.unit,
			UnitPrice:   View(a.listPrice),
			Net:         View(net),
			VAT:         View(vat),
			Gross:       View(net + vat),
			VATPermille: a.vat,
		}
		out.lines = append(out.lines, ln)
		out.net += net
		out.vat += vat
		out.gross += net + vat
	}
	return out, nil
}

func (w *World) addOrder(c Customer, built builtLines, at time.Time, status, source string) *orderRec {
	w.orderSeq++
	o := &orderRec{
		no:           fmt.Sprintf("AU-%d-%05d", at.Year(), w.orderSeq),
		customerNo:   c.CustomerNo,
		customerName: c.Name,
		status:       status,
		orderedAt:    at,
		lines:        built.lines,
		net:          built.net,
		vat:          built.vat,
		gross:        built.gross,
		source:       source,
	}
	w.orders[o.no] = o
	return o
}

func (w *World) addInvoice(c Customer, o *orderRec, at time.Time) *invoiceRec {
	w.invoiceSeq++
	inv := &invoiceRec{
		no:           fmt.Sprintf("RE-%d-%05d", at.Year(), w.invoiceSeq),
		orderNo:      o.no,
		customerNo:   c.CustomerNo,
		customerName: c.Name,
		qr:           qrReference(at.Year(), w.invoiceSeq),
		invoicedAt:   at,
		dueAt:        at.Add(time.Duration(c.PaymentTermsDays) * 24 * time.Hour),
		lines:        append([]OrderLine(nil), o.lines...),
		net:          o.net,
		vat:          o.vat,
		gross:        o.gross,
	}
	w.invoices[inv.no] = inv
	return inv
}

func (w *World) publicArticle(no string) Article {
	a := w.articles[no]
	return Article{
		ArticleNo:    a.no,
		Name:         a.name,
		NameDE:       a.nameDE,
		Unit:         a.unit,
		Size:         a.size,
		Barcode:      a.barcode,
		ListPrice:    View(a.listPrice),
		VATPermille:  a.vat,
		ProductGroup: a.group,
	}
}

func (w *World) publicOrder(o *orderRec) Order {
	return Order{
		OrderNo:      o.no,
		CustomerNo:   o.customerNo,
		CustomerName: o.customerName,
		Status:       o.status,
		OrderedAt:    o.orderedAt,
		DeliveredAt:  o.deliveredAt,
		InvoiceNo:    o.invoiceNo,
		Lines:        append([]OrderLine(nil), o.lines...),
		Net:          View(o.net),
		VAT:          View(o.vat),
		Gross:        View(o.gross),
		Currency:     "CHF",
		Source:       o.source,
	}
}

func (w *World) publicInvoice(inv *invoiceRec) Invoice {
	open := inv.gross - inv.paid
	return Invoice{
		InvoiceNo:    inv.no,
		OrderNo:      inv.orderNo,
		CustomerNo:   inv.customerNo,
		CustomerName: inv.customerName,
		Status:       invoiceStatus(open, inv.paid),
		InvoicedAt:   inv.invoicedAt,
		DueAt:        inv.dueAt,
		QRReference:  inv.qr,
		Lines:        append([]OrderLine(nil), inv.lines...),
		Net:          View(inv.net),
		VAT:          View(inv.vat),
		Gross:        View(inv.gross),
		Paid:         View(inv.paid),
		Open:         View(open),
		Currency:     "CHF",
	}
}

func (w *World) publicPayment(p *paymentRec) Payment {
	return Payment{
		PaymentNo:  p.no,
		InvoiceNo:  p.invoiceNo,
		CustomerNo: p.customerNo,
		PaidAt:     p.paidAt,
		Amount:     View(p.amount),
		Method:     p.method,
		Currency:   "CHF",
	}
}

func (w *World) publicStock(no string) Stock {
	a := w.articles[no]
	st := w.stock[no]
	return Stock{
		ArticleNo: no,
		Name:      a.name,
		Warehouse: WarehouseName,
		Lagerort:  WarehouseCode,
		Unit:      a.unit,
		OnHand:    st.onHand,
		Reserved:  st.reserved,
		Available: st.onHand - st.reserved,
	}
}

func (w *World) publicOpenItem(inv *invoiceRec) OpenItem {
	open := inv.gross - inv.paid
	return OpenItem{
		InvoiceNo:    inv.no,
		CustomerNo:   inv.customerNo,
		CustomerName: inv.customerName,
		InvoicedAt:   inv.invoicedAt,
		DueAt:        inv.dueAt,
		Gross:        View(inv.gross),
		Paid:         View(inv.paid),
		Open:         View(open),
		Status:       invoiceStatus(open, inv.paid),
		Currency:     "CHF",
	}
}

func invoiceStatus(open, paid Money) string {
	if open <= 0 {
		return StatusPaid
	}
	if paid > 0 {
		return StatusPartial
	}
	return StatusOpen
}

func orderHasArticle(o *orderRec, no string) bool {
	for _, ln := range o.lines {
		if ln.ArticleNo == no {
			return true
		}
	}
	return false
}

func invoiceHasArticle(inv *invoiceRec, no string) bool {
	for _, ln := range inv.lines {
		if ln.ArticleNo == no {
			return true
		}
	}
	return false
}

func page[T any](now, start time.Time, f ListFilter, items []T) ListResult[T] {
	limit, offset := f.Limit, f.Offset
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	total := len(items)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	out := items[offset:end]
	if out == nil {
		out = []T{}
	}
	return ListResult[T]{
		Now:         now,
		WindowStart: start,
		Total:       total,
		Limit:       limit,
		Offset:      offset,
		Items:       out,
	}
}

func (w *World) rngFor(t time.Time, kind string) *rand.Rand {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%d|%s|%s", w.seed, t.UTC().Format(time.RFC3339), kind)
	v := h.Sum64()
	return rand.New(rand.NewPCG(v, v^0x9e3779b97f4a7c15))
}

func qrReference(year, seq int) string {
	return fmt.Sprintf("%04d%023d", year, seq)
}

func (w *World) Problems() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.problemsLocked()
}

func (w *World) problemsLocked() []string {
	var problems []string
	now := w.clock.Now()
	start := w.windowStart(now)
	reserved := map[string]int{}

	for no, st := range w.stock {
		if st.onHand < 0 {
			problems = append(problems, fmt.Sprintf("stock %s on_hand %d", no, st.onHand))
		}
		if st.reserved < 0 {
			problems = append(problems, fmt.Sprintf("stock %s reserved %d", no, st.reserved))
		}
		if st.onHand-st.reserved < 0 {
			problems = append(problems, fmt.Sprintf("stock %s available %d", no, st.onHand-st.reserved))
		}
	}

	for _, o := range w.orders {
		if o.orderedAt.Before(start) || o.orderedAt.After(now) {
			problems = append(problems, fmt.Sprintf("order %s outside window (%s)", o.no, o.orderedAt))
		}
		var net, vat, gross Money
		for _, ln := range o.lines {
			if _, ok := w.articles[ln.ArticleNo]; !ok {
				problems = append(problems, fmt.Sprintf("order %s unknown article %s", o.no, ln.ArticleNo))
			}
			if ln.Qty <= 0 {
				problems = append(problems, fmt.Sprintf("order %s line qty %d", o.no, ln.Qty))
			}
			net += Money(ln.Net.Rappen)
			vat += Money(ln.VAT.Rappen)
			gross += Money(ln.Gross.Rappen)
			if o.status == StatusOpen {
				reserved[ln.ArticleNo] += ln.Qty
			}
		}
		if net != o.net || vat != o.vat || gross != o.gross {
			problems = append(problems, fmt.Sprintf("order %s totals mismatch", o.no))
		}
		if o.status == StatusInvoiced && o.invoiceNo != "" {
			if _, ok := w.invoices[o.invoiceNo]; !ok {
				problems = append(problems, fmt.Sprintf("order %s missing invoice %s", o.no, o.invoiceNo))
			}
		}
	}

	for no, want := range reserved {
		if w.stock[no] == nil || w.stock[no].reserved != want {
			got := 0
			if w.stock[no] != nil {
				got = w.stock[no].reserved
			}
			problems = append(problems, fmt.Sprintf("reserved %s want %d got %d", no, want, got))
		}
	}
	for no, st := range w.stock {
		if _, ok := reserved[no]; !ok && st.reserved != 0 {
			problems = append(problems, fmt.Sprintf("reserved %s leftover %d", no, st.reserved))
		}
	}

	paidByInv := map[string]Money{}
	for _, p := range w.payments {
		if p.paidAt.Before(start) || p.paidAt.After(now) {
			problems = append(problems, fmt.Sprintf("payment %s outside window", p.no))
		}
		if p.amount <= 0 {
			problems = append(problems, fmt.Sprintf("payment %s non-positive", p.no))
		}
		paidByInv[p.invoiceNo] += p.amount
	}

	for _, inv := range w.invoices {
		if inv.invoicedAt.Before(start) || inv.invoicedAt.After(now) {
			problems = append(problems, fmt.Sprintf("invoice %s outside window", inv.no))
		}
		if inv.paid < 0 || inv.paid > inv.gross {
			problems = append(problems, fmt.Sprintf("invoice %s paid %s of %s", inv.no, inv.paid.CHFString(), inv.gross.CHFString()))
		}
		if paid := paidByInv[inv.no]; paid != inv.paid {
			problems = append(problems, fmt.Sprintf("invoice %s paid ledger %s payments %s", inv.no, inv.paid.CHFString(), paid.CHFString()))
		}
		var net, vat, gross Money
		for _, ln := range inv.lines {
			net += Money(ln.Net.Rappen)
			vat += Money(ln.VAT.Rappen)
			gross += Money(ln.Gross.Rappen)
		}
		if net != inv.net || vat != inv.vat || gross != inv.gross {
			problems = append(problems, fmt.Sprintf("invoice %s totals mismatch", inv.no))
		}
	}

	sort.Strings(problems)
	return problems
}
