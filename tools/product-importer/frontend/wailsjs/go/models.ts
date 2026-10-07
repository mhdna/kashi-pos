export namespace main {
	
	export class ProductInput {
	    barcode: string;
	    code: string;
	    name: string;
	    description: string;
	    color: string;
	    size: string;
	    brand: string;
	    family: string;
	    kind: string;
	    season: string;
	    price: string;
	    discount: number;
	    year: number;
	    isActive: boolean;
	    quickAccess: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProductInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.barcode = source["barcode"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.color = source["color"];
	        this.size = source["size"];
	        this.brand = source["brand"];
	        this.family = source["family"];
	        this.kind = source["kind"];
	        this.season = source["season"];
	        this.price = source["price"];
	        this.discount = source["discount"];
	        this.year = source["year"];
	        this.isActive = source["isActive"];
	        this.quickAccess = source["quickAccess"];
	    }
	}
	export class CSVRow {
	    file: string;
	    line: number;
	    input: ProductInput;
	
	    static createFrom(source: any = {}) {
	        return new CSVRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.line = source["line"];
	        this.input = this.convertValues(source["input"], ProductInput);
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
	export class RowResult {
	    file?: string;
	    index: number;
	    barcode: string;
	    status: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new RowResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file = source["file"];
	        this.index = source["index"];
	        this.barcode = source["barcode"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class CSVPreview {
	    files: string[];
	    total: number;
	    new: number;
	    existing: number;
	    invalid: number;
	    problems: RowResult[];
	    sample: CSVRow[];
	
	    static createFrom(source: any = {}) {
	        return new CSVPreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = source["files"];
	        this.total = source["total"];
	        this.new = source["new"];
	        this.existing = source["existing"];
	        this.invalid = source["invalid"];
	        this.problems = this.convertValues(source["problems"], RowResult);
	        this.sample = this.convertValues(source["sample"], CSVRow);
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
	
	export class DatabaseInfo {
	    brands: string[];
	    families: string[];
	    kinds: string[];
	    seasons: string[];
	    colors: string[];
	    sizes: string[];
	    priceList: string;
	    discountList: string;
	    listPricing: boolean;
	    connected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DatabaseInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.brands = source["brands"];
	        this.families = source["families"];
	        this.kinds = source["kinds"];
	        this.seasons = source["seasons"];
	        this.colors = source["colors"];
	        this.sizes = source["sizes"];
	        this.priceList = source["priceList"];
	        this.discountList = source["discountList"];
	        this.listPricing = source["listPricing"];
	        this.connected = source["connected"];
	    }
	}
	export class FixResult {
	    priceList: string;
	    createdList: boolean;
	    priced: number;
	    discounted: number;
	    createdDiscountList: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FixResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.priceList = source["priceList"];
	        this.createdList = source["createdList"];
	        this.priced = source["priced"];
	        this.discounted = source["discounted"];
	        this.createdDiscountList = source["createdDiscountList"];
	    }
	}
	export class ImportResult {
	    imported: number;
	    failed: number;
	    rows: RowResult[];
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = source["imported"];
	        this.failed = source["failed"];
	        this.rows = this.convertValues(source["rows"], RowResult);
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
	export class PriceStatus {
	    listPricing: boolean;
	    connected: boolean;
	    priceList: string;
	    missing: number;
	    fixable: number;
	    noPrice: number;
	    discounts: number;
	
	    static createFrom(source: any = {}) {
	        return new PriceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.listPricing = source["listPricing"];
	        this.connected = source["connected"];
	        this.priceList = source["priceList"];
	        this.missing = source["missing"];
	        this.fixable = source["fixable"];
	        this.noPrice = source["noPrice"];
	        this.discounts = source["discounts"];
	    }
	}
	
	export class QuickItem {
	    id: number;
	    name: string;
	    code: string;
	    barcode: string;
	    color: string;
	    size: string;
	    isActive: boolean;
	    cardColor: string;
	    cardTextColor: string;
	
	    static createFrom(source: any = {}) {
	        return new QuickItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.code = source["code"];
	        this.barcode = source["barcode"];
	        this.color = source["color"];
	        this.size = source["size"];
	        this.isActive = source["isActive"];
	        this.cardColor = source["cardColor"];
	        this.cardTextColor = source["cardTextColor"];
	    }
	}
	export class QuickAccessData {
	    items: QuickItem[];
	    cardColors: boolean;
	    textColors: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QuickAccessData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], QuickItem);
	        this.cardColors = source["cardColors"];
	        this.textColors = source["textColors"];
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
	

}

