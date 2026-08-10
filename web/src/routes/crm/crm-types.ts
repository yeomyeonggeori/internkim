export type CRMAccountType = 'customer' | 'partner' | 'sponsor' | 'vendor' | 'investor' | 'other';
export type CRMAccountStatus = 'prospect' | 'active' | 'paused';
export type CRMOpportunityStage = string;
export type CRMActivityKind = 'note' | 'email' | 'meeting' | 'call' | 'task' | 'file' | 'event' | 'stage_change';
export type CRMNextActionStatus = 'todo' | 'in_progress' | 'waiting' | 'done';
export type CRMActionUrgency = 'overdue' | 'today' | 'due_soon' | 'scheduled' | 'done';
export type CRMIntakeDraftSource = 'file' | 'mail' | 'calendar';
export type CRMProgressKind = 'sales' | 'fundraising' | 'investment' | 'sponsorship' | 'partnership' | 'procurement';
export type CRMRecordKind = 'relationship' | 'contact' | 'progress' | 'activity';
export type CRMImportance = 'high' | 'medium' | 'low';
export type CRMCalendarRegistrationState = 'registered' | 'failed';
export type CRMCurrency = 'KRW' | 'USD' | 'JPY' | 'EUR';
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
	accountID: string;
	name: string;
	title: string;
	department?: string;
	email: string;
	phone?: string;
	isPrimary: boolean;
	note?: string;
	ownerPersonID?: string;
	ownerCircleID?: string;
};

export type CRMOpportunityContact = {
	contactID: string;
	isPrimary: boolean;
};

export type CRMAccount = {
	id: string;
	name: string;
	types: CRMAccountType[];
	status: CRMAccountStatus;
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
	accountName: string;
	summary: string;
	confidence: number;
	suggestedAction: string;
};

export type CRMOpportunity = {
	id: string;
	accountID: string;
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
	accountType: CRMAccountType;
	status: CRMAccountStatus;
	importance: CRMImportance;
	ownerName: string;
	team: string;
	address: string;
	tags: string[];
	description: string;
	lastContactDate: string;
	nextActionDate: string;
};

export type CRMContactCreateDraft = {
	kind: 'contact';
	accountID: string;
	name: string;
	title: string;
	email: string;
	phone: string;
	isPrimary: boolean;
	note: string;
};

export type CRMOpportunityCreateDraft = {
	kind: 'progress';
	accountID: string;
	business: string;
	name: string;
	progressKind: CRMProgressKind;
	stage: CRMOpportunityStage;
	lostReason: string;
	ownerName: string;
	amount?: number;
	currency: CRMCurrency;
	importance: CRMImportance;
	targetDate: string;
	description: string;
	calendar: CRMCalendarRegistrationDraft;
};

export type CRMActivityCreateDraft = {
	kind: 'activity';
	accountID: string;
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
	accountID: string;
	opportunityID?: string;
	business: string;
	kind: CRMActivityKind;
	title: string;
	occurredAt: string;
	summary: string;
	taskOwnerID: string;
	taskStatus: string;
	shouldUpdateLinkedCalendar: boolean;
};

export type CRMCreateDraft = CRMRelationshipCreateDraft | CRMContactCreateDraft | CRMOpportunityCreateDraft | CRMActivityCreateDraft;

export type CRMNextAction = {
	id: string;
	accountID: string;
	opportunityID?: string;
	title: string;
	ownerName: string;
	dueDate: string;
	status: CRMNextActionStatus;
};

export type CRMActivity = {
	id: string;
	accountID: string;
	contactID?: string;
	opportunityID?: string;
	business: string;
	kind: CRMActivityKind;
	title: string;
	occurredAt: string;
	summary: string;
	taskID: string;
	taskStatus?: string;
	taskOwnerName?: string;
	calendarEventID?: string;
	calendarEventDate?: string;
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
		accountCount: number;
		openOpportunityCount: number;
		missingActionCount: number;
	}>;
};
