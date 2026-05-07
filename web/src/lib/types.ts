export interface Device {
	device_id: string;
	device_secret_hash?: string;
	tunnel_id: string;
	tunnel_token: string;
	dns_record_id: string;
	ssh_dns_record_id?: string;
	access_app_id?: string;
	access_policy_id?: string;
	ssh_access_app_id?: string;
	ssh_hostname?: string;
	admin_email: string;
	created_at: string;
	versions: {
		blueclaw: string;
		cli: string;
	};
	fleet?: Fleet;
}

export type FleetMemberStatus = 'active' | 'pending';

export interface FleetMember {
	boardID: string;
	boardKey?: string;
	status: FleetMemberStatus;
	joinedAt: string;
	activatedAt?: string;
	nodeTunnelID?: string;
	nodeTunnelToken?: string;
	sshDNSRecordID?: string;
	sshAccessAppID?: string;
	sshHostname?: string;
}

export interface Fleet {
	fleetID: string;
	members: FleetMember[];
	workspaceHead?: string;
	ledgerRevision?: number;
}

export type UserRole = 'admin' | 'member';

export interface UserRecord {
	userID: string;
	handle: string;
	name?: string;
	email: string;
	role: UserRole;
	mattermostUserID?: string;
	mattermostUsername?: string;
	status?: string;
	isIncomplete?: boolean;
}

export interface Invite {
	device_id: string;
	expires_at: number;
}

export interface OTAInfo {
	blueclaw: { version: string; url: string; sha256: string };
	cli: { version: string; url: string; sha256: string };
}
