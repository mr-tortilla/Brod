export namespace main {

	export class ConnectResult {
	    profile: store.Profile;
	    nodeId: string;
	    refreshError?: string;

	    static createFrom(source: any = {}) {
	        return new ConnectResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], store.Profile);
	        this.nodeId = source["nodeId"];
	        this.refreshError = source["refreshError"];
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
	export class CreateResult {
	    profile: store.Profile;
	    skipped: number;

	    static createFrom(source: any = {}) {
	        return new CreateResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], store.Profile);
	        this.skipped = source["skipped"];
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
	export class SaveSettingsResult {
	    invalid: string[];
	    reconnected: boolean;

	    static createFrom(source: any = {}) {
	        return new SaveSettingsResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.invalid = source["invalid"];
	        this.reconnected = source["reconnected"];
	    }
	}
	export class StartupAction {
	    profileId: string;
	    nodeId: string;
	    mode: string;

	    static createFrom(source: any = {}) {
	        return new StartupAction(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileId = source["profileId"];
	        this.nodeId = source["nodeId"];
	        this.mode = source["mode"];
	    }
	}

}

export namespace node {

	export class Node {
	    id: string;
	    protocol?: string;
	    name: string;
	    server: string;
	    port: number;
	    uuid?: string;
	    password?: string;
	    flow?: string;
	    network: string;
	    security: string;
	    sni?: string;
	    fingerprint?: string;
	    publicKey?: string;
	    shortId?: string;
	    alpn?: string[];
	    path?: string;
	    host?: string;
	    serviceName?: string;
	    upMbps?: number;
	    downMbps?: number;
	    obfsType?: string;
	    obfsPassword?: string;
	    congestionControl?: string;
	    udpRelayMode?: string;
	    insecure?: boolean;

	    static createFrom(source: any = {}) {
	        return new Node(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.protocol = source["protocol"];
	        this.name = source["name"];
	        this.server = source["server"];
	        this.port = source["port"];
	        this.uuid = source["uuid"];
	        this.password = source["password"];
	        this.flow = source["flow"];
	        this.network = source["network"];
	        this.security = source["security"];
	        this.sni = source["sni"];
	        this.fingerprint = source["fingerprint"];
	        this.publicKey = source["publicKey"];
	        this.shortId = source["shortId"];
	        this.alpn = source["alpn"];
	        this.path = source["path"];
	        this.host = source["host"];
	        this.serviceName = source["serviceName"];
	        this.upMbps = source["upMbps"];
	        this.downMbps = source["downMbps"];
	        this.obfsType = source["obfsType"];
	        this.obfsPassword = source["obfsPassword"];
	        this.congestionControl = source["congestionControl"];
	        this.udpRelayMode = source["udpRelayMode"];
	        this.insecure = source["insecure"];
	    }
	}

}

export namespace share {

	export class Data {
	    title: string;
	    link: string;
	    linkKind: string;
	    json: string;
	    qr: string;
	    qrError?: string;
	    skipped: number;

	    static createFrom(source: any = {}) {
	        return new Data(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.link = source["link"];
	        this.linkKind = source["linkKind"];
	        this.json = source["json"];
	        this.qr = source["qr"];
	        this.qrError = source["qrError"];
	        this.skipped = source["skipped"];
	    }
	}

}

export namespace store {

	export class Profile {
	    id: string;
	    name: string;
	    source: string;
	    url?: string;
	    nodes: node.Node[];
	    pinned: string[];
	    // Go type: time
	    updatedAt: any;
	    sub?: subscription.Info;

	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.source = source["source"];
	        this.url = source["url"];
	        this.nodes = this.convertValues(source["nodes"], node.Node);
	        this.pinned = source["pinned"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.sub = this.convertValues(source["sub"], subscription.Info);
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
	    mixedPort: number;
	    allowLan: boolean;
	    bypassLan: boolean;
	    refreshOnConnect: boolean;
	    testUrl: string;
	    systemProxyInTun: boolean;
	    tunStack: string;
	    tunMtu: number;
	    strictRoute: boolean;
	    directDns: string;
	    remoteDns: string;
	    ipv6: string;
	    bypass: string[];
	    theme: string;

	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mixedPort = source["mixedPort"];
	        this.allowLan = source["allowLan"];
	        this.bypassLan = source["bypassLan"];
	        this.refreshOnConnect = source["refreshOnConnect"];
	        this.testUrl = source["testUrl"];
	        this.systemProxyInTun = source["systemProxyInTun"];
	        this.tunStack = source["tunStack"];
	        this.tunMtu = source["tunMtu"];
	        this.strictRoute = source["strictRoute"];
	        this.directDns = source["directDns"];
	        this.remoteDns = source["remoteDns"];
	        this.ipv6 = source["ipv6"];
	        this.bypass = source["bypass"];
	        this.theme = source["theme"];
	    }
	}

}

export namespace subscription {

	export class Info {
	    upload: number;
	    download: number;
	    total: number;
	    expire: number;

	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.upload = source["upload"];
	        this.download = source["download"];
	        this.total = source["total"];
	        this.expire = source["expire"];
	    }
	}

}

export namespace tunnel {

	export class Status {
	    state: string;
	    mode: string;
	    profileId: string;
	    nodeId: string;
	    nodeName: string;
	    server: string;
	    since: number;
	    upSpeed: number;
	    downSpeed: number;
	    upTotal: number;
	    downTotal: number;
	    latencyMs: number;
	    ip: string;
	    countryCode: string;
	    city: string;
	    org: string;

	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.mode = source["mode"];
	        this.profileId = source["profileId"];
	        this.nodeId = source["nodeId"];
	        this.nodeName = source["nodeName"];
	        this.server = source["server"];
	        this.since = source["since"];
	        this.upSpeed = source["upSpeed"];
	        this.downSpeed = source["downSpeed"];
	        this.upTotal = source["upTotal"];
	        this.downTotal = source["downTotal"];
	        this.latencyMs = source["latencyMs"];
	        this.ip = source["ip"];
	        this.countryCode = source["countryCode"];
	        this.city = source["city"];
	        this.org = source["org"];
	    }
	}

}

