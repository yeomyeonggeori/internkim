import type { OrgGroup, UserRecord } from '$lib/organization/types';
import {
	interimCurrencyCatalogue,
	loadCurrencyCatalogue,
	type CurrencyCatalogue
} from '$lib/currency/currency-catalogue';
import { interimCompanyBaseCurrency, loadCompanyBaseCurrency } from '$lib/company/base-currency';
import { fetchOrganizationDirectory } from '../organization/organization-api';
import {
	CRMApiError,
	archiveCRMOrganization,
	archiveCRMOpportunity,
	createCRMActivity,
	createCRMContact,
	createCRMOpportunity,
	loadCRMData,
	saveCRMVocabulary,
	positionCRMOpportunity,
	settlementIsConvertedByServer,
	transitionCRMOpportunity,
	updateCRMOrganization,
	updateCRMActivity,
	updateCRMContact,
	updateCRMOpportunity
} from './crm-data-source';
import {
	organizationPayload,
	activityPayload,
	activityPayloadFromDraft,
	browserTimeZone,
	contactPayload,
	contactPayloadFromDraft,
	mapCRMViewData,
	opportunityPayload,
	opportunityPayloadFromDraft,
	resolveOwner,
	type CRMViewData
} from './crm-mappers';
import { crmOrganizations, crmActivities, crmContacts, crmNextActions, crmOpportunities } from './dev-crm-fixture';
import type {
	CRMOrganization,
	CRMActivity,
	CRMActivityEditDraft,
	CRMContact,
	CRMCreateDraft,
	CRMLostReason,
	CRMNextAction,
	CRMOpportunity,
	CRMPipeline,
	CRMPipelineStage,
	CRMProgressKind
} from './crm-types';
import type { CRMTransitionPayload, CRMVocabulary } from './crm-api-types';
import type { TaskVocabulary } from '$lib/flow/task-vocabulary';
import type { CRMPipelineBoardMoveRequest } from './crm-pipeline-board-drag';
import { crmErrorMessage, CRMPageError } from './crm-error-text';
import { crmLabel } from './crm-labels';
import {
	opportunityTransitionOutcome,
	opportunitySettlesOnTransition,
	type CRMOpportunityTransitionValues
} from './crm-opportunity-transition';
import type { CRMText } from './text';
import {
	createCRMRelationshipRecords,
	CRMRelationshipContactCreateError
} from './crm-relationship-create';

const fixtureMode = import.meta.env.VITE_MOCK_CRM === '1';

export class CRMPageController {
	organizations = $state<CRMOrganization[]>([]);
	contacts = $state<CRMContact[]>([]);
	opportunities = $state<CRMOpportunity[]>([]);
	activities = $state<CRMActivity[]>([]);
	nextActions = $state<CRMNextAction[]>([]);
	pipelines = $state<CRMPipeline[]>([]);
	stages = $state<CRMPipelineStage[]>([]);
	lostReasons = $state<CRMLostReason[]>([]);
	vocabulary = $state<CRMVocabulary>({ organization_types: [], pipelines: [], lost_reasons: [] });
	currencyCatalogue = $state<CurrencyCatalogue>(interimCurrencyCatalogue);
	companyBaseCurrency = $state<string>(interimCompanyBaseCurrency);
	taskVocabulary = $state<TaskVocabulary>({});
	people = $state<UserRecord[]>([]);
	groups = $state<OrgGroup[]>([]);
	isLoading = $state(true);
	isSaving = $state(false);
	permissionDenied = $state(false);
	currentEmail = $state('');
	private error = $state<unknown>(null);

	constructor(private readonly text: CRMText) {}

	get errorMessage(): string {
		return this.error === null ? '' : crmErrorMessage(this.error, this.text);
	}

	get currentOwnerName(): string {
		return this.people.find((person) => person.email.toLowerCase() === this.currentEmail.toLowerCase())?.name
			?? this.currentEmail
			?? '';
	}

	get currentOwnerPersonID(): string {
		return this.people.find((person) => person.email.toLowerCase() === this.currentEmail.toLowerCase())?.userID ?? '';
	}

	get businessOptions(): string[] {
		const values = this.taskVocabulary.businesses?.map((entry) => entry.name) ?? [];
		return values.length > 0 ? values : ['general'];
	}

	get activityKindOptions(): string[] {
		const values = this.taskVocabulary.types?.map((entry) => entry.name) ?? [];
		return values.length > 0 ? values : ['note', 'email', 'meeting', 'call', 'task', 'file', 'event'];
	}

	get organizationTypeOptions(): string[] {
		return this.vocabulary.organization_types.map((entry) => entry.id);
	}

	async load(currentEmail: string): Promise<void> {
		this.currentEmail = currentEmail;
		this.isLoading = true;
		this.error = null;
		this.permissionDenied = false;
		try {
			if (fixtureMode) {
				this.loadFixture(currentEmail);
				return;
			}
			const [data, directory] = await Promise.all([
				loadCRMData(),
				this.loadOrganizationDirectory()
			]);
			this.people = directory.records ?? [];
			this.groups = directory.availableGroups ?? [];
			this.applyViewData(mapCRMViewData(data, this.people, browserTimeZone(), this.groups));
		} catch (error) {
			this.applyError(error);
		} finally {
			this.isLoading = false;
		}
	}

	async create(draft: CRMCreateDraft): Promise<void> {
		if (draft.kind === 'relationship') {
			if (fixtureMode) throw new Error(this.text.fixtureModeReadOnly);
			await this.createRelationship(draft);
			return;
		}
		await this.mutate(async () => {
			const owner = resolveOwner(this.people, ownerHint(draft), this.currentEmail);
			if (draft.kind === 'contact') {
				await createCRMContact(contactPayloadFromDraft(draft, owner));
				return;
			}
			if (draft.kind === 'progress') {
				const payload = opportunityPayloadFromDraft(draft, owner, browserTimeZone());
				const transition = {
					...this.transitionPayload('', draft.stage, null, draft.progressKind, {
						amountMinor: payload.amountMinor,
						currencyCode: payload.currencyCode,
						baseAmountMinor: null,
						baseCurrencyCode: '',
						lostReason: draft.lostReason
					}),
					stagePosition: 0
				};
				const created = await createCRMOpportunity({ ...payload, transition });
				if (this.settlesThroughCloseEndpoint('', draft.progressKind, draft.stage)) {
					await transitionCRMOpportunity(created.id, transition);
				}
				return;
			}
			await createCRMActivity(activityPayloadFromDraft(draft));
		});
	}

	async saveOrganization(organization: CRMOrganization): Promise<void> {
		await this.mutate(() => {
			const owner = resolveOwner(this.people, organization.ownerPersonID ?? organization.ownerName, this.currentEmail);
			const nextOrganization = {
				...organization,
				ownerPersonID: owner.userID,
				ownerCircleID: owner.groupID
			};
			return updateCRMOrganization(organization.id, organizationPayload(nextOrganization));
		});
	}

	async archiveOrganization(organizationID: string): Promise<void> {
		await this.mutate(() => archiveCRMOrganization(organizationID));
	}

	async saveContact(contact: CRMContact): Promise<void> {
		await this.mutate(() => updateCRMContact(contact.id, contactPayload(contact)));
	}

	async saveOpportunity(opportunity: CRMOpportunity): Promise<void> {
		await this.mutate(async () => {
			const existing = this.opportunities.find((candidate) => candidate.id === opportunity.id);
			const payload = opportunityPayload(opportunity);
			if (!existing || opportunity.stage === existing.stage) {
				await updateCRMOpportunity(opportunity.id, payload);
				return;
			}
			const transition = this.transitionPayload(opportunity.id, opportunity.stage, null, opportunity.pipeline, {
				amountMinor: payload.amountMinor,
				currencyCode: payload.currencyCode,
				baseAmountMinor: opportunity.baseAmountMinor ?? null,
				baseCurrencyCode: opportunity.baseCurrencyCode ?? '',
				lostReason: opportunity.lostReason ?? ''
			});
			if (!this.settlesThroughCloseEndpoint(opportunity.id, opportunity.pipeline, opportunity.stage)) {
				await updateCRMOpportunity(opportunity.id, { ...payload, transition });
				return;
			}
			await updateCRMOpportunity(opportunity.id, payload);
			await transitionCRMOpportunity(opportunity.id, transition);
		});
	}

	async archiveOpportunity(opportunityID: string): Promise<void> {
		await this.mutate(() => archiveCRMOpportunity(opportunityID));
	}

	async saveVocabulary(vocabulary: CRMVocabulary): Promise<void> {
		await this.mutate(() => saveCRMVocabulary(vocabulary));
	}

	async moveOpportunity(request: CRMPipelineBoardMoveRequest): Promise<void> {
		const opportunity = this.opportunities.find((candidate) => candidate.id === request.opportunityID);
		if (!opportunity) return;
		await this.mutate(async () => {
			if (opportunity.stage !== request.targetStage) {
				const payload = opportunityPayload(opportunity);
				await transitionCRMOpportunity(
					opportunity.id,
					this.transitionPayload(
						opportunity.id,
						request.targetStage,
						request.beforeOpportunityID,
						opportunity.pipeline ?? opportunity.kind,
						{
							amountMinor: payload.amountMinor,
							currencyCode: payload.currencyCode,
							baseAmountMinor: opportunity.baseAmountMinor ?? null,
							baseCurrencyCode: opportunity.baseCurrencyCode ?? '',
							lostReason: opportunity.lostReason ?? ''
						}
					)
				);
				return;
			}
			await positionCRMOpportunity(opportunity.id, {
				position: this.nextPosition(opportunity, request.beforeOpportunityID),
				beforeOpportunityID: request.beforeOpportunityID ?? '',
				updatedAt: new Date().toISOString()
			});
		});
	}

	async saveActivity(activity: CRMActivity, draft: CRMActivityEditDraft): Promise<void> {
		const updated: CRMActivity = {
			...activity,
			organizationID: draft.organizationID,
			contactID: draft.contactID,
			opportunityID: draft.opportunityID,
			business: draft.business,
			kind: draft.kind,
			title: draft.title,
			occurredAt: new Date(draft.occurredAt).toISOString(),
			summary: draft.summary,
			calendarEventID: draft.isEvent ? activity.id : undefined,
			calendarEventDate: draft.isEvent ? new Date(draft.startsAt).toISOString() : undefined,
			calendarEndsAt: draft.isEvent ? new Date(draft.endsAt || draft.startsAt).toISOString() : undefined,
			isWholeDay: draft.isWholeDay,
			calendarLocation: draft.location
		};
		await this.mutate(() => updateCRMActivity(activity.id, {
			...activityPayload(updated),
			taskOwnerID: draft.taskOwnerID || activity.taskOwnerID || '',
			taskStatus: draft.taskStatus || activity.taskStatus || 'todo'
		}));
	}

	stagesFor(pipeline: CRMProgressKind): CRMPipelineStage[] {
		return this.stages.filter((stage) => stage.pipeline === pipeline).sort((left, right) => left.position - right.position);
	}

	private async mutate(operation: () => Promise<unknown>): Promise<void> {
		if (fixtureMode) throw new Error(this.text.fixtureModeReadOnly);
		this.isSaving = true;
		this.error = null;
		this.permissionDenied = false;
		try {
			try {
				await operation();
			} catch (error) {
				this.applyError(error);
				throw new Error(this.errorMessage);
			}
			try {
				await this.reloadRemote();
			} catch {
				this.error = new CRMPageError('refresh_after_save_failed');
			}
		} finally {
			this.isSaving = false;
		}
	}

	private async reloadRemote(): Promise<void> {
		const data = await loadCRMData();
		this.applyViewData(mapCRMViewData(data, this.people, browserTimeZone(), this.groups));
	}

	private async createRelationship(draft: Extract<CRMCreateDraft, { kind: 'relationship' }>): Promise<void> {
		this.isSaving = true;
		this.error = null;
		this.permissionDenied = false;
		try {
			const owner = resolveOwner(this.people, draft.ownerPersonID, this.currentEmail);
			try {
				await createCRMRelationshipRecords(draft, owner, this.text.relationshipContactCreateFailed);
			} catch (error) {
				if (error instanceof CRMRelationshipContactCreateError) {
					try {
						await this.reloadRemote();
					} catch {
						this.error = new CRMPageError('refresh_after_save_failed');
					}
					throw error;
				}
				this.applyError(error);
				throw new Error(this.errorMessage);
			}
			try {
				await this.reloadRemote();
			} catch {
				this.error = new CRMPageError('refresh_after_save_failed');
			}
		} finally {
			this.isSaving = false;
		}
	}

	private applyViewData(data: CRMViewData): void {
		this.organizations = data.organizations;
		this.contacts = data.contacts;
		this.opportunities = data.opportunities;
		this.activities = data.activities;
		this.nextActions = data.nextActions;
		this.pipelines = data.pipelines;
		this.stages = data.stages;
		this.lostReasons = data.lostReasons;
		this.vocabulary = data.vocabulary;
		this.taskVocabulary = data.taskVocabulary;
		void this.refreshCurrencyCatalogue();
		void this.refreshCompanyBaseCurrency();
	}

	private async refreshCurrencyCatalogue(): Promise<void> {
		this.currencyCatalogue = await loadCurrencyCatalogue();
	}

	private async refreshCompanyBaseCurrency(): Promise<void> {
		this.companyBaseCurrency = await loadCompanyBaseCurrency();
	}

	private applyError(error: unknown): void {
		this.permissionDenied = error instanceof CRMApiError && (error.status === 401 || error.status === 403);
		this.error = error;
	}

	private async loadOrganizationDirectory(): Promise<Awaited<ReturnType<typeof fetchOrganizationDirectory>>> {
		try {
			return await fetchOrganizationDirectory('organization_load_failed');
		} catch {
			throw new CRMPageError('organization_load_failed');
		}
	}

	private transitionPayload(
		opportunityID: string,
		stage: string,
		beforeOpportunityID: string | null,
		pipelineHint: CRMProgressKind | undefined,
		values: CRMOpportunityTransitionValues
	): CRMTransitionPayload {
		const opportunity = this.opportunities.find((candidate) => candidate.id === opportunityID);
		const pipeline = this.pipelineOf(opportunityID, pipelineHint);
		return {
			stage,
			stagePosition: this.nextPosition(opportunity, beforeOpportunityID, stage, pipelineHint),
			beforeOpportunityID: beforeOpportunityID ?? '',
			occurredAt: new Date().toISOString(),
			...opportunityTransitionOutcome(this.stages, pipeline, stage, values, {
				baseCurrency: this.companyBaseCurrency,
				isConvertedByServer: settlementIsConvertedByServer()
			})
		};
	}

	private pipelineOf(opportunityID: string, pipelineHint: CRMProgressKind | undefined): CRMProgressKind {
		const opportunity = this.opportunities.find((candidate) => candidate.id === opportunityID);
		return opportunity?.pipeline ?? opportunity?.kind ?? pipelineHint ?? 'sales';
	}

	private settlesThroughCloseEndpoint(opportunityID: string, pipelineHint: CRMProgressKind | undefined, stage: string): boolean {
		if (!settlementIsConvertedByServer()) return false;
		return opportunitySettlesOnTransition(this.stages, this.pipelineOf(opportunityID, pipelineHint), stage);
	}

	private nextPosition(
		opportunity: CRMOpportunity | undefined,
		beforeOpportunityID: string | null,
		targetStage = opportunity?.stage ?? '',
		pipelineHint?: CRMProgressKind
	): number {
		const pipeline = opportunity?.pipeline ?? opportunity?.kind ?? pipelineHint ?? 'sales';
		const sameStage = this.opportunities
			.filter((candidate) => (candidate.pipeline ?? candidate.kind) === pipeline && candidate.stage === targetStage && candidate.id !== opportunity?.id)
			.sort((left, right) => (left.stagePosition ?? 0) - (right.stagePosition ?? 0));
		if (!beforeOpportunityID) return (sameStage.at(-1)?.stagePosition ?? 0) + 1024;
		const beforeIndex = sameStage.findIndex((candidate) => candidate.id === beforeOpportunityID);
		if (beforeIndex < 0) return (sameStage.at(-1)?.stagePosition ?? 0) + 1024;
		const before = sameStage[beforeIndex]?.stagePosition ?? 1024;
		const previous = sameStage[beforeIndex - 1]?.stagePosition ?? 0;
		return (before + previous) / 2;
	}

	private loadFixture(currentEmail: string): void {
		this.people = [{ userID: 'fixture-user', handle: 'fixture', name: crmOrganizations[0]?.ownerName ?? 'Fixture User', email: currentEmail || 'fixture@example.com' }];
		this.groups = [];
		this.organizations = structuredClone(crmOrganizations);
		this.contacts = structuredClone(crmContacts);
		this.opportunities = structuredClone(crmOpportunities);
		this.activities = structuredClone(crmActivities);
		this.nextActions = structuredClone(crmNextActions);
		const pipelineNames = [...new Set(this.opportunities.map((opportunity) => opportunity.kind ?? 'sales'))];
		this.pipelines = pipelineNames.map((pipeline) => ({ pipeline, label: crmLabel(this.text.progressKinds, pipeline), direction: 'outbound', isActive: true }));
		this.stages = pipelineNames.flatMap((pipeline) => [...new Set(this.opportunities
			.filter((opportunity) => (opportunity.kind ?? 'sales') === pipeline)
			.map((opportunity) => opportunity.stage))]
			.map((stage, index) => ({ pipeline, stage, label: stage, position: index + 1, outcome: fixtureOutcome(stage) })));
		this.lostReasons = [];
		this.vocabulary = {
			organization_types: [],
			pipelines: this.pipelines.map((pipeline) => ({
				id: pipeline.pipeline,
				name: pipeline.label,
				direction: pipeline.direction,
				stages: this.stages.filter((stage) => stage.pipeline === pipeline.pipeline).map((stage) => ({
					id: stage.stage,
					name: stage.stage,
					outcome: stage.outcome
				}))
			})),
			lost_reasons: []
		};
		this.taskVocabulary = {
			businesses: [...new Set(this.opportunities.map((opportunity) => opportunity.business))].map((name) => ({ name })),
			types: [...new Set(this.activities.map((activity) => activity.kind))].map((name) => ({ name }))
		};
	}
}

function ownerHint(draft: CRMCreateDraft): string {
	return draft.kind === 'progress' ? draft.ownerPersonID : '';
}

function fixtureOutcome(stage: string): CRMPipelineStage['outcome'] {
	if (stage === 'won' || stage === 'closed' || stage === 'contract_award') return 'won';
	if (stage === 'lost') return 'lost';
	if (stage === 'on_hold') return 'on_hold';
	return 'open';
}
