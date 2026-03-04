export namespace main {
	
	export class BootstrapDTO {
	    addr: string;
	    device_id: string;
	
	    static createFrom(source: any = {}) {
	        return new BootstrapDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.addr = source["addr"];
	        this.device_id = source["device_id"];
	    }
	}
	export class StatusDTO {
	    work_dir: string;
	    connected: boolean;
	    addr: string;
	    reporting: boolean;
	    auth: runtime.AuthSnapshot;
	    metrics: Record<string, string>;
	    last_error: string;
	
	    static createFrom(source: any = {}) {
	        return new StatusDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.work_dir = source["work_dir"];
	        this.connected = source["connected"];
	        this.addr = source["addr"];
	        this.reporting = source["reporting"];
	        this.auth = this.convertValues(source["auth"], runtime.AuthSnapshot);
	        this.metrics = source["metrics"];
	        this.last_error = source["last_error"];
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

export namespace runtime {
	
	export class AuthSnapshot {
	    device_id?: string;
	    node_id?: number;
	    hub_id?: number;
	    role?: string;
	    logged_in: boolean;
	    last_action?: string;
	    last_message?: string;
	    last_unix_time?: number;
	
	    static createFrom(source: any = {}) {
	        return new AuthSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_id = source["device_id"];
	        this.node_id = source["node_id"];
	        this.hub_id = source["hub_id"];
	        this.role = source["role"];
	        this.logged_in = source["logged_in"];
	        this.last_action = source["last_action"];
	        this.last_message = source["last_message"];
	        this.last_unix_time = source["last_unix_time"];
	    }
	}
	export class MetricSetting {
	    metric: string;
	    var_name: string;
	    enabled: boolean;
	    writable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MetricSetting(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.metric = source["metric"];
	        this.var_name = source["var_name"];
	        this.enabled = source["enabled"];
	        this.writable = source["writable"];
	    }
	}

}

