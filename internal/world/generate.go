package world

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"time"

	_ "time/tzdata"
)

var zurich *time.Location

func init() {
	var err error
	zurich, err = time.LoadLocation("Europe/Zurich")
	if err != nil {
		zurich = time.FixedZone("CET", 3600)
	}
}

type customerSeed struct {
	name, street, plz, city, kanton, lang string
	uidN                                  int
}

type articleSeed struct {
	name, nameDE, unit, size, group string
	priceRappen                     int64
	barcodeBody                     string
}

var customerSeeds = []customerSeed{
	{"Alpine Filter AG", "Breitenstrasse 16b", "8500", "Frauenfeld", "TG", "de-CH", 101234567},
	{"Luzerner Werkzeuge GmbH", "Pilatusstrasse 8", "6003", "Luzern", "LU", "de-CH", 102345678},
	{"Seetal Automation AG", "Industriestrasse 12", "6280", "Hochdorf", "LU", "de-CH", 103456789},
	{"Rigi Komponenten AG", "Seestrasse 4", "6403", "Küssnacht am Rigi", "SZ", "de-CH", 104567890},
	{"Emme Hydraulik AG", "Lyssachstrasse 21", "3400", "Burgdorf", "BE", "de-CH", 105678901},
	{"Pilatus Dichtungen AG", "Engelbergstrasse 9", "6370", "Stans", "NW", "de-CH", 106789012},
	{"Aare Präzision AG", "Bielstrasse 55", "4500", "Solothurn", "SO", "de-CH", 107890123},
	{"Tessin Componenti SA", "Via Industria 7", "6850", "Mendrisio", "TI", "it-CH", 108901234},
	{"Léman Messtechnik SA", "Route de l'Etraz 3", "1260", "Nyon", "VD", "fr-CH", 109012345},
	{"Basler Antriebstechnik AG", "St. Jakobs-Strasse 200", "4132", "Muttenz", "BL", "de-CH", 110123456},
	{"Zürcher Sensorik AG", "Binzmühlestrasse 14", "8050", "Zürich", "ZH", "de-CH", 111234567},
	{"Thur Dicht + Dämm AG", "Wilerstrasse 80", "9500", "Wil", "SG", "de-CH", 112345678},
	{"Jura Pneumatik AG", "Bahnhofstrasse 2", "4512", "Bellach", "SO", "de-CH", 113456789},
	{"Rhone Process SA", "Avenue du Simplon 11", "1920", "Martigny", "VS", "fr-CH", 114567890},
	{"Aargau Fördertechnik AG", "Hauptstrasse 36", "5600", "Lenzburg", "AG", "de-CH", 115678901},
	{"Ostschweiz Laborbedarf AG", "Lerchenfeldstrasse 5", "9014", "St. Gallen", "SG", "de-CH", 116789012},
	{"Bernese Tools AG", "Morgenstrasse 139", "3018", "Bern", "BE", "de-CH", 117890123},
	{"Innerschweiz Service AG", "Werkhofstrasse 5", "6020", "Emmenbrücke", "LU", "de-CH", 118901234},
	{"Schaffhauser Metall AG", "Ebnatstrasse 65", "8200", "Schaffhausen", "SH", "de-CH", 119012345},
	{"Graubünden Alpine Parts AG", "Comercialstrasse 19", "7000", "Chur", "GR", "de-CH", 120123456},
}

var articleSeeds = []articleSeed{
	{"Filter cartridge cellulose", "Filterkerze Cellulose", "STK", "10\"", "Filter", 2450, "761234500001"},
	{"Filter cartridge cellulose", "Filterkerze Cellulose", "STK", "20\"", "Filter", 3890, "761234500002"},
	{"Filter cartridge polypropylene", "Filterkerze Polypropylen", "STK", "10\"", "Filter", 3120, "761234500003"},
	{"Filter housing stainless", "Filtergehäuse Edelstahl", "STK", "DN25", "Filter", 18900, "761234500004"},
	{"Filter housing stainless", "Filtergehäuse Edelstahl", "STK", "DN50", "Filter", 24600, "761234500005"},
	{"Hydraulic fitting elbow", "Hydraulik-Winkelverschraubung", "STK", "12L", "Hydraulik", 870, "761234500006"},
	{"Hydraulic fitting elbow", "Hydraulik-Winkelverschraubung", "STK", "15L", "Hydraulik", 980, "761234500007"},
	{"Hydraulic hose assembly", "Hydraulikschlauch konfektioniert", "STK", "DN10", "Hydraulik", 5400, "761234500008"},
	{"Hydraulic hose assembly", "Hydraulikschlauch konfektioniert", "STK", "DN16", "Hydraulik", 7200, "761234500009"},
	{"O-ring NBR 70", "O-Ring NBR 70", "STK", "20x2.5", "Dichtung", 85, "761234500010"},
	{"O-ring NBR 70", "O-Ring NBR 70", "STK", "30x3", "Dichtung", 110, "761234500011"},
	{"O-ring FKM 75", "O-Ring FKM 75", "STK", "20x2.5", "Dichtung", 240, "761234500012"},
	{"Flat gasket PTFE", "Flachdichtung PTFE", "STK", "DN25", "Dichtung", 620, "761234500013"},
	{"Flat gasket PTFE", "Flachdichtung PTFE", "STK", "DN50", "Dichtung", 980, "761234500014"},
	{"Pressure sensor 0-10 bar", "Drucksensor 0-10 bar", "STK", "G1/4", "Sensorik", 16800, "761234500015"},
	{"Pressure sensor 0-25 bar", "Drucksensor 0-25 bar", "STK", "G1/4", "Sensorik", 17900, "761234500016"},
	{"Temperature probe PT100", "Temperaturfühler PT100", "STK", "100mm", "Sensorik", 9400, "761234500017"},
	{"Proximity switch M12", "Näherungsschalter M12", "STK", "M12", "Sensorik", 4100, "761234500018"},
	{"Hex bolt 8.8", "Sechskantschraube 8.8", "STK", "M8x30", "Befestigung", 35, "761234500019"},
	{"Hex bolt 8.8", "Sechskantschraube 8.8", "STK", "M10x40", "Befestigung", 48, "761234500020"},
	{"Hex nut 8", "Sechskantmutter 8", "STK", "M8", "Befestigung", 18, "761234500021"},
	{"Hex nut 8", "Sechskantmutter 8", "STK", "M10", "Befestigung", 22, "761234500022"},
	{"Washer A2", "Scheibe A2", "STK", "M8", "Befestigung", 12, "761234500023"},
	{"Cable gland polyamide", "Kabelverschraubung Polyamid", "STK", "M16", "Elektro", 210, "761234500024"},
	{"Cable gland polyamide", "Kabelverschraubung Polyamid", "STK", "M20", "Elektro", 260, "761234500025"},
	{"Motor protection switch", "Motorschutzschalter", "STK", "2.5-4A", "Elektro", 7800, "761234500026"},
	{"Contactor 24VDC", "Schütz 24VDC", "STK", "9A", "Elektro", 5600, "761234500027"},
	{"Pneumatic cylinder ISO", "Pneumatikzylinder ISO", "STK", "32-100", "Pneumatik", 12400, "761234500028"},
	{"Pneumatic cylinder ISO", "Pneumatikzylinder ISO", "STK", "50-200", "Pneumatik", 16800, "761234500029"},
	{"Solenoid valve 5/2", "Magnetventil 5/2", "STK", "G1/8", "Pneumatik", 8900, "761234500030"},
	{"Service kit compressor", "Wartungskit Kompressor", "STK", "", "Ersatzteil", 13200, "761234500031"},
	{"V-belt SPA", "Keilriemen SPA", "STK", "1250", "Ersatzteil", 2800, "761234500032"},
	{"Coupling insert", "Kupplungseinsatz", "STK", "38", "Ersatzteil", 4500, "761234500033"},
	{"Linear guide carriage", "Führungswagen", "STK", "15", "Mechanik", 9800, "761234500034"},
	{"Linear rail", "Führungsschiene", "M", "15", "Mechanik", 6200, "761234500035"},
	{"Ball bearing 6204-2RS", "Rillenkugellager 6204-2RS", "STK", "20x47x14", "Mechanik", 1450, "761234500036"},
	{"Ball bearing 6206-2RS", "Rillenkugellager 6206-2RS", "STK", "30x62x16", "Mechanik", 1980, "761234500037"},
	{"Gear oil ISO VG 220", "Getriebeöl ISO VG 220", "KG", "20", "Betriebsstoff", 890, "761234500038"},
	{"Hydraulic oil HLP 46", "Hydrauliköl HLP 46", "KG", "20", "Betriebsstoff", 640, "761234500039"},
	{"Cleaning cloth industrial", "Industriereinigungstuch", "STK", "200er", "Betrieb", 1850, "761234500040"},
}

func ean13(body12 string) string {
	sum := 0
	for i := 0; i < 12 && i < len(body12); i++ {
		d := int(body12[i] - '0')
		if i%2 == 0 {
			sum += d
		} else {
			sum += 3 * d
		}
	}
	check := (10 - (sum % 10)) % 10
	return fmt.Sprintf("%s%d", body12, check)
}

func swissUID(n int) string {
	return fmt.Sprintf("CHE-%03d.%03d.%03d MWST", n/1_000_000, (n/1000)%1000, n%1000)
}

func (w *World) seedMaster() {
	for i, s := range customerSeeds {
		no := fmt.Sprintf("K-%05d", 10001+i)
		w.customers[no] = Customer{
			CustomerNo:       no,
			Name:             s.name,
			UID:              swissUID(s.uidN),
			Street:           s.street,
			PLZ:              s.plz,
			City:             s.city,
			Kanton:           s.kanton,
			Country:          "CH",
			Language:         s.lang,
			PaymentTermsDays: 30,
		}
		w.customerNos = append(w.customerNos, no)
	}
	for i, s := range articleSeeds {
		no := fmt.Sprintf("A-%05d", 20001+i)
		w.articles[no] = &articleRec{
			no:        no,
			name:      s.name,
			nameDE:    s.nameDE,
			unit:      s.unit,
			size:      s.size,
			barcode:   ean13(s.barcodeBody),
			group:     s.group,
			listPrice: Money(s.priceRappen),
			vat:       StdVAT,
		}
		w.stock[no] = &stockRec{onHand: 80 + (i*7)%140}
		w.articleNos = append(w.articleNos, no)
	}
}

func (w *World) generateHour(t time.Time) {
	local := t.In(zurich)
	if local.Weekday() == time.Saturday || local.Weekday() == time.Sunday {
		return
	}
	hour := local.Hour()
	if hour < 8 || hour > 17 {
		return
	}
	rng := w.rngFor(t, "hour")
	if hour == 8 {
		w.replenish(rng, t)
	}
	if hour >= 9 && hour <= 16 {
		if rng.IntN(100) < 45 {
			w.generateOpenOrder(rng, t)
		}
		if hour == 16 {
			w.progressOpenOrders(t)
		}
		if rng.IntN(100) < 35 {
			w.maybePay(rng, t)
		}
	}
}

func (w *World) replenish(rng *rand.Rand, t time.Time) {
	_ = t
	for _, no := range w.articleNos {
		st := w.stock[no]
		avail := st.onHand - st.reserved
		if avail >= 25 {
			continue
		}
		st.onHand += 40 + rng.IntN(80)
	}
}

func (w *World) generateOpenOrder(rng *rand.Rand, t time.Time) {
	c := w.customers[w.customerNos[rng.IntN(len(w.customerNos))]]
	nLines := 1 + rng.IntN(3)
	seen := map[string]bool{}
	var in []LineInput
	for i := 0; i < nLines; i++ {
		no := w.articleNos[rng.IntN(len(w.articleNos))]
		if seen[no] {
			continue
		}
		seen[no] = true
		qty := 1 + rng.IntN(6)
		st := w.stock[no]
		if st.onHand-st.reserved < qty {
			continue
		}
		in = append(in, LineInput{ArticleNo: no, Qty: qty})
	}
	if len(in) == 0 {
		return
	}
	built, err := w.buildLines(in)
	if err != nil {
		return
	}
	for _, ln := range built.lines {
		w.stock[ln.ArticleNo].reserved += ln.Qty
	}
	w.addOrder(c, built, t, StatusOpen, SourceGenerator)
}

func (w *World) progressOpenOrders(t time.Time) {
	cutoff := t.Add(-20 * time.Hour)
	var due []*orderRec
	for _, o := range w.orders {
		if o.status != StatusOpen || o.source != SourceGenerator {
			continue
		}
		if o.orderedAt.After(cutoff) {
			continue
		}
		due = append(due, o)
	}
	sort.Slice(due, func(i, j int) bool { return due[i].no < due[j].no })
	for _, o := range due {
		c, ok := w.customers[o.customerNo]
		if !ok {
			continue
		}
		for _, ln := range o.lines {
			st := w.stock[ln.ArticleNo]
			st.onHand -= ln.Qty
			st.reserved -= ln.Qty
			if st.reserved < 0 {
				st.reserved = 0
			}
		}
		delivered := t
		o.deliveredAt = &delivered
		o.status = StatusInvoiced
		inv := w.addInvoice(c, o, t)
		o.invoiceNo = inv.no
	}
}

func (w *World) maybePay(rng *rand.Rand, t time.Time) {
	var candidates []*invoiceRec
	for _, inv := range w.invoices {
		if inv.gross-inv.paid <= 0 {
			continue
		}
		if t.Before(inv.invoicedAt.Add(5 * 24 * time.Hour)) {
			continue
		}
		candidates = append(candidates, inv)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].no < candidates[j].no })
	if len(candidates) == 0 {
		return
	}
	inv := candidates[rng.IntN(len(candidates))]
	open := inv.gross - inv.paid
	amount := open
	if rng.IntN(100) < 20 && open > 100 {
		amount = open / 2
	}
	inv.paid += amount
	w.paymentSeq++
	p := &paymentRec{
		no:         fmt.Sprintf("ZA-%d-%05d", t.Year(), w.paymentSeq),
		invoiceNo:  inv.no,
		customerNo: inv.customerNo,
		method:     "QR-Rechnung",
		paidAt:     t,
		amount:     amount,
	}
	w.payments[p.no] = p
}
