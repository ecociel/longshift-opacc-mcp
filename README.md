# longshift-opacc-mcp

Example [MCP](https://modelcontextprotocol.io/) server in Go that **simulates**
an [Opacc](https://www.opacc.ch/)-style Swiss mid-market ERP. It does not call
Opacc, and it does not require credentials.

The same binary runs over stdio for a local MCP client, or over streamable HTTP
so a Longshift data cell can host it as a tenant-scoped Firecracker app. This
repository also publishes the guest image the cell consumes:

`ghcr.io/ecociel/longshift-opacc-mcp`

## What is being simulated

Public Opacc product pages (checked 2026-10-05) describe a Swiss-made platform
in Rothenburg with **ERP + CRM + online shop** on one stack. The ERP
**Handel** module is document-oriented (Dokumentprinzip): product master data
and sales from **Angebot → Auftrag → Lieferung → Faktura**, plus
Warenwirtschaft, Einkauf, and a transaction-accurate MIS. Warehouse-Management
adds Lagerorte, Wareneingang / Warenausgang, and Chargen. Addresses
(Kunden, Lieferanten) live in the Basic module. Invoicing can be per delivery
or Sammel-Verrechnung. Payments show up in the same system (including shop
checkout integrations such as Payrexx). Currency and tax in this simulator
are **CHF** and **8.1% MWST**.

This server is not a client of that product. It is a self-contained world that
uses those nouns so a demo looks like a Swiss Handels-ERP:

| Record | Opacc-style number | Notes |
| --- | --- | --- |
| Kunde | `K-10001` | CH address, UID MWST, 30 Tage netto |
| Artikel | `A-20001` | barcode (EAN-13, prefix 76), unit, size, list price |
| Auftrag | `AU-2026-00001` | sales order with positions |
| Rechnung | `RE-2026-00001` | Faktura + QR reference + due date |
| Zahlung | `ZA-2026-00001` | QR-Rechnung or Überweisung |
| Lagerbestand | `LG-ROTH` | Hauptlager Rothenburg: on hand / reserved / available |
| Offener Posten | (from invoice) | unpaid remainder of a Faktura |

The tenant is a fictional industrial trader (**Nordlicht-style** assortment:
filter, hydraulics, seals, sensors, fasteners). Names are invented.

## Time and the 3-month window

The world is generated on startup for a **rolling 90-day** period ending at
“now”, then keeps moving. Documents older than the window **drop out**. Master
data (Kunden, Artikel) stays. Stock, open items, and remaining documents stay
internally consistent with the clock: a later read agrees with earlier writes
until those records age out.

Time runs **faster than the wall clock** so a demo does not wait for real
quarters. Default mapping:

| Wall clock | Simulated time |
| --- | --- |
| 1 second | 1 hour (`OPACC_MCP_TIME_SCALE=1h`) |
| 24 seconds | 1 business-ish day |
| ~36 minutes | a full 90-day roll of live time |

Startup already fills the past 90 simulated days, so tools have data
immediately. Set `OPACC_MCP_TIME_SCALE=24h` to advance one simulated day per
wall second, or `1s` to run in real time.

Business hours and weekends follow **Europe/Zurich**. The generator creates
Aufträge, delivers and invoices them (Dokumentprinzip), posts QR payments,
and replenishes low stock with Wareneingang. An MCP `create_sales_order`
issues stock and posts the Faktura immediately so a client can `post_payment`
without waiting for the next simulated afternoon.

## MCP tools

| Tool | Purpose |
| --- | --- |
| `get_world_clock` | Simulated now, window start, scale, counts |
| `list_customers` / `get_customer` | Kunden |
| `list_articles` / `get_article` | Artikel |
| `list_orders` / `get_order` | Aufträge |
| `list_invoices` / `get_invoice` | Rechnungen |
| `list_payments` / `get_payment` | Zahlungen |
| `list_stock` / `get_stock` | Lagerbestand |
| `list_open_items` | Offene Posten |
| `create_sales_order` | Write: Auftrag + stock issue + Faktura |
| `post_payment` | Write: Zahlung against an invoice (`amount_chf` 0 = pay remainder) |

List tools take optional `customer_no`, `article_no`, `status`, `query`,
`limit` (default 50, max 200), `offset`. Writes fail on missing partners,
insufficient stock, or overpayment. Amounts are exact rappen (`rappen` plus
`chf` string).

## Configure and run

No secrets. Configuration is optional environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `OPACC_MCP_TRANSPORT` | `http` | `http` (streamable HTTP) or `stdio` |
| `OPACC_MCP_ADDR` | `:8080` | Listen address in HTTP mode. Bind `0.0.0.0`, not localhost, inside a cell. |
| `OPACC_MCP_TIME_SCALE` | `1h` | Simulated duration **per wall-clock second**. Also accepts `24h`, `1s`, or `90d`. |
| `OPACC_MCP_WINDOW` | `90d` | How far back records are kept. Go durations or `Nd`. |
| `OPACC_MCP_SEED` | `1` | Deterministic generator seed. |
| `OPACC_MCP_TICK` | `1s` | Wall interval for advancing the world. |
| `OPACC_MCP_NOW` | (current time) | Optional RFC3339 origin for simulated “now”. |

HTTP mode also serves `GET /healthz` (`ok`) and `GET /` (service JSON plus
clock). The MCP endpoint is `/mcp`.

```bash
go test ./...
go run ./cmd/opacc-mcp
# MCP:  http://127.0.0.1:8080/mcp
# health: http://127.0.0.1:8080/healthz
```

Local stdio (Cursor / Claude Desktop and similar):

```json
{
  "mcpServers": {
    "opacc": {
      "command": "go",
      "args": ["run", "./cmd/opacc-mcp"],
      "env": {
        "OPACC_MCP_TRANSPORT": "stdio"
      }
    }
  }
}
```

Point a streamable-HTTP MCP client at `http://<host>:8080/mcp` after HTTP mode is up.

## OCI image this repo publishes

GitHub Actions workflow [`.github/workflows/publish-image.yml`](.github/workflows/publish-image.yml)
builds `linux/amd64` and pushes to GHCR:

| Tag | When |
| --- | --- |
| `ghcr.io/ecociel/longshift-opacc-mcp:latest` | default branch |
| `ghcr.io/ecociel/longshift-opacc-mcp:sha-<full-sha>` | every build |
| `ghcr.io/ecociel/longshift-opacc-mcp:<short-sha>` | every build |
| `ghcr.io/ecociel/longshift-opacc-mcp:pr-<n>` | pull requests |
| `ghcr.io/ecociel/longshift-opacc-mcp:<x.y.z>` | `v*` tags |

The image is a static Go binary on `gcr.io/distroless/static-debian12:nonroot`.
`ENTRYPOINT` is `/opacc-mcp`. It listens on `8080`. There is no kernel, no
init system, and no secret in the layers.

Local build (optional; CI is the source of the published name):

```bash
docker build -t ghcr.io/ecociel/longshift-opacc-mcp:dev .
docker run --rm -p 8080:8080 ghcr.io/ecociel/longshift-opacc-mcp:dev
```

First GHCR push creates a package that is private to the org by default. For a
data cell to pull it, either make
[the package public](https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility)
or grant the cell’s pull identity `read:packages`. Do not put a GHCR token in
the guest image or in guest env.

## Deploy that image as a Longshift data-cell app

### Source of the contract

The task points at current `main` of `https://github.com/ecociel/longshift`,
especially `docs/19-cell-apps.md` and the merged OCI cell-app deploy (PR
**#258**, `dde2427`, “Deploy an OCI image onto a data cell”).

This environment cannot read that repository (GitHub returns 404 / “not found”
for anonymous and for the tokens available here). The steps below therefore
follow the **deploy contract those sources are specified to encode**, not an
invented CLI. If you have the repo locally and `docs/19-cell-apps.md` and the
#258 code disagree, **follow the code on `main`**.

### Contract (what the cell accepts)

| You supply | You do not supply |
| --- | --- |
| One OCI image reference | A kernel / `vmlinux` |
| Tenant that owns the app | A rootfs disk (`rootfs.ext4`, `overlay.img`, …) |
| Non-secret runtime config the control plane already supports | Guest-visible secrets, tokens, or `.env` files |

The cell:

1. Pulls the OCI image.
2. Materializes a guest rootfs **internally** from the image layers.
3. Boots **Firecracker only**, with the **platform kernel**.
4. Starts the image `ENTRYPOINT` (`/opacc-mcp`) as a **tenant-scoped app**.
5. Keeps **secrets off the guest**. Registry credentials, cell identity, and any
   later secret store stay on the host / control plane.

This image matches that split: userspace only, `linux/amd64`, process as PID 1,
port 8080, no key material.

### Packaging for Longshift (not a generic container host)

- Do not wrap the image in Docker Compose, `runc`, Kata, or a second guest
  runtime. The cell is Firecracker; the OCI image is input, not the hypervisor.
- Do not bake a kernel or an ext4 rootfs next to the Dockerfile. If a deploy
  form has “kernel” or “rootfs” fields, leave them to the platform.
- Do not `docker save` a rootfs tarball as the artifact. Push the image to
  GHCR; the cell consumes the registry reference.
- Do not put `OPACC_MCP_*` secrets in the image. This server has none. It does
  not dial Opacc and needs **no egress** and **no inbound credentials**.
- If you later add authenticated upstreams, inject those values from the cell
  control plane so they never land in the image or in a guest-writable secret
  file.

### Cell app steps

Use whatever app-create path current `main` exposes after #258 (API, CLI, or
UI). The payload that matters is:

1. **Image** (required): `ghcr.io/ecociel/longshift-opacc-mcp:latest`
   or pin `ghcr.io/ecociel/longshift-opacc-mcp:sha-<git-sha>` from the
   workflow that built the revision you want.
2. **Tenant**: the tenant that should own the app. The app is not a
   cell-global service.
3. **Guest listen port**: `8080`. MCP clients use path `/mcp`. Liveness can
   probe `/healthz` from wherever the platform runs health checks — that probe
   belongs on the host side if the platform offers one.
4. **Optional env** (non-secret): `OPACC_MCP_TIME_SCALE`, `OPACC_MCP_WINDOW`,
   `OPACC_MCP_SEED`, `OPACC_MCP_ADDR=:8080`. Omit anything that looks like a
   token.
5. **Registry pull**: host-side. If GHCR is private, configure pull auth on
   the cell, not as `GITHUB_TOKEN` inside the guest.

After the app is running, point an MCP client at the URL the cell publishes
for that app, path `/mcp`.

### If you are reading longshift `main`

Confirm these against the #258 implementation, not only the doc:

- Image field is an OCI reference; there is no caller-supplied kernel/rootfs.
- Runtime enum / path is Firecracker; other isolators are rejected.
- App records include a tenant id and cannot be listed across tenants.
- Secret / registry credentials are not copied into the guest filesystem.

Where the doc and that code diverge, change the deploy call to match the code
and treat the doc as stale.

## Layout

```
cmd/opacc-mcp/           HTTP or stdio process
internal/config/         environment
internal/world/          rolling 3-month ERP simulator
internal/server/         MCP tools + /mcp + /healthz
Dockerfile               linux/amd64 guest image
.github/workflows/       test, then push to GHCR
```
