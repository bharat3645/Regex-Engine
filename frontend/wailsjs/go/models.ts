export namespace config {
	
	export class Config {
	    root_dir: string;
	    rules_dir: string;
	    output_file: string;
	    log_file: string;
	    num_workers: number;
	    num_python_workers: number;
	    num_text_workers: number;
	    log_level: string;
	    max_cpu_percentage: number;
	    pipeline_buffer_size: number;
	    scanner_buffer_size_mb: number;
	    discovery_batch_size: number;
	    python_batch_size: number;
	    python_batch_timeout_ms: number;
	    gc_trigger_mb: number;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root_dir = source["root_dir"];
	        this.rules_dir = source["rules_dir"];
	        this.output_file = source["output_file"];
	        this.log_file = source["log_file"];
	        this.num_workers = source["num_workers"];
	        this.num_python_workers = source["num_python_workers"];
	        this.num_text_workers = source["num_text_workers"];
	        this.log_level = source["log_level"];
	        this.max_cpu_percentage = source["max_cpu_percentage"];
	        this.pipeline_buffer_size = source["pipeline_buffer_size"];
	        this.scanner_buffer_size_mb = source["scanner_buffer_size_mb"];
	        this.discovery_batch_size = source["discovery_batch_size"];
	        this.python_batch_size = source["python_batch_size"];
	        this.python_batch_timeout_ms = source["python_batch_timeout_ms"];
	        this.gc_trigger_mb = source["gc_trigger_mb"];
	    }
	}

}

export namespace stats {
	
	export class SystemStats {
	    cpu_usage: number;
	    ram_usage_gb: number;
	    disk_read_mbps: number;
	    disk_write_mbps: number;
	    queue_depth: number;
	
	    static createFrom(source: any = {}) {
	        return new SystemStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpu_usage = source["cpu_usage"];
	        this.ram_usage_gb = source["ram_usage_gb"];
	        this.disk_read_mbps = source["disk_read_mbps"];
	        this.disk_write_mbps = source["disk_write_mbps"];
	        this.queue_depth = source["queue_depth"];
	    }
	}

}

