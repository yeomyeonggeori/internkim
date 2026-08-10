import type {
	CRMAccountStatus,
	CRMAccountType,
	CRMActivityKind,
	CRMCurrency,
	CRMImportance,
	CRMOpportunityContact,
	CRMProgressKind
} from './crm-types';

export type CRMAuditResponse = {
	createdAt: string;
	createdByPersonID: string;
	updatedAt: string;
	updatedByPersonID: string;
	archivedAt?: string;
	archivedByPersonID?: string;
};

export type CRMAccountResponse = {
	id: string;
	name: string;
	status: CRMAccountStatus;
	types: CRMAccountType[];
	tags: string[];
	importance: CRMImportance;
	ownerPersonID: string;
	ownerCircleID?: string;
	address?: string;
	description?: string;
	audit: CRMAuditResponse;
};

export type CRMContactResponse = {
	id: string;
	accountID: string;
	name: string;
	email?: string;
	phone?: string;
	title?: string;
	department?: string;
	isPrimary: boolean;
	ownerPersonID: string;
	ownerCircleID?: string;
	description?: string;
	audit: CRMAuditResponse;
};

export type CRMOpportunityContactResponse = CRMOpportunityContact;

export type CRMOpportunityResponse = {
	id: string;
	accountID?: string;
	business?: string;
	name: string;
	pipeline: CRMProgressKind;
	stage: string;
	stagePosition: number;
	stageChangedAt: string;
	ownerPersonID: string;
	ownerCircleID?: string;
	amountMinor?: number;
	currencyCode: CRMCurrency | '';
	baseAmountMinor?: number;
	baseCurrencyCode?: CRMCurrency;
	importance: CRMImportance;
	dueAt?: string;
	dueTimeZone?: string;
	lostReason?: string;
	description?: string;
	contacts?: CRMOpportunityContactResponse[];
	audit: CRMAuditResponse;
};

export type CRMActivityResponse = {
	id: string;
	accountID?: string;
	contactID?: string;
	opportunityID?: string;
	business?: string;
	kind: CRMActivityKind;
	title: string;
	occurredAt: string;
	content?: string;
	audit: CRMAuditResponse;
};

export type CRMPipelineResponse = {
	pipeline: CRMProgressKind;
	label: string;
	direction: string;
	isActive: boolean;
};

export type CRMPipelineStageResponse = {
	pipeline: CRMProgressKind;
	stage: string;
	position: number;
	outcome: 'open' | 'won' | 'lost' | 'on_hold';
};

export type CRMLostReasonResponse = {
	reason: string;
	label: string;
	isActive: boolean;
};

export type CRMAccountPayload = Omit<CRMAccountResponse, 'id' | 'audit'>;
export type CRMContactPayload = Omit<CRMContactResponse, 'id' | 'audit'>;
export type CRMOpportunityPayload = {
	accountID: string;
	business: string;
	name: string;
	pipeline: CRMProgressKind;
	ownerPersonID: string;
	ownerCircleID: string;
	amountMinor: number | null;
	currencyCode: CRMCurrency | '';
	importance: CRMImportance;
	dueAt: string;
	dueTimeZone: string;
	description: string;
	contacts: CRMOpportunityContactResponse[];
	transition?: CRMTransitionPayload;
};
export type CRMActivityPayload = {
	accountID: string;
	contactID: string;
	opportunityID: string;
	business: string;
	kind: Exclude<CRMActivityKind, 'stage_change'>;
	title: string;
	occurredAt: string;
	content: string;
};

export type CRMTransitionPayload = {
	stage: string;
	stagePosition: number;
	beforeOpportunityID: string;
	occurredAt: string;
	lostReason: string;
	baseAmountMinor: number | null;
	baseCurrencyCode: CRMCurrency | '';
};

export type CRMPositionPayload = {
	position: number;
	beforeOpportunityID: string;
	updatedAt: string;
};

export type CRMDataResponse = {
	accounts: CRMAccountResponse[];
	contacts: CRMContactResponse[];
	opportunities: CRMOpportunityResponse[];
	activities: CRMActivityResponse[];
	pipelines: CRMPipelineResponse[];
	stages: CRMPipelineStageResponse[];
	lostReasons: CRMLostReasonResponse[];
};
