export namespace main {
	
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

}

