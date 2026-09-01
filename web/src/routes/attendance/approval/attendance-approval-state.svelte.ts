import { getContext, setContext } from 'svelte';
import {
	decideSupabaseApproval,
	supabasePendingApprovals,
	withdrawSupabaseApproval
} from '$lib/attendance/supabase-approval';
import { isSupabaseConfigured } from '$lib/supabase';
import type { AttendanceText } from '../text';
import type { AttendanceApprovalDecision, AttendanceApprovalRequest } from './attendance-approval-types';

export class AttendanceApprovalState {
	requests = $state<AttendanceApprovalRequest[]>([]);
	isLoading = $state(false);
	isMutating = $state(false);
	errorMessage = $state('');
	mutationErrorMessage = $state('');
	private loadSequence = 0;

	constructor(
		private readonly text: AttendanceText['approval'],
		private readonly onMutationCompleted: () => Promise<void> = async () => undefined
	) {}

	requestsFrom(memberID: string | undefined): AttendanceApprovalRequest[] {
		if (!memberID) return [];
		return this.requests.filter((request) => request.memberID === memberID);
	}

	async load(): Promise<void> {
		if (!isSupabaseConfigured()) return;
		this.errorMessage = '';
		const loadSequence = ++this.loadSequence;
		this.isLoading = true;
		try {
			const requests = await supabasePendingApprovals();
			if (loadSequence !== this.loadSequence) return;
			this.requests = requests;
		} catch {
			if (loadSequence === this.loadSequence) this.errorMessage = this.text.attendanceLoadFailed;
		} finally {
			if (loadSequence === this.loadSequence) this.isLoading = false;
		}
	}

	async decide(
		approvalID: string,
		decision: AttendanceApprovalDecision,
		note: string
	): Promise<void> {
		await this.mutate(() => decideSupabaseApproval(approvalID, decision, note));
	}

	async withdraw(approvalID: string): Promise<void> {
		await this.mutate(() => withdrawSupabaseApproval(approvalID));
	}

	clear(): void {
		this.loadSequence += 1;
		this.requests = [];
		this.isLoading = false;
		this.errorMessage = '';
		this.mutationErrorMessage = '';
	}

	private async mutate(change: () => Promise<void>): Promise<void> {
		if (this.isMutating) return;
		this.isMutating = true;
		this.mutationErrorMessage = '';
		try {
			await change();
			await this.load();
			await this.onMutationCompleted();
		} catch (failure) {
			this.mutationErrorMessage =
				failure instanceof Error ? failure.message : this.text.attendanceProcessingFailed;
			throw failure;
		} finally {
			this.isMutating = false;
		}
	}
}

const attendanceApprovalStateKey = Symbol('attendance-approval-state');

export function setAttendanceApprovalState(state: AttendanceApprovalState): void {
	setContext(attendanceApprovalStateKey, state);
}

export function getAttendanceApprovalState(): AttendanceApprovalState {
	const state = getContext<AttendanceApprovalState | undefined>(attendanceApprovalStateKey);
	if (!state) throw new Error('AttendanceApprovalState not provided');
	return state;
}
