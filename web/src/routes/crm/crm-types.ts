export const crmOrganizationTypes = ['customer', 'partner', 'sponsor', 'vendor', 'investor', 'portfolio', 'other'] as const;
export type CRMOrganizationType = string;
export type CRMOrganizationStatus = 'prospect' | 'active' | 'paused';
export type CRMOpportunityStage = string;
export type CRMActivityKind = string;

export const deviceCRMActivityKinds: CRMActivityKind[] = ['note', 'email', 'meeting', 'call', 'task', 'file', 'event'];
export type CRMNextActionStatus = 'todo' | 'in_progress' | 'waiting' | 'done';
export type CRMActionUrgency = 'overdue' | 'today' | 'due_soon' | 'scheduled' | 'done';
export type CRMIntakeDraftSource = 'file' | 'mail' | 'calendar';
export type CRMProgressKind = string;
export type CRMRecordKind = 'relationship' | 'contact' | 'progress' | 'activity';
export type CRMImportance = 'high' | 'medium' | 'low';
export type CRMCalendarRegistrationState = 'registered' | 'failed';
export type CRMCurrency = string;
export type CRMMoneyTotals = Partial<Record<CRMCurrency, number>>;

export type CRMCalendarRegistrationDraft = {
	isRequested: boolean;
	isAllDay: boolean;
	startTime: string;
	endTime: string;
	location: string;
};

export type CRMContact = {
	id: string;
	organizationID: string;
	name: string;
	title: string;
	department?: string;
	email: string;
	phone?: string;
	note?: string;
	ownerPersonID?: string;
	ownerCircleID?: string;
};

export type CRMOpportunityContact = {
	contactID: string;
};

export type CRMOrganization = {
	id: string;
	name: string;
	types: CRMOrganizationType[];
	status: CRMOrganizationStatus;
	importance: CRMImportance;
	ownerPersonID?: string;
	ownerCircleID?: string;
	ownerName: string;
	ownerEmail: string;
	team: string;
	address?: string;
	tags: string[];
	description: string;
	lastContactDate: string;
	nextActionDate: string;
	openOpportunityCount: number;
	expectedValues: CRMMoneyTotals;
};

export type CRMIntakeDraft = {
	id: string;
	source: CRMIntakeDraftSource;
	title: string;
	organizationName: string;
	summary: string;
	confidence: number;
	suggestedAction: string;
};

export type CRMOpportunity = {
	id: string;
	organizationID: string;
	business: string;
	name: string;
	pipeline?: CRMProgressKind;
	stage: CRMOpportunityStage;
	stagePosition?: number;
	stageChangedAt?: string;
	ownerPersonID?: string;
	ownerCircleID?: string;
	ownerName: string;
	expectedValue?: number;
	currency: CRMCurrency;
	baseAmountMinor?: number;
	baseCurrencyCode?: CRMCurrency;
	importance: CRMImportance;
	nextActionID?: string;
	targetDate: string;
	dueTimeZone?: string;
	staleDays: number;
	kind?: CRMProgressKind;
	description?: string;
	lostReason?: string;
	contacts?: CRMOpportunityContact[];
	calendarEventID?: string;
	calendarRegistrationState?: CRMCalendarRegistrationState;
};

export type CRMRelationshipCreateDraft = {
	kind: 'relationship';
	name: string;
	types: CRMOrganizationType[];
	status: CRMOrganizationStatus;
	importance: CRMImportance;
	ownerPersonID: string;
	address: string;
	tags: string[];
	description: string;
	createdOrganizationID?: string;
	contact?: Omit<CRMContactCreateDraft, 'kind' | 'organizationID'>;
};

export type CRMContactCreateDraft = {
	kind: 'contact';
	organizationID: string;
	name: string;
	title: string;
	email: string;
	phone: string;
	note: string;
};

export type CRMOpportunityCreateDraft = {
	kind: 'progress';
	organizationID: string;
	contacts: CRMOpportunityContact[];
	business: string;
	name: string;
	progressKind: CRMProgressKind;
	stage: CRMOpportunityStage;
	lostReason: string;
	ownerPersonID: string;
	amount?: number;
	currency: CRMCurrency;
	importance: CRMImportance;
	targetDate: string;
	description: string;
	calendar: CRMCalendarRegistrationDraft;
};

export type CRMActivityCreateDraft = {
	kind: 'activity';
	organizationID: string;
	contactID?: string;
	opportunityID?: string;
	business: string;
	activityKind: CRMActivityKind;
	title: string;
	occurredAt: string;
	summary: string;
	taskOwnerID: string;
	taskStatus: string;
	calendar: CRMCalendarRegistrationDraft;
};

export type CRMActivityEditDraft = {
	organizationID: string;
	contactID?: string;
	opportunityID?: string;
	business: string;
	kind: CRMActivityKind;
	title: string;
	occurredAt: string;
	summary: string;
	taskOwnerID: string;
	taskStatus: string;
	isEvent: boolean;
	isWholeDay: boolean;
	startsAt: string;
	endsAt: string;
	location: string;
};

export type CRMCreateDraft = CRMRelationshipCreateDraft | CRMContactCreateDraft | CRMOpportunityCreateDraft | CRMActivityCreateDraft;

export type CRMNextAction = {
	id: string;
	organizationID: string;
	opportunityID?: string;
	title: string;
	ownerName: string;
	dueDate: string;
	status: CRMNextActionStatus;
};

export type CRMActivity = {
	id: string;
	organizationID: string;
	contactID?: string;
	opportunityID?: string;
	business: string;
	kind: CRMActivityKind;
	title: string;
	occurredAt: string;
	summary: string;
	taskID: string;
	taskStatus?: string;
	taskOwnerID?: string;
	taskOwnerName?: string;
	calendarEventID?: string;
	calendarEventDate?: string;
	isWholeDay?: boolean;
	calendarEndsAt?: string;
	calendarLocation?: string;
	calendarRegistrationState?: CRMCalendarRegistrationState;
};

export type CRMPipeline = {
	pipeline: CRMProgressKind;
	label: string;
	direction: string;
	isActive: boolean;
};

export type CRMPipelineStage = {
	pipeline: CRMProgressKind;
	stage: CRMOpportunityStage;
	label: string;
	position: number;
	outcome: 'open' | 'won' | 'lost' | 'on_hold';
};

export type CRMLostReason = {
	reason: string;
	label: string;
	isActive: boolean;
};

export type CRMReportSummary = {
	stageCounts: Record<string, number>;
	ownerSummaries: Array<{
		ownerName: string;
		organizationCount: number;
		openOpportunityCount: number;
		missingActionCount: number;
	}>;
};
