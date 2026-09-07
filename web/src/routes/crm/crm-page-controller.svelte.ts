import type { OrgGroup, UserRecord } from '$lib/organization/types';
import {
	interimCurrencyCatalogue,
	loadCurrencyCatalogue,
	type CurrencyCatalogue
} from '$lib/currency/currency-catalogue';
import { interimCompanyBaseCurrency, loadCompanyBaseCurrency } from '$lib/company/base-currency';
import { loadCRMOrganizationDirectory } from './crm-data-source';
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
	crmPipelinesOf,
	mapCRMViewData,
	nextActionsOf,
	opportunityPayload,
	opportunityPayloadFromDraft,
	resolveOwner,
	withNextAction,
	type CRMViewData
} from './crm-mappers';
import { crmFixtureActivityKindColors, crmFixtureMode as fixtureMode, crmFixtureOrganizationTypeColors, crmFixturePeople, crmFixturePipelineColors, crmOrganizations, crmActivities, crmContacts, crmOpportunities } from './dev-crm-fixture';
import { crmStages } from './crm-stages';
import type {
	CRMOrganization,
	CRMActivity,
	CRMActivityEditDraft,
	CRMContact,
	CRMCreateDraft,
	CRMNextAction,
	CRMOpportunity,
	CRMOpportunityStage,
	CRMPipeline,
	CRMPipelineStage,
} from './crm-types';
import type { CRMTransitionPayload, CRMVocabulary } from './crm-api-types';
import type { TaskVocabulary } from '$lib/task/task-vocabulary';
import { taskStatus } from '$lib/task/central-task';
import type { CRMPipelineBoardMoveRequest } from './crm-pipeline-board-drag';
import { cloneCRMVocabulary } from './crm-definitions';
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


export class CRMPageController {
	organizations = $state<CRMOrganization[]>([]);
	contacts = $state<CRMContact[]>([]);
	opportunities = $state<CRMOpportunity[]>([]);
	activities = $state<CRMActivity[]>([]);
	nextActions = $state<CRMNextAction[]>([]);
	pipelines = $state<CRMPipeline[]>([]);
	vocabulary = $state<CRMVocabulary>({ organization_types: [], pipelines: [] });
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
		return this.people.find((person) => person.email.toLowerCase() === this.currentEmail.toLowerCase())?.memberID ?? '';
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

	get stages(): CRMPipelineStage[] {
		return crmStages;
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
			const pendingData = loadCRMData();
			const pendingCatalogue = loadCurrencyCatalogue();
			const pendingBaseCurrency = loadCompanyBaseCurrency();
			const directory = await this.loadOrganizationDirectory();
			this.people = directory.records ?? [];
			this.groups = directory.availableGroups ?? [];
			const [data, catalogue, baseCurrency] = await Promise.all([
				pendingData,
				pendingCatalogue,
				pendingBaseCurrency
			]);
			this.currencyCatalogue = catalogue;
			this.companyBaseCurrency = baseCurrency;
			this.applyViewData(mapCRMViewData(data, this.people, this.currencyCatalogue, browserTimeZone(), this.groups));
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
				const payload = opportunityPayloadFromDraft(draft, owner, browserTimeZone(), this.currencyCatalogue);
				const transition = {
					...this.transitionPayload('', draft.stage, null, {
						amountMinor: payload.amountMinor,
						currencyCode: payload.currencyCode,
						baseAmountMinor: null,
						baseCurrencyCode: '',
						lostReason: draft.lostReason
					}),
					stagePosition: 0
				};
				const created = await createCRMOpportunity({ ...payload, transition });
				if (this.settlesThroughCloseEndpoint(draft.stage)) {
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
				ownerPersonID: owner.memberID,
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
			const payload = opportunityPayload(opportunity, this.currencyCatalogue);
			if (!existing || opportunity.stage === existing.stage) {
				await updateCRMOpportunity(opportunity.id, payload);
				return;
			}
			const transition = this.transitionPayload(opportunity.id, opportunity.stage, null, {
				amountMinor: payload.amountMinor,
				currencyCode: payload.currencyCode,
				baseAmountMinor: opportunity.baseAmountMinor ?? null,
				baseCurrencyCode: opportunity.baseCurrencyCode ?? '',
				lostReason: opportunity.lostReason ?? ''
			});
			if (!this.settlesThroughCloseEndpoint(opportunity.stage)) {
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
		if (fixtureMode) {
			this.vocabulary = cloneCRMVocabulary(vocabulary);
			this.pipelines = crmPipelinesOf(vocabulary);
			return;
		}
		this.isSaving = true;
		this.error = null;
		this.permissionDenied = false;
		try {
			await saveCRMVocabulary(vocabulary);
			this.vocabulary = cloneCRMVocabulary(vocabulary);
			this.pipelines = crmPipelinesOf(vocabulary);
		} catch (error) {
			this.applyError(error);
			throw new Error(this.errorMessage);
		} finally {
			this.isSaving = false;
		}
	}

	async moveOpportunity(request: CRMPipelineBoardMoveRequest): Promise<void> {
		const opportunity = this.opportunities.find((candidate) => candidate.id === request.opportunityID);
		if (!opportunity) return;
		await this.mutate(async () => {
			if (opportunity.stage !== request.targetStage) {
				const payload = opportunityPayload(opportunity, this.currencyCatalogue);
				await transitionCRMOpportunity(
					opportunity.id,
					this.transitionPayload(
						opportunity.id,
						request.targetStage,
						request.beforeOpportunityID,
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
			participantIDs: draft.participantPersonIDs.length > 0 ? draft.participantPersonIDs : activity.participantIDs,
			taskStatus: draft.taskStatus || activity.taskStatus || taskStatus.planned
		}));
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
		this.applyViewData(mapCRMViewData(data, this.people, this.currencyCatalogue, browserTimeZone(), this.groups));
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
		this.vocabulary = data.vocabulary;
		this.taskVocabulary = data.taskVocabulary;
	}

	private applyError(error: unknown): void {
		this.permissionDenied = error instanceof CRMApiError && (error.status === 401 || error.status === 403);
		this.error = error;
	}

	private async loadOrganizationDirectory(): Promise<Awaited<ReturnType<typeof loadCRMOrganizationDirectory>>> {
		try {
			return await loadCRMOrganizationDirectory();
		} catch {
			throw new CRMPageError('organization_load_failed');
		}
	}

	private transitionPayload(
		opportunityID: string,
		stage: CRMOpportunityStage,
		beforeOpportunityID: string | null,
		values: CRMOpportunityTransitionValues
	): CRMTransitionPayload {
		const opportunity = this.opportunities.find((candidate) => candidate.id === opportunityID);
		return {
			stage,
			stagePosition: this.nextPosition(opportunity, beforeOpportunityID, stage),
			beforeOpportunityID: beforeOpportunityID ?? '',
			occurredAt: new Date().toISOString(),
			...opportunityTransitionOutcome(this.stages, stage, values, {
				baseCurrency: this.companyBaseCurrency
			})
		};
	}

	private settlesThroughCloseEndpoint(stage: string): boolean {
		return opportunitySettlesOnTransition(this.stages, stage);
	}

	private nextPosition(
		opportunity: CRMOpportunity | undefined,
		beforeOpportunityID: string | null,
		targetStage = opportunity?.stage ?? ''
	): number {
		const sameStage = this.opportunities
			.filter((candidate) => candidate.stage === targetStage && candidate.id !== opportunity?.id)
			.sort((left, right) => (left.stagePosition ?? 0) - (right.stagePosition ?? 0));
		if (!beforeOpportunityID) return (sameStage.at(-1)?.stagePosition ?? 0) + 1024;
		const beforeIndex = sameStage.findIndex((candidate) => candidate.id === beforeOpportunityID);
		if (beforeIndex < 0) return (sameStage.at(-1)?.stagePosition ?? 0) + 1024;
		const before = sameStage[beforeIndex]?.stagePosition ?? 1024;
		const previous = sameStage[beforeIndex - 1]?.stagePosition ?? 0;
		return (before + previous) / 2;
	}

	private loadFixture(currentEmail: string): void {
		this.people = [
			...crmFixturePeople,
			{ memberID: 'fixture-user', handle: 'fixture', name: 'Fixture User', email: currentEmail || 'fixture@example.com' }
		];
		this.groups = [];
		this.organizations = structuredClone(crmOrganizations);
		this.contacts = structuredClone(crmContacts);
		this.activities = structuredClone(crmActivities);
		this.nextActions = nextActionsOf(this.activities, browserTimeZone());
		this.opportunities = structuredClone(crmOpportunities).map((opportunity) =>
			withNextAction(opportunity, this.nextActions)
		);
		const pipelineNames = [...new Set(this.opportunities.map((opportunity) => opportunity.kind ?? 'sales'))];
		this.pipelines = pipelineNames.map((pipeline) => ({ pipeline, label: crmLabel(this.text.progressKinds, pipeline), direction: 'outbound', isActive: true, color: crmFixturePipelineColors[pipeline] }));
		const organizationTypeNames = [...new Set(this.organizations.flatMap((organization) => organization.types))];
		this.vocabulary = {
			organization_types: organizationTypeNames.map((organizationType) => ({
				id: organizationType,
				name: crmLabel(this.text.organizationTypes, organizationType),
				color: crmFixtureOrganizationTypeColors[organizationType]
			})),
			pipelines: this.pipelines.map((pipeline) => ({
				id: pipeline.pipeline,
				name: pipeline.label,
				direction: pipeline.direction,
				color: pipeline.color
			}))
		};
		this.taskVocabulary = {
			businesses: [...new Set(this.opportunities.map((opportunity) => opportunity.business))].map((name) => ({ name })),
			types: [...new Set(this.activities.map((activity) => activity.kind))].map((name) => ({
				name,
				color: crmFixtureActivityKindColors[name]
			}))
		};
	}
}

function ownerHint(draft: CRMCreateDraft): string {
	return draft.kind === 'progress' ? draft.ownerPersonID : '';
}
