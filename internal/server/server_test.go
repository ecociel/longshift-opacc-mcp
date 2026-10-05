package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ecociel/longshift-opacc-mcp/internal/config"
	"github.com/ecociel/longshift-opacc-mcp/internal/world"
)

func testERP(t *testing.T) (*mcp.Server, *world.World) {
	t.Helper()
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	w := world.New(world.Options{
		Seed:      1,
		Window:    90 * 24 * time.Hour,
		TimeScale: time.Hour,
		Clock:     world.NewManualClock(now),
	})
	return New(config.Config{}, w), w
}

func TestHealthzAndIndex(t *testing.T) {
	srv, w := testERP(t)
	h := HTTPHandler(srv, w)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || rr.Body.String() != "ok\n" {
		t.Fatalf("healthz: %d %q", rr.Code, rr.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("index status %d", rr.Code)
	}
	var info map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info["mcp"] != "/mcp" {
		t.Fatalf("index = %#v", info)
	}
}

func TestMCPToolsListFetchAndWrite(t *testing.T) {
	srv, w := testERP(t)
	ctx := context.Background()
	session := connect(t, ctx, srv)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"get_world_clock":    false,
		"list_customers":     false,
		"get_customer":       false,
		"list_articles":      false,
		"get_article":        false,
		"list_orders":        false,
		"get_order":          false,
		"list_invoices":      false,
		"get_invoice":        false,
		"list_payments":      false,
		"get_payment":        false,
		"list_stock":         false,
		"get_stock":          false,
		"list_open_items":    false,
		"create_sales_order": false,
		"post_payment":       false,
	}
	for _, tool := range tools.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("missing tool %s", name)
		}
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_customers", Arguments: map[string]any{"limit": 1.0}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("list_customers: %#v", res)
	}

	var article string
	for _, s := range w.ListStock(world.ListFilter{Limit: 200}).Items {
		if s.Available >= 2 {
			article = s.ArticleNo
			break
		}
	}
	cust := w.ListCustomers(world.ListFilter{Limit: 1}).Items[0]
	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_sales_order",
		Arguments: map[string]any{
			"customer_no": cust.CustomerNo,
			"lines":       []map[string]any{{"article_no": article, "qty": 2}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("create_sales_order: %#v", res)
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var ord world.Order
	if err := json.Unmarshal(raw, &ord); err != nil {
		t.Fatalf("%s: %v", raw, err)
	}
	if ord.InvoiceNo == "" {
		t.Fatalf("order missing invoice: %+v", ord)
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_order",
		Arguments: map[string]any{"order_no": ord.OrderNo},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("get_order: %#v", res)
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "post_payment",
		Arguments: map[string]any{"invoice_no": ord.InvoiceNo},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("post_payment: %#v", res)
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_invoice",
		Arguments: map[string]any{"invoice_no": ord.InvoiceNo},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("get_invoice: %#v", res)
	}
	raw, err = json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	var inv world.Invoice
	if err := json.Unmarshal(raw, &inv); err != nil {
		t.Fatal(err)
	}
	if inv.Status != world.StatusPaid {
		t.Fatalf("invoice after pay: %+v", inv)
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "create_sales_order",
		Arguments: map[string]any{
			"customer_no": cust.CustomerNo,
			"lines":       []map[string]any{{"article_no": article, "qty": 1_000_000}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected stock error")
	}
	if p := w.Problems(); len(p) > 0 {
		t.Fatalf("%v", p)
	}
}

func connect(t *testing.T, ctx context.Context, srv *mcp.Server) *mcp.ClientSession {
	t.Helper()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	go func() {
		_ = srv.Run(ctx, serverTransport)
	}()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}
