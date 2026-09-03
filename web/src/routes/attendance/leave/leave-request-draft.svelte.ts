import type {
	EmployeeLeavePartialPeriod,
	EmployeeLeavePreviewRequest,
	EmployeeLeaveSubmission,
	EmployeeLeaveType,
	EmployeeLeaveUnit
} from './employee-leave-types';

export class LeaveRequestDraft {
	leaveTypeID = $state('');
	unit = $state<EmployeeLeaveUnit>('fullDay');
	startDate = $state('');
	endDate = $state('');
	partialPeriod = $state<EmployeeLeavePartialPeriod>('morning');
	startTime = $state('');
	reason = $state('');

	synchronize(leaveTypes: EmployeeLeaveType[], defaultDate: string): void {
		const activeTypes = leaveTypes.filter((leaveType) => leaveType.isActive);
		const selectedType = leaveTypes.find((leaveType) => leaveType.id === this.leaveTypeID);
		if (selectedType?.isActive !== true) {
			this.leaveTypeID = activeTypes[0]?.id ?? '';
		}
		const synchronizedType = leaveTypes.find((leaveType) => leaveType.id === this.leaveTypeID);
		if (synchronizedType && !synchronizedType.allowedUnits.includes(this.unit)) {
			this.unit = synchronizedType.allowedUnits[0] ?? 'fullDay';
		}
		if (!this.startDate) this.startDate = defaultDate;
		if (!this.endDate) this.endDate = this.startDate;
	}

	setLeaveType(leaveType: EmployeeLeaveType): void {
		this.leaveTypeID = leaveType.id;
		if (!leaveType.allowedUnits.includes(this.unit)) {
			this.setUnit(leaveType.allowedUnits[0] ?? 'fullDay');
		}
	}

	setUnit(unit: EmployeeLeaveUnit): void {
		this.unit = unit;
		if (unit === 'fullDay') {
			this.endDate = this.endDate || this.startDate;
			return;
		}
		this.endDate = '';
		if (unit === 'quarterDay') {
			this.partialPeriod = 'custom';
		}
	}

	setStartDate(startDate: string): void {
		this.startDate = startDate;
		if (this.unit === 'fullDay' && (!this.endDate || this.endDate < startDate)) {
			this.endDate = startDate;
		}
	}

	reset(leaveTypes: EmployeeLeaveType[], defaultDate: string): void {
		this.leaveTypeID = '';
		this.unit = 'fullDay';
		this.startDate = defaultDate;
		this.endDate = defaultDate;
		this.partialPeriod = 'morning';
		this.startTime = '';
		this.reason = '';
		this.synchronize(leaveTypes, defaultDate);
	}

	previewRequest(): EmployeeLeavePreviewRequest | null {
		if (!this.leaveTypeID || !this.startDate) return null;
		if (this.unit === 'fullDay' && !this.endDate) return null;
		if (this.unit !== 'fullDay' && this.partialPeriod === 'custom' && !this.startTime) return null;
		return {
			leaveTypeID: this.leaveTypeID,
			unit: this.unit,
			startDate: this.startDate,
			...(this.unit === 'fullDay' ? { endDate: this.endDate } : {}),
			...(this.unit !== 'fullDay' ? { partialPeriod: this.partialPeriod } : {}),
			...(this.unit !== 'fullDay' && this.partialPeriod === 'custom'
				? { startTime: this.startTime }
				: {})
		};
	}

	submission(): EmployeeLeaveSubmission | null {
		const previewRequest = this.previewRequest();
		if (!previewRequest) return null;
		return { ...previewRequest, reason: this.reason.trim() };
	}
}
