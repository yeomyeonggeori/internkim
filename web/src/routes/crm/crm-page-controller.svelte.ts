import type { OrgGroup, UserRecord } from '$lib/organization/types';
import {
	interimCurrencyCatalogue,
	loadCurrencyCatalogue,
	type CurrencyCatalogue
} from '$lib/currency/currency-catalogue';
import { interimCompanyBaseCurrency, loadCompanyBaseCurrency } from '$lib/company/base-currency';
import { loadCRMOrganizationDirectory } from './crm-data-source';
import type { CRMReadPart } from './crm-data-source';
import type { CRMDataResponse } from './crm-api-types';
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
	hasData = $state(false);
	isDirectoryLoading = $state(false);
	isDirectoryReady = $state(false);
	directoryErrorMessage = $state('');
	isSaving = $state(false);
	permissionDenied = $state(false);
	currentEmail = $state('');
	private error = $state<unknown>(null);
	private remoteData: CRMDataResponse | undefined;
	private generation = 0;
	private mutationQueue: Promise<void> = Promise.resolve();
	private pendingMutations = 0;

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
		const generation = ++this.generation;
		this.currentEmail = currentEmail;
		this.isLoading = true;
		this.hasData = false;
		this.isSaving = false;
		this.pendingMutations = 0;
		this.mutationQueue = Promise.resolve();
		this.remoteData = undefined;
		this.people = [];
		this.groups = [];
		this.isDirectoryReady = false;
		this.directoryErrorMessage = '';
		this.error = null;
		this.permissionDenied = false;
		try {
			if (fixtureMode) {
				this.loadFixture(currentEmail);
				this.hasData = true;
				this.isDirectoryReady = true;
				return;
			}
			void this.loadDirectory(generation);
			const [data, catalogue, baseCurrency] = await Promise.all([
				loadCRMData(),
				loadCurrencyCatalogue(),
				loadCompanyBaseCurrency()
			]);
			if (generation !== this.generation) return;
			this.validateCurrencies(data, catalogue);
			this.currencyCatalogue = catalogue;
			this.companyBaseCurrency = baseCurrency;
			this.remoteData = data;
			this.applyViewData(mapCRMViewData(data, this.people, this.currencyCatalogue, browserTimeZone(), this.groups));
			this.hasData = true;
		} catch (error) {
			if (generation === this.generation) this.applyError(error);
		} finally {
			if (generation === this.generation) this.isLoading = false;
		}
	}

	dispose(): void {
		this.generation += 1;
		this.hasData = false;
		this.isDirectoryReady = false;
		this.remoteData = undefined;
	}

	async retryDirectory(): Promise<void> {
		if (this.isDirectoryLoading) return;
		await this.loadDirectory(this.generation);
	}

	private async loadDirectory(generation: number): Promise<void> {
		this.isDirectoryLoading = true;
		this.directoryErrorMessage = '';
		try {
			const directory = await loadCRMOrganizationDirectory();
			if (generation !== this.generation) return;
			this.people = directory.records ?? [];
			this.groups = directory.availableGroups ?? [];
			this.isDirectoryReady = true;
			if (this.remoteData) this.applyViewData(mapCRMViewData(this.remoteData, this.people, this.currencyCatalogue, browserTimeZone(), this.groups));
		} catch {
			if (generation === this.generation) this.directoryErrorMessage = this.text.organizationLoadFailed;
		} finally {
			if (generation === this.generation) this.isDirectoryLoading = false;
		}
	}

	async create(draft: CRMCreateDraft): Promise<void> {
		if (draft.kind === 'relationship') {
			if (fixtureMode) throw new Error(this.text.fixtureModeReadOnly);
			await this.createRelationship(draft);
			return;
		}
		await this.mutate(async (assertCurrent) => {
			if (draft.kind === 'contact') {
				const owner = resolveOwner(this.people, ownerHint(draft), this.currentEmail);
				await createCRMContact(contactPayloadFromDraft(draft, owner));
				return;
			}
			if (draft.kind === 'progress') {
				const owner = resolveOwner(this.people, ownerHint(draft), this.currentEmail);
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
				assertCurrent();
				if (this.settlesThroughCloseEndpoint(draft.stage)) {
					await transitionCRMOpportunity(created.id, transition);
				}
				return;
			}
			await createCRMActivity(activityPayloadFromDraft(draft));
		}, draft.kind === 'contact' ? ['contacts'] : ['opportunities', 'activities']);
	}

	async saveOrganization(organization: CRMOrganization): Promise<void> {
		await this.mutate(() => {
			if (!this.isDirectoryReady) {
				const existing = this.organizations.find((candidate) => candidate.id === organization.id);
				if (!existing || existing.ownerPersonID !== organization.ownerPersonID) throw new Error(this.text.organizationLoadFailed);
				return updateCRMOrganization(organization.id, organizationPayload({
					...organization, ownerPersonID: existing.ownerPersonID, ownerCircleID: existing.ownerCircleID
				}));
			}
			const owner = resolveOwner(this.people, organization.ownerPersonID ?? organization.ownerName, this.currentEmail);
			const nextOrganization = {
				...organization,
				ownerPersonID: owner.memberID,
				ownerCircleID: owner.groupID
			};
			return updateCRMOrganization(organization.id, organizationPayload(nextOrganization));
		}, ['organizations']);
	}

	async archiveOrganization(organizationID: string): Promise<void> {
		await this.mutate(() => archiveCRMOrganization(organizationID), ['organizations']);
	}

	async saveContact(contact: CRMContact): Promise<void> {
		await this.mutate(() => updateCRMContact(contact.id, contactPayload(contact)), ['contacts']);
	}

	async saveOpportunity(opportunity: CRMOpportunity): Promise<void> {
		await this.mutate(async (assertCurrent) => {
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
			assertCurrent();
			await transitionCRMOpportunity(opportunity.id, transition);
		}, ['opportunities', 'activities']);
	}

	async archiveOpportunity(opportunityID: string): Promise<void> {
		await this.mutate(() => archiveCRMOpportunity(opportunityID), ['opportunities']);
	}

	async saveVocabulary(vocabulary: CRMVocabulary): Promise<void> {
		if (fixtureMode) {
			this.vocabulary = cloneCRMVocabulary(vocabulary);
			this.pipelines = crmPipelinesOf(vocabulary);
			return;
		}
		await this.mutate(async (assertCurrent) => {
			await saveCRMVocabulary(vocabulary);
			assertCurrent();
			this.vocabulary = cloneCRMVocabulary(vocabulary);
			this.pipelines = crmPipelinesOf(vocabulary);
			if (this.remoteData) this.remoteData = { ...this.remoteData, vocabulary: this.vocabulary, pipelines: this.pipelines };
		}, []);
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
		}, ['opportunities', 'activities']);
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
		}), ['activities', 'opportunities']);
	}

	private async mutate(operation: (assertCurrent: () => void) => Promise<unknown>, changed: readonly CRMReadPart[]): Promise<void> {
		if (fixtureMode) throw new Error(this.text.fixtureModeReadOnly);
		if (!this.hasData || this.isLoading || this.permissionDenied) throw new Error(this.text.loading);
		const generation = this.generation;
		const assertCurrent = () => {
			if (generation !== this.generation) throw new Error(this.text.processingFailed);
		};
		this.pendingMutations += 1;
		this.isSaving = true;
		const pending = this.mutationQueue.then(async () => {
			assertCurrent();
			if (this.permissionDenied) throw new Error(this.errorMessage);
			this.error = null;
			try {
				await operation(assertCurrent);
			} catch (error) {
				if (generation !== this.generation) throw error;
				this.applyError(error);
				if (error instanceof CRMRelationshipContactCreateError) throw error;
				throw new Error(this.errorMessage);
			}
			assertCurrent();
			try {
				await this.reloadRemote(changed, generation);
			} catch (error) {
				if (generation === this.generation) {
					this.remoteData = undefined;
					if (error instanceof CRMApiError && (error.status === 401 || error.status === 403)) this.applyError(error);
					else this.error = new CRMPageError('refresh_after_save_failed');
				}
			}
			assertCurrent();
		});
		this.mutationQueue = pending.catch(() => {});
		try {
			await pending;
		} finally {
			if (generation === this.generation) {
				this.pendingMutations -= 1;
				this.isSaving = this.pendingMutations > 0;
			}
		}
	}

	private async reloadRemote(changed?: readonly CRMReadPart[], generation = this.generation): Promise<void> {
		const data = await loadCRMData(this.remoteData, changed);
		if (generation !== this.generation) return;
		this.validateCurrencies(data, this.currencyCatalogue);
		this.remoteData = data;
		this.applyViewData(mapCRMViewData(data, this.people, this.currencyCatalogue, browserTimeZone(), this.groups));
	}

	private validateCurrencies(data: CRMDataResponse, catalogue: CurrencyCatalogue): void {
		for (const opportunity of data.opportunities) {
			const currencies = [
				...(opportunity.amountMinor === undefined ? [] : [opportunity.currencyCode]),
				...(opportunity.baseAmountMinor === undefined ? [] : [opportunity.baseCurrencyCode])
			];
			if (currencies.some((code) => !catalogue.some((entry) => entry.code === code))) {
				throw new Error(this.text.currencyUnavailable);
			}
		}
	}

	private async createRelationship(draft: Extract<CRMCreateDraft, { kind: 'relationship' }>): Promise<void> {
		const generation = this.generation;
		await this.mutate(async (assertCurrent) => {
			const owner = resolveOwner(this.people, draft.ownerPersonID, this.currentEmail);
			try {
				await createCRMRelationshipRecords(draft, owner, this.text.relationshipContactCreateFailed, assertCurrent);
			} catch (error) {
				if (error instanceof CRMRelationshipContactCreateError && generation === this.generation) {
					try {
						await this.reloadRemote(undefined, generation);
					} catch {
						if (generation === this.generation) {
							this.remoteData = undefined;
							this.error = new CRMPageError('refresh_after_save_failed');
						}
					}
					throw error;
				}
				throw error;
			}
		}, ['organizations', 'contacts']);
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
