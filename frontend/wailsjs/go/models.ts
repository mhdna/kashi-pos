export namespace main {
	
	export class ClientCursorPage {
	    items: any[];
	    nextCursor: number;
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ClientCursorPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.nextCursor = source["nextCursor"];
	        this.hasMore = source["hasMore"];
	    }
	}
	export class ClockStatus {
	    checked: boolean;
	    zone: string;
	    pcZone: string;
	    zoneMismatch: boolean;
	    driftSeconds: number;
	    clockOff: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ClockStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checked = source["checked"];
	        this.zone = source["zone"];
	        this.pcZone = source["pcZone"];
	        this.zoneMismatch = source["zoneMismatch"];
	        this.driftSeconds = source["driftSeconds"];
	        this.clockOff = source["clockOff"];
	    }
	}
	export class SyncPendingByType {
	    entityType: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new SyncPendingByType(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entityType = source["entityType"];
	        this.count = source["count"];
	    }
	}
	export class CloseShiftSyncGate {
	    required: boolean;
	    pending: number;
	    failed: number;
	    byType: SyncPendingByType[];
	
	    static createFrom(source: any = {}) {
	        return new CloseShiftSyncGate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.required = source["required"];
	        this.pending = source["pending"];
	        this.failed = source["failed"];
	        this.byType = this.convertValues(source["byType"], SyncPendingByType);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ConversionDay {
	    day: string;
	    weekday: string;
	    visitors: number;
	    invoices: number;
	    revenue: number;
	
	    static createFrom(source: any = {}) {
	        return new ConversionDay(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.day = source["day"];
	        this.weekday = source["weekday"];
	        this.visitors = source["visitors"];
	        this.invoices = source["invoices"];
	        this.revenue = source["revenue"];
	    }
	}
	export class ConversionGroup {
	    label: string;
	    days: number;
	    visitors: number;
	    invoices: number;
	    revenue: number;
	    buyRate: number;
	    avgInvoice: number;
	    revenuePerVisitor: number;
	
	    static createFrom(source: any = {}) {
	        return new ConversionGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.days = source["days"];
	        this.visitors = source["visitors"];
	        this.invoices = source["invoices"];
	        this.revenue = source["revenue"];
	        this.buyRate = source["buyRate"];
	        this.avgInvoice = source["avgInvoice"];
	        this.revenuePerVisitor = source["revenuePerVisitor"];
	    }
	}
	export class ConversionStats {
	    days: number;
	    visitors: number;
	    invoices: number;
	    revenue: number;
	    buyRate: number;
	    avgInvoice: number;
	    revenuePerVisitor: number;
	
	    static createFrom(source: any = {}) {
	        return new ConversionStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.days = source["days"];
	        this.visitors = source["visitors"];
	        this.invoices = source["invoices"];
	        this.revenue = source["revenue"];
	        this.buyRate = source["buyRate"];
	        this.avgInvoice = source["avgInvoice"];
	        this.revenuePerVisitor = source["revenuePerVisitor"];
	    }
	}
	export class ConversionReport {
	    from: string;
	    to: string;
	    monthOnly: boolean;
	    overall: ConversionStats;
	    uncountedDays: number;
	    byTraffic: ConversionGroup[];
	    byWeekday: ConversionGroup[];
	    days: ConversionDay[];
	
	    static createFrom(source: any = {}) {
	        return new ConversionReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.monthOnly = source["monthOnly"];
	        this.overall = this.convertValues(source["overall"], ConversionStats);
	        this.uncountedDays = source["uncountedDays"];
	        this.byTraffic = this.convertValues(source["byTraffic"], ConversionGroup);
	        this.byWeekday = this.convertValues(source["byWeekday"], ConversionGroup);
	        this.days = this.convertValues(source["days"], ConversionDay);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class DeviceStatus {
	    scanner: string;
	    printer: string;
	    keyboard: string;
	    mouse: string;
	    printerName: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scanner = source["scanner"];
	        this.printer = source["printer"];
	        this.keyboard = source["keyboard"];
	        this.mouse = source["mouse"];
	        this.printerName = source["printerName"];
	    }
	}
	export class DisplayPrefs {
	    prefs: Record<string, any>;
	    rev: number;
	
	    static createFrom(source: any = {}) {
	        return new DisplayPrefs(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prefs = source["prefs"];
	        this.rev = source["rev"];
	    }
	}
	export class ExchangeQuote {
	    returnedTotalCents: number;
	    replacementTotalCents: number;
	    taxCents: number;
	    grandTotalCents: number;
	    needsMoreReplacementValue: boolean;
	    shortfallCents: number;
	
	    static createFrom(source: any = {}) {
	        return new ExchangeQuote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.returnedTotalCents = source["returnedTotalCents"];
	        this.replacementTotalCents = source["replacementTotalCents"];
	        this.taxCents = source["taxCents"];
	        this.grandTotalCents = source["grandTotalCents"];
	        this.needsMoreReplacementValue = source["needsMoreReplacementValue"];
	        this.shortfallCents = source["shortfallCents"];
	    }
	}
	export class FullCheckEntityReport {
	    entity: string;
	    central: number;
	    fetched: number;
	    deactivated: number;
	
	    static createFrom(source: any = {}) {
	        return new FullCheckEntityReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entity = source["entity"];
	        this.central = source["central"];
	        this.fetched = source["fetched"];
	        this.deactivated = source["deactivated"];
	    }
	}
	export class FullCheckReport {
	    adoptedClientUids: number;
	    entities: FullCheckEntityReport[];
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new FullCheckReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.adoptedClientUids = source["adoptedClientUids"];
	        this.entities = this.convertValues(source["entities"], FullCheckEntityReport);
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InvoicePage {
	    items: any[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new InvoicePage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.total = source["total"];
	    }
	}
	export class LineQuote {
	    code: string;
	    barcode: string;
	    lineTotalCents: number;
	    discountCents: number;
	    qty: number;
	    deal?: string;
	    dealLabel?: string;
	    dealDiscountCents?: number;
	    free?: boolean;
	    offDiscount?: number;
	    offDealLabel?: string;
	
	    static createFrom(source: any = {}) {
	        return new LineQuote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.barcode = source["barcode"];
	        this.lineTotalCents = source["lineTotalCents"];
	        this.discountCents = source["discountCents"];
	        this.qty = source["qty"];
	        this.deal = source["deal"];
	        this.dealLabel = source["dealLabel"];
	        this.dealDiscountCents = source["dealDiscountCents"];
	        this.free = source["free"];
	        this.offDiscount = source["offDiscount"];
	        this.offDealLabel = source["offDealLabel"];
	    }
	}
	export class InvoiceQuote {
	    subtotalCents: number;
	    discountedSubtotalCents: number;
	    taxCents: number;
	    grandTotalCents: number;
	    roundingCurrencyCode: string;
	    roundingMode: string;
	    lines: LineQuote[];
	
	    static createFrom(source: any = {}) {
	        return new InvoiceQuote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subtotalCents = source["subtotalCents"];
	        this.discountedSubtotalCents = source["discountedSubtotalCents"];
	        this.taxCents = source["taxCents"];
	        this.grandTotalCents = source["grandTotalCents"];
	        this.roundingCurrencyCode = source["roundingCurrencyCode"];
	        this.roundingMode = source["roundingMode"];
	        this.lines = this.convertValues(source["lines"], LineQuote);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InvoiceSyncState {
	    invoiceId: number;
	    state: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new InvoiceSyncState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.invoiceId = source["invoiceId"];
	        this.state = source["state"];
	        this.error = source["error"];
	    }
	}
	
	export class ManagedProductPage {
	    items: any[];
	    nextCursor: number;
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ManagedProductPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.nextCursor = source["nextCursor"];
	        this.hasMore = source["hasMore"];
	    }
	}
	export class OversellLine {
	    code: string;
	    name: string;
	    requested: number;
	    available: number;
	
	    static createFrom(source: any = {}) {
	        return new OversellLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.requested = source["requested"];
	        this.available = source["available"];
	    }
	}
	export class OversellCheck {
	    mode: string;
	    lines: OversellLine[];
	
	    static createFrom(source: any = {}) {
	        return new OversellCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.lines = this.convertValues(source["lines"], OversellLine);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class PaymentStatus {
	    grandTotalCents: number;
	    paidCents: number;
	    diffCents: number;
	    isPaidInFull: boolean;
	    isCashRounded: boolean;
	    checkoutReady: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PaymentStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.grandTotalCents = source["grandTotalCents"];
	        this.paidCents = source["paidCents"];
	        this.diffCents = source["diffCents"];
	        this.isPaidInFull = source["isPaidInFull"];
	        this.isCashRounded = source["isCashRounded"];
	        this.checkoutReady = source["checkoutReady"];
	    }
	}
	export class PendingDeactivation {
	    productBarcodes: string[];
	    productNames: string[];
	
	    static createFrom(source: any = {}) {
	        return new PendingDeactivation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.productBarcodes = source["productBarcodes"];
	        this.productNames = source["productNames"];
	    }
	}
	export class ProductCSVProblem {
	    line: number;
	    barcode: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ProductCSVProblem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.barcode = source["barcode"];
	        this.message = source["message"];
	    }
	}
	export class ProductCSVImportResult {
	    imported: number;
	    failed: number;
	    problems: ProductCSVProblem[];
	
	    static createFrom(source: any = {}) {
	        return new ProductCSVImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = source["imported"];
	        this.failed = source["failed"];
	        this.problems = this.convertValues(source["problems"], ProductCSVProblem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProductCSVPreview {
	    file: string;
	    total: number;
	    new: number;
	    existing: number;
	    invalid: number;
	    problems: ProductCSVProblem[];
	
	    static createFrom(source: any = {}) {
	        return new ProductCSVPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.total = source["total"];
	        this.new = source["new"];
	        this.existing = source["existing"];
	        this.invalid = source["invalid"];
	        this.problems = this.convertValues(source["problems"], ProductCSVProblem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ProductCursorPage {
	    items: any[];
	    nextCursor: number;
	    hasMore: boolean;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new ProductCursorPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.nextCursor = source["nextCursor"];
	        this.hasMore = source["hasMore"];
	        this.total = source["total"];
	    }
	}
	export class ProductInput {
	    barcode: string;
	    code: string;
	    name: string;
	    description: string;
	    priceCents: number;
	    discount: number;
	    color: string;
	    size: string;
	    isActive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProductInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.barcode = source["barcode"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.priceCents = source["priceCents"];
	        this.discount = source["discount"];
	        this.color = source["color"];
	        this.size = source["size"];
	        this.isActive = source["isActive"];
	    }
	}
	export class ProductPage {
	    items: any[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new ProductPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.total = source["total"];
	    }
	}
	export class QuickAccessSource {
	    mode: string;
	    kashiAvailable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QuickAccessSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.kashiAvailable = source["kashiAvailable"];
	    }
	}
	export class ResyncEntityReport {
	    entity: string;
	    local: number;
	    central: number;
	    missing: number;
	    queued: number;
	
	    static createFrom(source: any = {}) {
	        return new ResyncEntityReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entity = source["entity"];
	        this.local = source["local"];
	        this.central = source["central"];
	        this.missing = source["missing"];
	        this.queued = source["queued"];
	    }
	}
	export class ResyncOptions {
	    applyStock: boolean;
	    dryRun: boolean;
	    confirmDeactivation: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ResyncOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.applyStock = source["applyStock"];
	        this.dryRun = source["dryRun"];
	        this.confirmDeactivation = source["confirmDeactivation"];
	    }
	}
	export class ResyncReport {
	    target: string;
	    dryRun: boolean;
	    entities: ResyncEntityReport[];
	    conflicts: number;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt: any;
	    requiresConfirmation: boolean;
	    pendingDeactivation?: PendingDeactivation;
	    fullCheck?: FullCheckReport;
	
	    static createFrom(source: any = {}) {
	        return new ResyncReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = source["target"];
	        this.dryRun = source["dryRun"];
	        this.entities = this.convertValues(source["entities"], ResyncEntityReport);
	        this.conflicts = source["conflicts"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
	        this.requiresConfirmation = source["requiresConfirmation"];
	        this.pendingDeactivation = this.convertValues(source["pendingDeactivation"], PendingDeactivation);
	        this.fullCheck = this.convertValues(source["fullCheck"], FullCheckReport);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SalespersonProductSale {
	    salespersonId: number;
	    salespersonName: string;
	    productId: number;
	    barcode: string;
	    code: string;
	    name: string;
	    color: string;
	    size: string;
	    currencyCode: string;
	    quantity: number;
	    revenue: number;
	
	    static createFrom(source: any = {}) {
	        return new SalespersonProductSale(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.salespersonId = source["salespersonId"];
	        this.salespersonName = source["salespersonName"];
	        this.productId = source["productId"];
	        this.barcode = source["barcode"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.color = source["color"];
	        this.size = source["size"];
	        this.currencyCode = source["currencyCode"];
	        this.quantity = source["quantity"];
	        this.revenue = source["revenue"];
	    }
	}
	export class SalespersonSalesSummary {
	    salespersonId: number;
	    salespersonName: string;
	    invoiceCount: number;
	    itemsSold: number;
	    grossAmount: number;
	    returnAmount: number;
	    netAmount: number;
	    discountAmount: number;
	    subtotalAmount: number;
	
	    static createFrom(source: any = {}) {
	        return new SalespersonSalesSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.salespersonId = source["salespersonId"];
	        this.salespersonName = source["salespersonName"];
	        this.invoiceCount = source["invoiceCount"];
	        this.itemsSold = source["itemsSold"];
	        this.grossAmount = source["grossAmount"];
	        this.returnAmount = source["returnAmount"];
	        this.netAmount = source["netAmount"];
	        this.discountAmount = source["discountAmount"];
	        this.subtotalAmount = source["subtotalAmount"];
	    }
	}
	export class ShiftBalanceReplacementPreview {
	    valid: boolean;
	    reason: string;
	    sourceRemainingCents: number;
	    destinationReceivesCents: number;
	
	    static createFrom(source: any = {}) {
	        return new ShiftBalanceReplacementPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.reason = source["reason"];
	        this.sourceRemainingCents = source["sourceRemainingCents"];
	        this.destinationReceivesCents = source["destinationReceivesCents"];
	    }
	}
	export class SyncFlushProgress {
	    total: number;
	    done: number;
	    failed: number;
	    remaining: number;
	    perSecond: number;
	    etaSeconds: number;
	    elapsedSeconds: number;
	    byType: SyncPendingByType[];
	    finished: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SyncFlushProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.done = source["done"];
	        this.failed = source["failed"];
	        this.remaining = source["remaining"];
	        this.perSecond = source["perSecond"];
	        this.etaSeconds = source["etaSeconds"];
	        this.elapsedSeconds = source["elapsedSeconds"];
	        this.byType = this.convertValues(source["byType"], SyncPendingByType);
	        this.finished = source["finished"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SyncStatusResult {
	    configured: boolean;
	    target: string;
	    pending: number;
	    failed: number;
	    confirmedOnTarget: number;
	    confirmedElsewhere: number;
	    lastSyncedAt: string;
	    lastReconcileAt: string;
	    lastFullCheckAt: string;
	    fullySynced: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SyncStatusResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.target = source["target"];
	        this.pending = source["pending"];
	        this.failed = source["failed"];
	        this.confirmedOnTarget = source["confirmedOnTarget"];
	        this.confirmedElsewhere = source["confirmedElsewhere"];
	        this.lastSyncedAt = source["lastSyncedAt"];
	        this.lastReconcileAt = source["lastReconcileAt"];
	        this.lastFullCheckAt = source["lastFullCheckAt"];
	        this.fullySynced = source["fullySynced"];
	    }
	}
	export class confirmTransferItemInput {
	    Barcode: string;
	    ProductUID: string;
	    ReceivedQuantity: number;
	
	    static createFrom(source: any = {}) {
	        return new confirmTransferItemInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Barcode = source["Barcode"];
	        this.ProductUID = source["ProductUID"];
	        this.ReceivedQuantity = source["ReceivedQuantity"];
	    }
	}
	export class transferRequestItemInput {
	    Barcode: string;
	    Quantity: number;
	
	    static createFrom(source: any = {}) {
	        return new transferRequestItemInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Barcode = source["Barcode"];
	        this.Quantity = source["Quantity"];
	    }
	}

}

