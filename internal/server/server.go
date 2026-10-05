package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ecociel/longshift-opacc-mcp/internal/config"
	"github.com/ecociel/longshift-opacc-mcp/internal/world"
)

const Version = "0.1.0"

type ERP struct {
	World *world.World
}

func New(cfg config.Config, w *world.World) *mcp.Server {
	if w == nil {
		w = world.New(world.Options{
			Seed:      cfg.Seed,
			Window:    cfg.Window,
			TimeScale: cfg.TimeScale,
			Now:       cfg.Now,
		})
	}
	e := &ERP{World: w}

	s := mcp.NewServer(&mcp.Implementation{
		Name:    "longshift-opacc-mcp",
		Version: Version,
	}, &mcp.ServerOptions{
		Instructions: "Simulated Opacc-style Swiss ERP (Handels-Dokumentprinzip: Auftrag → Lieferung → Faktura). No live Opacc connection and no credentials. Master data and a rolling three-month world stay internally consistent with the simulated clock.",
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_world_clock",
		Description: "Return the simulated clock, 3-month window, time scale, and record counts.",
	}, e.getWorldClock)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_customers",
		Description: "List Kunden (business-partner addresses) in the simulated Opacc world.",
	}, e.listCustomers)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_customer",
		Description: "Fetch one Kunde by customer_no (e.g. K-10001).",
	}, e.getCustomer)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_articles",
		Description: "List Artikel with barcode, unit, size, list price, and MWST.",
	}, e.listArticles)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_article",
		Description: "Fetch one Artikel by article_no (e.g. A-20001).",
	}, e.getArticle)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_orders",
		Description: "List Verkaufsaufträge (AU-…) inside the rolling 3-month window.",
	}, e.listOrders)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_order",
		Description: "Fetch one Auftrag by order_no.",
	}, e.getOrder)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_invoices",
		Description: "List Fakturen / Rechnungen (RE-…) inside the window.",
	}, e.listInvoices)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_invoice",
		Description: "Fetch one Rechnung by invoice_no, including open amount and QR reference.",
	}, e.getInvoice)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_payments",
		Description: "List Zahlungen (ZA-…) posted against invoices in the window.",
	}, e.listPayments)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_payment",
		Description: "Fetch one Zahlung by payment_no.",
	}, e.getPayment)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_stock",
		Description: "List Lagerbestand at Hauptlager Rothenburg (on hand, reserved, available).",
	}, e.listStock)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_stock",
		Description: "Fetch Lagerbestand for one article.",
	}, e.getStock)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_open_items",
		Description: "List offene Posten (unpaid or partially paid invoices).",
	}, e.listOpenItems)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_sales_order",
		Description: "Create an Auftrag, issue stock, and post the Faktura plus open item. Fails if stock is insufficient.",
	}, e.createSalesOrder)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "post_payment",
		Description: "Post a Zahlung against an invoice. amount_chf 0 pays the remaining open item. Rejects overpay.",
	}, e.postPayment)
	return s
}

func HTTPHandler(s *mcp.Server, w *world.World) http.Handler {
	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return s
	}, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = rw.Write([]byte("ok\n"))
	})
	mux.Handle("/mcp", mcpHandler)
	mux.Handle("/mcp/", mcpHandler)
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(rw, r)
			return
		}
		info := map[string]any{
			"name":    "longshift-opacc-mcp",
			"version": Version,
			"mcp":     "/mcp",
			"health":  "/healthz",
		}
		if w != nil {
			st := w.Status()
			info["now"] = st.Now
			info["window_start"] = st.WindowStart
			info["time_scale"] = st.TimeScale
		}
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(info)
	})
	return mux
}

type listInput struct {
	CustomerNo string `json:"customer_no,omitempty" jsonschema:"Optional Kundennummer filter, e.g. K-10001"`
	ArticleNo  string `json:"article_no,omitempty" jsonschema:"Optional Artikelnummer filter, e.g. A-20001"`
	Status     string `json:"status,omitempty" jsonschema:"Optional status: open, invoiced, partial, paid"`
	Query      string `json:"query,omitempty" jsonschema:"Optional case-insensitive search over name and number"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Page size, default 50, max 200"`
	Offset     int    `json:"offset,omitempty" jsonschema:"Page offset"`
}

type getCustomerInput struct {
	CustomerNo string `json:"customer_no" jsonschema:"Kundennummer, e.g. K-10001"`
}

type getArticleInput struct {
	ArticleNo string `json:"article_no" jsonschema:"Artikelnummer, e.g. A-20001"`
}

type getOrderInput struct {
	OrderNo string `json:"order_no" jsonschema:"Auftragsnummer, e.g. AU-2026-00001"`
}

type getInvoiceInput struct {
	InvoiceNo string `json:"invoice_no" jsonschema:"Rechnungsnummer, e.g. RE-2026-00001"`
}

type getPaymentInput struct {
	PaymentNo string `json:"payment_no" jsonschema:"Zahlungsnummer, e.g. ZA-2026-00001"`
}

type getStockInput struct {
	ArticleNo string `json:"article_no" jsonschema:"Artikelnummer"`
}

type createOrderInput struct {
	CustomerNo string          `json:"customer_no" jsonschema:"Kundennummer"`
	Lines      []orderLineIn   `json:"lines" jsonschema:"Order positions"`
}

type orderLineIn struct {
	ArticleNo string `json:"article_no" jsonschema:"Artikelnummer"`
	Qty       int    `json:"qty" jsonschema:"Quantity in the article unit"`
}

type postPaymentInput struct {
	InvoiceNo string  `json:"invoice_no" jsonschema:"Rechnungsnummer to pay"`
	AmountCHF float64 `json:"amount_chf,omitempty" jsonschema:"Amount in CHF. Omit or 0 to pay the remaining open item."`
	Method    string  `json:"method,omitempty" jsonschema:"QR-Rechnung (default) or Überweisung"`
}

type emptyInput struct{}

func (e *ERP) getWorldClock(_ context.Context, _ *mcp.CallToolRequest, _ emptyInput) (*mcp.CallToolResult, any, error) {
	return toolResult(e.World.Status(), nil)
}

func (e *ERP) listCustomers(_ context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return toolResult(e.World.ListCustomers(filter(in)), nil)
}

func (e *ERP) getCustomer(_ context.Context, _ *mcp.CallToolRequest, in getCustomerInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.CustomerNo) == "" {
		return toolError("customer_no is required")
	}
	return toolResult(e.World.GetCustomer(in.CustomerNo))
}

func (e *ERP) listArticles(_ context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return toolResult(e.World.ListArticles(filter(in)), nil)
}

func (e *ERP) getArticle(_ context.Context, _ *mcp.CallToolRequest, in getArticleInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ArticleNo) == "" {
		return toolError("article_no is required")
	}
	return toolResult(e.World.GetArticle(in.ArticleNo))
}

func (e *ERP) listOrders(_ context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return toolResult(e.World.ListOrders(filter(in)), nil)
}

func (e *ERP) getOrder(_ context.Context, _ *mcp.CallToolRequest, in getOrderInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.OrderNo) == "" {
		return toolError("order_no is required")
	}
	return toolResult(e.World.GetOrder(in.OrderNo))
}

func (e *ERP) listInvoices(_ context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return toolResult(e.World.ListInvoices(filter(in)), nil)
}

func (e *ERP) getInvoice(_ context.Context, _ *mcp.CallToolRequest, in getInvoiceInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.InvoiceNo) == "" {
		return toolError("invoice_no is required")
	}
	return toolResult(e.World.GetInvoice(in.InvoiceNo))
}

func (e *ERP) listPayments(_ context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return toolResult(e.World.ListPayments(filter(in)), nil)
}

func (e *ERP) getPayment(_ context.Context, _ *mcp.CallToolRequest, in getPaymentInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.PaymentNo) == "" {
		return toolError("payment_no is required")
	}
	return toolResult(e.World.GetPayment(in.PaymentNo))
}

func (e *ERP) listStock(_ context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return toolResult(e.World.ListStock(filter(in)), nil)
}

func (e *ERP) getStock(_ context.Context, _ *mcp.CallToolRequest, in getStockInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.ArticleNo) == "" {
		return toolError("article_no is required")
	}
	return toolResult(e.World.GetStock(in.ArticleNo))
}

func (e *ERP) listOpenItems(_ context.Context, _ *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return toolResult(e.World.ListOpenItems(filter(in)), nil)
}

func (e *ERP) createSalesOrder(_ context.Context, _ *mcp.CallToolRequest, in createOrderInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.CustomerNo) == "" {
		return toolError("customer_no is required")
	}
	lines := make([]world.LineInput, 0, len(in.Lines))
	for _, ln := range in.Lines {
		lines = append(lines, world.LineInput{ArticleNo: ln.ArticleNo, Qty: ln.Qty})
	}
	return toolResult(e.World.CreateSalesOrder(in.CustomerNo, lines))
}

func (e *ERP) postPayment(_ context.Context, _ *mcp.CallToolRequest, in postPaymentInput) (*mcp.CallToolResult, any, error) {
	if strings.TrimSpace(in.InvoiceNo) == "" {
		return toolError("invoice_no is required")
	}
	return toolResult(e.World.PostPayment(in.InvoiceNo, in.AmountCHF, in.Method))
}

func filter(in listInput) world.ListFilter {
	return world.ListFilter{
		CustomerNo: strings.TrimSpace(in.CustomerNo),
		ArticleNo:  strings.TrimSpace(in.ArticleNo),
		Status:     strings.TrimSpace(in.Status),
		Query:      in.Query,
		Limit:      in.Limit,
		Offset:     in.Offset,
	}
}

func toolResult(out any, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return toolError(err.Error())
	}
	return nil, out, nil
}

func toolError(msg string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}, nil, nil
}
