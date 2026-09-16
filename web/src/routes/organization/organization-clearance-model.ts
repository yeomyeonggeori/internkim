import type { UserRecord } from '$lib/organization/types';

export const dataRoomClearances = [1, 2, 3] as const;

export type DataRoomClearance = (typeof dataRoomClearances)[number];

export type DataRoomClearanceWords = {
	clearanceMember: string;
	clearanceManagement: string;
	clearanceRepresentative: string;
};

const lowestDataRoomClearance: DataRoomClearance = 1;

export function dataRoomClearanceOf(record: UserRecord | undefined): number {
	return record?.clearance ?? lowestDataRoomClearance;
}

export function canEditDataRoomClearance(viewer: UserRecord | undefined, target: UserRecord | undefined): boolean {
	if (!viewer || !target) return false;
	if (viewer.role !== 'admin') return false;
	if (viewer.memberID === target.memberID) return true;
	return dataRoomClearanceOf(target) < dataRoomClearanceOf(viewer);
}

export function offeredDataRoomClearances(viewer: UserRecord | undefined): DataRoomClearance[] {
	return dataRoomClearances.filter((clearance) => clearance <= dataRoomClearanceOf(viewer));
}

export function dataRoomClearanceLabel(clearance: number, words: DataRoomClearanceWords): string {
	const word = dataRoomClearanceWord(clearance, words);
	return word ? `${clearance} ${word}` : String(clearance);
}

function dataRoomClearanceWord(clearance: number, words: DataRoomClearanceWords): string {
	if (clearance === 1) return words.clearanceMember;
	if (clearance === 2) return words.clearanceManagement;
	if (clearance === 3) return words.clearanceRepresentative;
	return '';
}
