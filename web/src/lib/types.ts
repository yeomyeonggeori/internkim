import type { MemberRole, MemberStatus } from '$lib/member-vocabulary';

export interface Device {
	fleet_id: string;
	fleet_secret_hash?: string;
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
	nodeID: string;
	nodeKey?: string;
	status: FleetMemberStatus;
	joinedAt: string;
	activatedAt?: string;
}

export interface Fleet {
	fleetID: string;
	members: FleetMember[];
	workspaceHead?: string;
	ledgerRevision?: number;
}

export type UserRole = MemberRole;

export interface Invite {
	fleet_id: string;
	expires_at: number;
}

export interface OTAInfo {
	blueclaw: { version: string; url: string; sha256: string };
	cli: { version: string; url: string; sha256: string };
}
