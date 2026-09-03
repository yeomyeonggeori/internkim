import { getContext, setContext } from 'svelte';
import type { AttendanceText } from '../text';
import {
	cancelEmployeeLeaveRequest,
	createEmployeeLeaveRequest,
	fetchEmployeeLeave,
	previewEmployeeLeave
} from './employee-leave-api';
import { employeeLeaveErrorMessage } from './employee-leave-error';
import type {
	EmployeeLeavePayload,
	EmployeeLeavePreview,
	EmployeeLeavePreviewRequest,
	EmployeeLeaveSubmission
} from './employee-leave-types';

export class EmployeeLeaveState {
	payload = $state<EmployeeLeavePayload | null>(null);
	isLoading = $state(false);
	isMutating = $state(false);
	errorMessage = $state('');
	mutationErrorMessage = $state('');
	private loadSequence = 0;

	constructor(
		private readonly text: AttendanceText['leave'],
		private readonly onMutationCompleted: () => Promise<void> = async () => undefined
	) {}

	async load(): Promise<void> {
		this.errorMessage = '';
		try {
			await this.replacePayload();
		} catch (error) {
			this.errorMessage = this.localizedErrorMessage(error, this.text.loadFailed);
		}
	}

	preview(request: EmployeeLeavePreviewRequest): Promise<EmployeeLeavePreview> {
		return previewEmployeeLeave(request);
	}

	localizedErrorMessage(error: unknown, fallbackMessage: string): string {
		return employeeLeaveErrorMessage(error, this.text, fallbackMessage);
	}

	async create(request: EmployeeLeaveSubmission): Promise<void> {
		await this.mutate(() => createEmployeeLeaveRequest(request));
	}

	async cancel(requestID: string): Promise<void> {
		await this.mutate(() => cancelEmployeeLeaveRequest(requestID));
	}

	private async mutate(action: () => Promise<unknown>): Promise<void> {
		if (this.isMutating) return;
		this.isMutating = true;
		this.mutationErrorMessage = '';
		try {
			await action();
			await this.replacePayload();
			await this.onMutationCompleted();
		} catch (error) {
			this.mutationErrorMessage = this.localizedErrorMessage(error, this.text.mutationFailed);
			throw error;
		} finally {
			this.isMutating = false;
		}
	}

	private async replacePayload(): Promise<void> {
		const loadSequence = ++this.loadSequence;
		this.isLoading = true;
		try {
			const payload = await fetchEmployeeLeave();
			if (loadSequence !== this.loadSequence) return;
			this.payload = payload;
		} catch (error) {
			if (loadSequence === this.loadSequence) throw error;
		} finally {
			if (loadSequence === this.loadSequence) {
				this.isLoading = false;
			}
		}
	}
}

const employeeLeaveStateKey = Symbol('employee-leave-state');

export function setEmployeeLeaveState(state: EmployeeLeaveState): void {
	setContext(employeeLeaveStateKey, state);
}

export function getEmployeeLeaveState(): EmployeeLeaveState {
	const state = getContext<EmployeeLeaveState | undefined>(employeeLeaveStateKey);
	if (!state) throw new Error('EmployeeLeaveState not provided');
	return state;
}
