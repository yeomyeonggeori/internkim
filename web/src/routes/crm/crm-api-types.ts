import type {
	CRMOrganizationStatus,
	CRMOrganizationType,
	CRMActivityKind,
	CRMCurrency,
	CRMImportance,
	CRMOpportunityContact,
	CRMProgressKind
} from './crm-types';
import type { TaskVocabulary } from '$lib/flow/task-vocabulary';

export type CRMAuditResponse = {
	createdAt: string;
	createdByPersonID: string;
	updatedAt: string;
	updatedByPersonID: string;
	archivedAt?: string;
	archivedByPersonID?: string;
};

export type CRMOrganizationResponse = {
	id: string;
	name: string;
	status: CRMOrganizationStatus;
	types: CRMOrganizationType[];
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
	organizationID: string;
	name: string;
	email?: string;
	phone?: string;
	title?: string;
	department?: string;
	ownerPersonID: string;
	ownerCircleID?: string;
	description?: string;
	audit: CRMAuditResponse;
};

export type CRMOpportunityContactResponse = CRMOpportunityContact;

export type CRMOpportunityResponse = {
	id: string;
	organizationID?: string;
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
	organizationID?: string;
	contactID?: string;
	opportunityID?: string;
	business?: string;
	kind: CRMActivityKind;
	title: string;
	occurredAt: string;
	content?: string;
	taskStatus?: string;
	taskOwnerID?: string;
	isEvent?: boolean;
	isWholeDay?: boolean;
	startsAt?: string;
	endsAt?: string;
	notifyMinutesBefore?: number;
	location?: string;
	audit: CRMAuditResponse;
};

export type CRMPipelineResponse = {
	pipeline: CRMProgressKind;
	label: string;
	direction: string;
	isActive: boolean;
};

export type CRMPipelineStageResponse = {
	stage: string;
	label: string;
	position: number;
	outcome: 'open' | 'won' | 'lost' | 'on_hold';
};

export type CRMLostReasonResponse = {
	reason: string;
	label: string;
	isActive: boolean;
};

export type CRMDefinition = {
	id: string;
	name: string;
	color?: string;
};

export type CRMStageDefinition = CRMDefinition & {
	outcome: 'open' | 'won' | 'lost' | 'on_hold';
};

export type CRMPipelineDefinition = CRMDefinition & {
	direction?: string;
};

export type CRMVocabulary = {
	organization_types: CRMDefinition[];
	pipelines: CRMPipelineDefinition[];
	stages: CRMStageDefinition[];
	lost_reasons: CRMDefinition[];
};

export type CRMOrganizationPayload = Omit<CRMOrganizationResponse, 'id' | 'audit'>;
export type CRMContactPayload = Omit<CRMContactResponse, 'id' | 'audit'>;
export type CRMOpportunityPayload = {
	organizationID: string;
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
	organizationID: string;
	contactID: string;
	opportunityID: string;
	business: string;
	kind: CRMActivityKind;
	title: string;
	occurredAt: string;
	content: string;
	taskStatus: string;
	taskOwnerID: string;
	isEvent: boolean;
	isWholeDay: boolean;
	startsAt: string;
	endsAt: string;
	notifyMinutesBefore: number | null;
	location: string;
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
	organizations: CRMOrganizationResponse[];
	contacts: CRMContactResponse[];
	opportunities: CRMOpportunityResponse[];
	activities: CRMActivityResponse[];
	pipelines: CRMPipelineResponse[];
	stages: CRMPipelineStageResponse[];
	lostReasons: CRMLostReasonResponse[];
	vocabulary: CRMVocabulary;
	taskVocabulary: TaskVocabulary;
};
