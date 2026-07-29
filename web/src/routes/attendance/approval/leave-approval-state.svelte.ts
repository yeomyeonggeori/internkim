import { getContext, setContext } from 'svelte';
import type { AttendanceText } from '../text';
import { decideLeaveApproval, fetchLeaveApprovalInbox } from './leave-approval-api';
import type { LeaveApprovalDecision, LeaveApprovalInbox } from './leave-approval-types';

export class LeaveApprovalState {
	inbox = $state<LeaveApprovalInbox | null>(null);
	isLoading = $state(false);
	isMutating = $state(false);
	errorMessage = $state('');
	mutationErrorMessage = $state('');
	private loadSequence = 0;

	constructor(
		private readonly text: AttendanceText['approval'],
		private readonly onMutationCompleted: () => Promise<void> = async () => undefined
	) {}

	async load(): Promise<void> {
		this.errorMessage = '';
		try {
			await this.replaceInbox();
		} catch {
			this.errorMessage = this.text.loadFailed;
		}
	}

	async decide(requestID: string, decision: LeaveApprovalDecision): Promise<void> {
		if (this.isMutating) return;
		this.isMutating = true;
		this.mutationErrorMessage = '';
		try {
			await decideLeaveApproval(requestID, decision);
			await this.replaceInbox();
			await this.onMutationCompleted();
		} catch (error) {
			this.mutationErrorMessage = this.text.processingFailed;
			throw error;
		} finally {
			this.isMutating = false;
		}
	}

	clear(): void {
		this.loadSequence += 1;
		this.inbox = null;
		this.isLoading = false;
		this.errorMessage = '';
		this.mutationErrorMessage = '';
	}

	private async replaceInbox(): Promise<void> {
		const loadSequence = ++this.loadSequence;
		this.isLoading = true;
		try {
			const inbox = await fetchLeaveApprovalInbox();
			if (loadSequence !== this.loadSequence) return;
			this.inbox = inbox;
		} catch (error) {
			if (loadSequence === this.loadSequence) throw error;
		} finally {
			if (loadSequence === this.loadSequence) {
				this.isLoading = false;
			}
		}
	}
}

const leaveApprovalStateKey = Symbol('leave-approval-state');

export function setLeaveApprovalState(state: LeaveApprovalState): void {
	setContext(leaveApprovalStateKey, state);
}

export function getLeaveApprovalState(): LeaveApprovalState {
	const state = getContext<LeaveApprovalState | undefined>(leaveApprovalStateKey);
	if (!state) throw new Error('LeaveApprovalState not provided');
	return state;
}
