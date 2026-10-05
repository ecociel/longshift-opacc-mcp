package world

import "time"

type MoneyView struct {
	Rappen   int64  `json:"rappen"`
	CHF      string `json:"chf"`
	Currency string `json:"currency"`
}

type Customer struct {
	CustomerNo       string `json:"customer_no"`
	Name             string `json:"name"`
	UID              string `json:"uid_mwst"`
	Street           string `json:"street"`
	PLZ              string `json:"plz"`
	City             string `json:"city"`
	Kanton           string `json:"kanton"`
	Country          string `json:"country"`
	Language         string `json:"language"`
	PaymentTermsDays int    `json:"payment_terms_days"`
}

type Article struct {
	ArticleNo    string    `json:"article_no"`
	Name         string    `json:"name"`
	NameDE       string    `json:"name_de"`
	Unit         string    `json:"unit"`
	Size         string    `json:"size,omitempty"`
	Barcode      string    `json:"barcode"`
	ListPrice    MoneyView `json:"list_price"`
	VATPermille  int       `json:"vat_permille"`
	ProductGroup string    `json:"product_group"`
}

type OrderLine struct {
	LineNo      int       `json:"line_no"`
	ArticleNo   string    `json:"article_no"`
	Name        string    `json:"name"`
	Qty         int       `json:"qty"`
	Unit        string    `json:"unit"`
	UnitPrice   MoneyView `json:"unit_price"`
	Net         MoneyView `json:"net"`
	VAT         MoneyView `json:"vat"`
	Gross       MoneyView `json:"gross"`
	VATPermille int       `json:"vat_permille"`
}

type Order struct {
	OrderNo      string     `json:"order_no"`
	CustomerNo   string     `json:"customer_no"`
	CustomerName string     `json:"customer_name"`
	Status       string     `json:"status"`
	OrderedAt    time.Time  `json:"ordered_at"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
	InvoiceNo    string     `json:"invoice_no,omitempty"`
	Lines        []OrderLine `json:"lines"`
	Net          MoneyView  `json:"net"`
	VAT          MoneyView  `json:"vat"`
	Gross        MoneyView  `json:"gross"`
	Currency     string     `json:"currency"`
	Source       string     `json:"source"`
}

type Invoice struct {
	InvoiceNo    string      `json:"invoice_no"`
	OrderNo      string      `json:"order_no"`
	CustomerNo   string      `json:"customer_no"`
	CustomerName string      `json:"customer_name"`
	Status       string      `json:"status"`
	InvoicedAt   time.Time   `json:"invoiced_at"`
	DueAt        time.Time   `json:"due_at"`
	QRReference  string      `json:"qr_reference"`
	Lines        []OrderLine `json:"lines"`
	Net          MoneyView   `json:"net"`
	VAT          MoneyView   `json:"vat"`
	Gross        MoneyView   `json:"gross"`
	Paid         MoneyView   `json:"paid"`
	Open         MoneyView   `json:"open"`
	Currency     string      `json:"currency"`
}

type Payment struct {
	PaymentNo  string    `json:"payment_no"`
	InvoiceNo  string    `json:"invoice_no"`
	CustomerNo string    `json:"customer_no"`
	PaidAt     time.Time `json:"paid_at"`
	Amount     MoneyView `json:"amount"`
	Method     string    `json:"method"`
	Currency   string    `json:"currency"`
}

type Stock struct {
	ArticleNo  string `json:"article_no"`
	Name       string `json:"name"`
	Warehouse  string `json:"warehouse"`
	Lagerort   string `json:"lagerort"`
	Unit       string `json:"unit"`
	OnHand     int    `json:"on_hand"`
	Reserved   int    `json:"reserved"`
	Available  int    `json:"available"`
}

type OpenItem struct {
	InvoiceNo    string    `json:"invoice_no"`
	CustomerNo   string    `json:"customer_no"`
	CustomerName string    `json:"customer_name"`
	InvoicedAt   time.Time `json:"invoiced_at"`
	DueAt        time.Time `json:"due_at"`
	Gross        MoneyView `json:"gross"`
	Paid         MoneyView `json:"paid"`
	Open         MoneyView `json:"open"`
	Status       string    `json:"status"`
	Currency     string    `json:"currency"`
}

type ListResult[T any] struct {
	Now         time.Time `json:"now"`
	WindowStart time.Time `json:"window_start"`
	Total       int       `json:"total"`
	Limit       int       `json:"limit"`
	Offset      int       `json:"offset"`
	Items       []T       `json:"items"`
}

type Status struct {
	Now         time.Time     `json:"now"`
	WindowStart time.Time     `json:"window_start"`
	Window      string        `json:"window"`
	TimeScale   string        `json:"time_scale"`
	Seed        int64         `json:"seed"`
	Counts      StatusCounts  `json:"counts"`
}

type StatusCounts struct {
	Customers int `json:"customers"`
	Articles  int `json:"articles"`
	Orders    int `json:"orders"`
	Invoices  int `json:"invoices"`
	Payments  int `json:"payments"`
	OpenItems int `json:"open_items"`
}

type LineInput struct {
	ArticleNo string
	Qty       int
}

type ListFilter struct {
	CustomerNo string
	ArticleNo  string
	Status     string
	Query      string
	Limit      int
	Offset     int
}
