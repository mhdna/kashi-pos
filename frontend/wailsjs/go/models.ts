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
	
	    static createFrom(source: any = {}) {
	        return new LineQuote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.barcode = source["barcode"];
	        this.lineTotalCents = source["lineTotalCents"];
	        this.discountCents = source["discountCents"];
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
	export class ProductCursorPage {
	    items: any[];
	    nextCursor: number;
	    hasMore: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProductCursorPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = source["items"];
	        this.nextCursor = source["nextCursor"];
	        this.hasMore = source["hasMore"];
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

}

