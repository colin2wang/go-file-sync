export namespace config {
	
	export class Filters {
	    exclude: string[];
	    include: string[];
	    caseSensitive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Filters(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exclude = source["exclude"];
	        this.include = source["include"];
	        this.caseSensitive = source["caseSensitive"];
	    }
	}
	export class Schedule {
	    enabled: boolean;
	    type: string;
	    intervalMinutes: number;
	    dailyTime: string;
	
	    static createFrom(source: any = {}) {
	        return new Schedule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.type = source["type"];
	        this.intervalMinutes = source["intervalMinutes"];
	        this.dailyTime = source["dailyTime"];
	    }
	}
	export class TaskConfig {
	    id: string;
	    name: string;
	    enabled: boolean;
	    left: string;
	    right: string;
	    mode: string;
	    incremental: boolean;
	    mirror: boolean;
	    conflict: string;
	    backupDeleted: boolean;
	    filters: Filters;
	    schedule: Schedule;
	    // Go type: time
	    created: any;
	    // Go type: time
	    updated: any;
	
	    static createFrom(source: any = {}) {
	        return new TaskConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.left = source["left"];
	        this.right = source["right"];
	        this.mode = source["mode"];
	        this.incremental = source["incremental"];
	        this.mirror = source["mirror"];
	        this.conflict = source["conflict"];
	        this.backupDeleted = source["backupDeleted"];
	        this.filters = this.convertValues(source["filters"], Filters);
	        this.schedule = this.convertValues(source["schedule"], Schedule);
	        this.created = this.convertValues(source["created"], null);
	        this.updated = this.convertValues(source["updated"], null);
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

export namespace engine {
	
	export class Action {
	    type: string;
	    path: string;
	    reason: string;
	    size?: number;
	    leftM?: number;
	    rightM?: number;
	
	    static createFrom(source: any = {}) {
	        return new Action(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.path = source["path"];
	        this.reason = source["reason"];
	        this.size = source["size"];
	        this.leftM = source["leftM"];
	        this.rightM = source["rightM"];
	    }
	}
	export class PlanStats {
	    copyL2R: number;
	    copyR2L: number;
	    delLeft: number;
	    delRight: number;
	    conflict: number;
	    skip: number;
	    unchanged: number;
	    totalBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new PlanStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.copyL2R = source["copyL2R"];
	        this.copyR2L = source["copyR2L"];
	        this.delLeft = source["delLeft"];
	        this.delRight = source["delRight"];
	        this.conflict = source["conflict"];
	        this.skip = source["skip"];
	        this.unchanged = source["unchanged"];
	        this.totalBytes = source["totalBytes"];
	    }
	}

}

export namespace main {
	
	export class PreviewResult {
	    actions: engine.Action[];
	    stats: engine.PlanStats;
	    truncated: boolean;
	    leftMissing: boolean;
	    rightMissing: boolean;
	    hasPrevState: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PreviewResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.actions = this.convertValues(source["actions"], engine.Action);
	        this.stats = this.convertValues(source["stats"], engine.PlanStats);
	        this.truncated = source["truncated"];
	        this.leftMissing = source["leftMissing"];
	        this.rightMissing = source["rightMissing"];
	        this.hasPrevState = source["hasPrevState"];
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
	export class Settings {
	    language: string;
	    theme: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.theme = source["theme"];
	    }
	}
	export class TaskSummary {
	    id: string;
	    name: string;
	    enabled: boolean;
	    left: string;
	    right: string;
	    mode: string;
	    incremental: boolean;
	    mirror: boolean;
	    conflict: string;
	    backupDeleted: boolean;
	    filters: config.Filters;
	    schedule: config.Schedule;
	    // Go type: time
	    created: any;
	    // Go type: time
	    updated: any;
	    // Go type: time
	    nextRun: any;
	    // Go type: time
	    lastRun: any;
	    lastResult: state.RunSummary;
	    running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TaskSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.left = source["left"];
	        this.right = source["right"];
	        this.mode = source["mode"];
	        this.incremental = source["incremental"];
	        this.mirror = source["mirror"];
	        this.conflict = source["conflict"];
	        this.backupDeleted = source["backupDeleted"];
	        this.filters = this.convertValues(source["filters"], config.Filters);
	        this.schedule = this.convertValues(source["schedule"], config.Schedule);
	        this.created = this.convertValues(source["created"], null);
	        this.updated = this.convertValues(source["updated"], null);
	        this.nextRun = this.convertValues(source["nextRun"], null);
	        this.lastRun = this.convertValues(source["lastRun"], null);
	        this.lastResult = this.convertValues(source["lastResult"], state.RunSummary);
	        this.running = source["running"];
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

export namespace state {
	
	export class RunSummary {
	    status: string;
	    copied: number;
	    deleted: number;
	    skipped: number;
	    conflicts: number;
	    failed: number;
	    bytes: number;
	    message?: string;
	    // Go type: time
	    finished: any;
	
	    static createFrom(source: any = {}) {
	        return new RunSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.copied = source["copied"];
	        this.deleted = source["deleted"];
	        this.skipped = source["skipped"];
	        this.conflicts = source["conflicts"];
	        this.failed = source["failed"];
	        this.bytes = source["bytes"];
	        this.message = source["message"];
	        this.finished = this.convertValues(source["finished"], null);
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

