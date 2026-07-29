import type {
	EmployeeLeavePartialPeriod,
	EmployeeLeavePreviewRequest,
	EmployeeLeaveRequest,
	EmployeeLeaveResubmission,
	EmployeeLeaveSubmission,
	EmployeeLeaveType,
	EmployeeLeaveUpdate,
	EmployeeLeaveUnit
} from './employee-leave-types';

export type LeaveRequestDraftMode = 'create' | 'edit' | 'resubmit';
export type LeaveRequestDraftSubmission =
	| { mode: 'create'; request: EmployeeLeaveSubmission }
	| { mode: 'edit'; request: EmployeeLeaveUpdate }
	| { mode: 'resubmit'; request: EmployeeLeaveResubmission };

export class LeaveRequestDraft {
	mode = $state<LeaveRequestDraftMode>('create');
	requestID = $state('');
	revision = $state(0);
	leaveTypeID = $state('');
	unit = $state<EmployeeLeaveUnit>('fullDay');
	startDate = $state('');
	endDate = $state('');
	partialPeriod = $state<EmployeeLeavePartialPeriod>('morning');
	startTime = $state('');
	reason = $state('');
	response = $state('');
	attachments = $state<File[]>([]);
	existingAttachments = $state<EmployeeLeaveRequest['attachments']>([]);
	removedAttachmentIDs = $state<string[]>([]);

	synchronize(leaveTypes: EmployeeLeaveType[], defaultDate: string): void {
		const activeTypes = leaveTypes.filter((leaveType) => leaveType.isActive);
		const selectedType = leaveTypes.find((leaveType) => leaveType.id === this.leaveTypeID);
		const canKeepSelectedType =
			selectedType?.isActive === true || (this.requestID !== '' && selectedType !== undefined);
		if (!canKeepSelectedType) {
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

	setAttachments(attachments: File[]): void {
		this.attachments = attachments;
	}

	removeExistingAttachment(attachmentID: string): void {
		this.existingAttachments = this.existingAttachments.filter(
			(attachment) => attachment.id !== attachmentID
		);
		if (!this.removedAttachmentIDs.includes(attachmentID)) {
			this.removedAttachmentIDs = [...this.removedAttachmentIDs, attachmentID];
		}
	}

	loadRequest(request: EmployeeLeaveRequest): void {
		this.mode = request.canEdit ? 'edit' : 'resubmit';
		this.requestID = request.id;
		this.revision = request.revision;
		this.leaveTypeID = request.leaveTypeID;
		this.unit = request.unit;
		this.startDate = request.startDate;
		this.endDate = request.endDate ?? '';
		this.partialPeriod =
			request.unit === 'quarterDay' ? 'custom' : (request.partialPeriod ?? 'morning');
		this.startTime = request.startTime ?? '';
		this.reason = request.reason;
		this.response = '';
		this.attachments = [];
		this.existingAttachments = request.attachments;
		this.removedAttachmentIDs = [];
	}

	reset(leaveTypes: EmployeeLeaveType[], defaultDate: string): void {
		this.mode = 'create';
		this.requestID = '';
		this.revision = 0;
		this.leaveTypeID = '';
		this.unit = 'fullDay';
		this.startDate = defaultDate;
		this.endDate = defaultDate;
		this.partialPeriod = 'morning';
		this.startTime = '';
		this.reason = '';
		this.response = '';
		this.attachments = [];
		this.existingAttachments = [];
		this.removedAttachmentIDs = [];
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

	submission(): LeaveRequestDraftSubmission | null {
		const previewRequest = this.previewRequest();
		const reason = this.reason.trim();
		if (!previewRequest) return null;
		if (this.mode === 'create') {
			return { mode: 'create', request: { ...previewRequest, reason } };
		}
		if (this.mode === 'edit') {
			return {
				mode: 'edit',
				request: {
					...previewRequest,
					reason,
					revision: this.revision,
					removedAttachmentIDs: this.removedAttachmentIDs
				}
			};
		}
		const response = this.response.trim();
		return {
			mode: 'resubmit',
			request: {
				...previewRequest,
				reason,
				...(response ? { response } : {})
			}
		};
	}
}
