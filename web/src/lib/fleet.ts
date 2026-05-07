import type { Fleet, FleetMember, FleetMemberStatus } from './types';

export const activeFleetMemberStatus: FleetMemberStatus = 'active';
export const pendingFleetMemberStatus: FleetMemberStatus = 'pending';

export function normalizeBoardID(boardID: string): string {
	return boardID.trim().toLowerCase();
}

export function normalizeBoardKey(boardKey: string): string {
	return boardKey.trim().toLowerCase();
}

export function resolveFleetBoardID(existingFleet: Fleet | undefined, requestedBoardID: string, boardKey: string): string {
	const fleet = normalizeFleet(existingFleet, '');
	const normalizedBoardKey = normalizeBoardKey(boardKey);
	const existingMember = findFleetMemberByKey(fleet, normalizedBoardKey);
	if (existingMember) return existingMember.boardID;
	return allocateFleetBoardID(fleet, requestedBoardID);
}

export function registerFleetBoard(existingFleet: Fleet | undefined, fleetID: string, boardID: string, boardKey: string, now: Date, memberMetadata: Partial<FleetMember> = {}): Fleet {
	const normalizedBoardID = normalizeBoardID(boardID);
	const normalizedBoardKey = normalizeBoardKey(boardKey);
	const fleet = normalizeFleet(existingFleet, fleetID);
	if (fleet.members.some((member) => member.boardID === normalizedBoardID)) {
		return promotePendingFleetMembers(updateFleetMember(fleet, normalizedBoardID, normalizedBoardKey, memberMetadata), now);
	}
	if (findFleetMemberByKey(fleet, normalizedBoardKey)) {
		return promotePendingFleetMembers(updateFleetMemberByKey(fleet, normalizedBoardKey, normalizedBoardID, memberMetadata), now);
	}
	return promotePendingFleetMembers(
		{
			...fleet,
			members: [
				...fleet.members,
				{
					boardID: normalizedBoardID,
					boardKey: normalizedBoardKey,
					status: pendingFleetMemberStatus,
					joinedAt: now.toISOString(),
					...memberMetadata
				}
			]
		},
		now
	);
}

export function fleetMemberStatus(fleet: Fleet, boardID: string): FleetMemberStatus {
	const normalizedBoardID = normalizeBoardID(boardID);
	return fleet.members.find((member) => member.boardID === normalizedBoardID)?.status ?? pendingFleetMemberStatus;
}

export function findFleetMember(fleet: Fleet | undefined, boardID: string): FleetMember | undefined {
	const normalizedBoardID = normalizeBoardID(boardID);
	return fleet?.members.find((member) => member.boardID === normalizedBoardID);
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

function normalizeFleet(existingFleet: Fleet | undefined, fleetID: string): Fleet {
	return {
		fleetID,
		members: existingFleet?.members?.map(normalizeFleetMember).filter(isFleetMember) ?? [],
		workspaceHead: existingFleet?.workspaceHead,
		ledgerRevision: existingFleet?.ledgerRevision
	};
}

function normalizeFleetMember(member: FleetMember): FleetMember {
	const normalizedBoardID = normalizeBoardID(member.boardID);
	return {
		...member,
		boardID: normalizedBoardID,
		boardKey: normalizeBoardKey(member.boardKey ?? ''),
		status: member.status === activeFleetMemberStatus ? activeFleetMemberStatus : pendingFleetMemberStatus
	};
}

function updateFleetMember(fleet: Fleet, boardID: string, boardKey: string, memberMetadata: Partial<FleetMember>): Fleet {
	return {
		...fleet,
		members: fleet.members.map((member) =>
			member.boardID === boardID
				? {
						...member,
						...memberMetadata,
						boardID,
						boardKey,
						status: member.status
					}
				: member
		)
	};
}

function updateFleetMemberByKey(fleet: Fleet, boardKey: string, boardID: string, memberMetadata: Partial<FleetMember>): Fleet {
	return {
		...fleet,
		members: fleet.members.map((member) =>
			normalizeBoardKey(member.boardKey ?? '') === boardKey
				? {
						...member,
						...memberMetadata,
						boardID,
						boardKey,
						status: member.status
					}
				: member
		)
	};
}

function isFleetMember(member: FleetMember): boolean {
	return member.boardID !== '';
}

function findFleetMemberByKey(fleet: Fleet, boardKey: string): FleetMember | undefined {
	if (!boardKey) return undefined;
	return fleet.members.find((member) => normalizeBoardKey(member.boardKey ?? '') === boardKey);
}

function allocateFleetBoardID(fleet: Fleet, requestedBoardID: string): string {
	const usedBoardIDs = new Set(fleet.members.map((member) => member.boardID));
	const normalizedBoardID = normalizeBoardID(requestedBoardID);
	if (isNumericBoardID(normalizedBoardID) && !usedBoardIDs.has(normalizedBoardID)) {
		return normalizedBoardID;
	}
	for (let boardNumber = 1; ; boardNumber += 1) {
		const boardID = String(boardNumber);
		if (!usedBoardIDs.has(boardID)) return boardID;
	}
}

function isNumericBoardID(boardID: string): boolean {
	return /^[1-9][0-9]*$/.test(boardID);
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
