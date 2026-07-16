export namespace main {
	
	export class SystemStats {
	    cpu_usage: number;
	    ram_usage_gb: number;
	
	    static createFrom(source: any = {}) {
	        return new SystemStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpu_usage = source["cpu_usage"];
	        this.ram_usage_gb = source["ram_usage_gb"];
	    }
	}

}


