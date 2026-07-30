import { getContext, setContext } from 'svelte';
import type { AttendanceText } from '../text';
import { employeeLeaveErrorMessage } from '../leave/employee-leave-error';
import {
	adjustManagedLeave,
	cancelManagedLeaveRequest,
	correctManagedLeaveTime,
	createManagedPastLeave,
	fetchLeaveManagement
} from './leave-management-api';
import type {
	LeaveManagementAdjustment,
	LeaveManagementPastLeave,
	LeaveManagementPayload,
	LeaveManagementTimeCorrection
} from './leave-management-types';

export class LeaveManagementState {
	payload = $state<LeaveManagementPayload | null>(null);
	selectedEmployeeEmail = $state('');
	isLoading = $state(false);
	isMutating = $state(false);
	errorMessage = $state('');
	private loadSequence = 0;

	constructor(
		private readonly text: AttendanceText['management'],
		private readonly onMutationCompleted: () => Promise<void> = async () => undefined
	) {}

	async load(employeeEmail = this.selectedEmployeeEmail): Promise<void> {
		const loadSequence = ++this.loadSequence;
		this.isLoading = true;
		this.errorMessage = '';
		try {
			const payload = await fetchLeaveManagement(employeeEmail);
			if (loadSequence !== this.loadSequence) return;
			this.payload = payload;
			if (employeeEmail && payload.detail?.employee.email === employeeEmail) {
				this.selectedEmployeeEmail = employeeEmail;
			}
		} catch {
			if (loadSequence !== this.loadSequence) return;
			this.errorMessage = this.text.loadFailed;
		} finally {
			if (loadSequence === this.loadSequence) {
				this.isLoading = false;
			}
		}
	}

	async selectEmployee(employeeEmail: string): Promise<void> {
		await this.load(employeeEmail);
	}

	async adjust(input: LeaveManagementAdjustment): Promise<void> {
		await this.mutate(() => adjustManagedLeave(input));
	}

	async addPastLeave(input: LeaveManagementPastLeave): Promise<void> {
		await this.mutate(() => createManagedPastLeave(input));
	}

	async cancelRequest(requestID: string): Promise<void> {
		if (!this.selectedEmployeeEmail) return;
		await this.mutate(() => cancelManagedLeaveRequest(requestID, this.selectedEmployeeEmail));
	}

	async correctTime(
		requestID: string,
		input: Omit<LeaveManagementTimeCorrection, 'employeeEmail'>
	): Promise<void> {
		if (!this.selectedEmployeeEmail) return;
		await this.mutate(() =>
			correctManagedLeaveTime(requestID, {
				...input,
				employeeEmail: this.selectedEmployeeEmail
			})
		);
	}

	clear(): void {
		this.loadSequence += 1;
		this.payload = null;
		this.selectedEmployeeEmail = '';
		this.isLoading = false;
		this.errorMessage = '';
	}

	private async mutate(action: () => Promise<void>): Promise<void> {
		if (this.isMutating) return;
		this.isMutating = true;
		this.errorMessage = '';
		try {
			await action();
			await this.load(this.selectedEmployeeEmail);
			await this.onMutationCompleted();
		} catch (error) {
			this.errorMessage = employeeLeaveErrorMessage(
				error,
				this.text,
				this.text.processingFailed
			);
			throw error;
		} finally {
			this.isMutating = false;
		}
	}
}

const leaveManagementStateKey = Symbol('leave-management-state');

export function setLeaveManagementState(state: LeaveManagementState): void {
	setContext(leaveManagementStateKey, state);
}

export function getLeaveManagementState(): LeaveManagementState {
	const state = getContext<LeaveManagementState | undefined>(leaveManagementStateKey);
	if (!state) throw new Error('LeaveManagementState not provided');
	return state;
}
