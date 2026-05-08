import type { Fleet, FleetMember, FleetMemberStatus } from './types';

export const activeFleetMemberStatus: FleetMemberStatus = 'active';
export const pendingFleetMemberStatus: FleetMemberStatus = 'pending';

export function normalizeNodeID(nodeID: string): string {
	return nodeID.trim().toLowerCase();
}

export function normalizeNodeKey(nodeKey: string): string {
	return nodeKey.trim().toLowerCase();
}

export function resolveFleetNodeID(existingFleet: Fleet | undefined, requestedNodeID: string, nodeKey: string): string {
	const fleet = normalizeFleet(existingFleet, '');
	const normalizedNodeKey = normalizeNodeKey(nodeKey);
	const existingMember = findFleetMemberByKey(fleet, normalizedNodeKey);
	if (existingMember) return existingMember.nodeID;
	return allocateFleetNodeID(fleet, requestedNodeID);
}

export function registerFleetNode(existingFleet: Fleet | undefined, fleetID: string, nodeID: string, nodeKey: string, now: Date, memberMetadata: Partial<FleetMember> = {}): Fleet {
	const normalizedNodeID = normalizeNodeID(nodeID);
	const normalizedNodeKey = normalizeNodeKey(nodeKey);
	const fleet = normalizeFleet(existingFleet, fleetID);
	if (fleet.members.some((member) => member.nodeID === normalizedNodeID)) {
		return promotePendingFleetMembers(updateFleetMember(fleet, normalizedNodeID, normalizedNodeKey, memberMetadata), now);
	}
	if (findFleetMemberByKey(fleet, normalizedNodeKey)) {
		return promotePendingFleetMembers(updateFleetMemberByKey(fleet, normalizedNodeKey, normalizedNodeID, memberMetadata), now);
	}
	const renumberedFleet = renumberSingleLegacyNodeToZero(fleet, normalizedNodeID, normalizedNodeKey, memberMetadata);
	if (renumberedFleet) return promotePendingFleetMembers(renumberedFleet, now);
	return promotePendingFleetMembers(
		{
			...fleet,
			members: [
				...fleet.members,
				{
					nodeID: normalizedNodeID,
					nodeKey: normalizedNodeKey,
					status: pendingFleetMemberStatus,
					joinedAt: now.toISOString(),
					...memberMetadata
				}
			]
		},
		now
	);
}

function renumberSingleLegacyNodeToZero(fleet: Fleet, nodeID: string, nodeKey: string, memberMetadata: Partial<FleetMember>): Fleet | undefined {
	const [member] = fleet.members;
	if (nodeID !== '0' || !member || fleet.members.length !== 1) return undefined;
	if (member.nodeID === '0' || !isNumericNodeID(member.nodeID)) return undefined;
	return {
		...fleet,
		members: [
			{
				...member,
				...memberMetadata,
				nodeID,
				nodeKey,
				status: member.status
			}
		]
	};
}

export function fleetMemberStatus(fleet: Fleet, nodeID: string): FleetMemberStatus {
	const normalizedNodeID = normalizeNodeID(nodeID);
	return fleet.members.find((member) => member.nodeID === normalizedNodeID)?.status ?? pendingFleetMemberStatus;
}

export function findFleetMember(fleet: Fleet | undefined, nodeID: string): FleetMember | undefined {
	const normalizedNodeID = normalizeNodeID(nodeID);
	return fleet?.members.find((member) => member.nodeID === normalizedNodeID);
}

export function activeFleetMembers(fleet: Fleet): FleetMember[] {
	return fleet.members.filter((member) => member.status === activeFleetMemberStatus);
}

export function pendingFleetMembers(fleet: Fleet): FleetMember[] {
	return fleet.members.filter((member) => member.status === pendingFleetMemberStatus);
}

export function fleetQuorumSize(fleet: Fleet): number {
	const activeCount = activeFleetMembers(fleet).length;
	return activeCount === 0 ? 0 : Math.floor(activeCount / 2) + 1;
}

export function normalizeFleet(existingFleet: Fleet | undefined, fleetID: string): Fleet {
	return {
		fleetID: existingFleet?.fleetID ?? fleetID,
		members: existingFleet?.members?.map(normalizeFleetMember).filter(isFleetMember) ?? [],
		workspaceHead: existingFleet?.workspaceHead,
		ledgerRevision: existingFleet?.ledgerRevision
	};
}

function normalizeFleetMember(member: FleetMember): FleetMember {
	const normalizedNodeID = normalizeNodeID(member.nodeID || '');
	const normalizedNodeKey = normalizeNodeKey(member.nodeKey || '');
	return {
		...member,
		nodeID: normalizedNodeID,
		nodeKey: normalizedNodeKey,
		status: member.status === activeFleetMemberStatus ? activeFleetMemberStatus : pendingFleetMemberStatus
	};
}

function updateFleetMember(fleet: Fleet, nodeID: string, nodeKey: string, memberMetadata: Partial<FleetMember>): Fleet {
	return {
		...fleet,
		members: fleet.members.map((member) =>
			member.nodeID === nodeID
				? {
						...member,
						...memberMetadata,
						nodeID,
						nodeKey,
						status: member.status
					}
				: member
		)
	};
}

function updateFleetMemberByKey(fleet: Fleet, nodeKey: string, nodeID: string, memberMetadata: Partial<FleetMember>): Fleet {
	return {
		...fleet,
		members: fleet.members.map((member) =>
			normalizeNodeKey(member.nodeKey ?? '') === nodeKey
				? {
						...member,
						...memberMetadata,
						nodeID,
						nodeKey,
						status: member.status
					}
				: member
		)
	};
}

function isFleetMember(member: FleetMember): boolean {
	return member.nodeID !== '';
}

function findFleetMemberByKey(fleet: Fleet, nodeKey: string): FleetMember | undefined {
	if (!nodeKey) return undefined;
	return fleet.members.find((member) => normalizeNodeKey(member.nodeKey ?? '') === nodeKey);
}

function allocateFleetNodeID(fleet: Fleet, requestedNodeID: string): string {
	const usedNodeIDs = new Set(fleet.members.map((member) => member.nodeID));
	const normalizedNodeID = normalizeNodeID(requestedNodeID);
	if (isNumericNodeID(normalizedNodeID) && !usedNodeIDs.has(normalizedNodeID)) {
		return normalizedNodeID;
	}
	for (let nodeNumber = 0; ; nodeNumber += 1) {
		const nodeID = String(nodeNumber);
		if (!usedNodeIDs.has(nodeID)) return nodeID;
	}
}

function isNumericNodeID(nodeID: string): boolean {
	return /^(0|[1-9][0-9]*)$/.test(nodeID);
}

function promotePendingFleetMembers(fleet: Fleet, now: Date): Fleet {
	let members = [...fleet.members];
	if (activeFleetMembers({ ...fleet, members }).length === 0) {
		members = activateOldestPendingMember(members, now);
	}
	while (pendingFleetMembers({ ...fleet, members }).length >= 2) {
		members = activateOldestPendingMember(activateOldestPendingMember(members, now), now);
	}
	return {
		...fleet,
		members
	};
}

function activateOldestPendingMember(members: FleetMember[], now: Date): FleetMember[] {
	const pendingIndex = members.findIndex((member) => member.status === pendingFleetMemberStatus);
	if (pendingIndex < 0) return members;
	return members.map((member, index) =>
		index === pendingIndex
			? {
					...member,
					status: activeFleetMemberStatus,
					activatedAt: member.activatedAt ?? now.toISOString()
				}
			: member
	);
}
