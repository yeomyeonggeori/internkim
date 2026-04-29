export interface Device {
	device_id: string;
	device_secret_hash?: string;
	tunnel_id: string;
	tunnel_token: string;
	dns_record_id: string;
	access_app_id?: string;
	access_policy_id?: string;
	admin_email: string;
	created_at: string;
	versions: {
		blueclaw: string;
		cli: string;
	};
}

export type UserRole = 'admin' | 'member';

export interface UserRecord {
	email: string;
	role: UserRole;
	mattermostUserID?: string;
	mattermostUsername?: string;
	status?: string;
}

export interface Invite {
	device_id: string;
	expires_at: number;
}

export interface OTAInfo {
	blueclaw: { version: string; url: string; sha256: string };
	cli: { version: string; url: string; sha256: string };
}
