export interface Device {
	device_id: string;
	tunnel_id: string;
	tunnel_token: string;
	dns_record_id: string;
	admin_email: string;
	created_at: string;
	versions: {
		picoclaw: string;
		cli: string;
	};
}

export interface Invite {
	device_id: string;
	expires_at: number;
}

export interface OTAInfo {
	picoclaw: { version: string; url: string; sha256: string };
	cli: { version: string; url: string; sha256: string };
}
