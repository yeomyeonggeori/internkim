import type { UserRecord } from '$lib/organization/types';
import { fetchOrganizationDirectory } from '../organization/organization-api';
import {
	CRMApiError,
	archiveCRMAccount,
	archiveCRMOpportunity,
	createCRMAccount,
	createCRMActivity,
	createCRMContact,
	createCRMOpportunity,
	loadCRMData,
	positionCRMOpportunity,
	transitionCRMOpportunity,
	updateCRMAccount,
	updateCRMActivity,
	updateCRMContact,
	updateCRMOpportunity
} from './crm-api';
import {
	accountPayload,
	accountPayloadFromDraft,
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
import { crmAccounts, crmActivities, crmContacts, crmNextActions, crmOpportunities } from './dev-crm-fixture';
import type {
	CRMAccount,
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
import type { CRMTransitionPayload } from './crm-api-types';
import type { CRMPipelineBoardMoveRequest } from './crm-pipeline-board-drag';
import { crmErrorMessage, CRMPageError } from './crm-error-text';
import {
	opportunityTransitionOutcome,
	type CRMOpportunityTransitionValues
} from './crm-opportunity-transition';
import type { CRMText } from './text';

const fixtureMode = import.meta.env.VITE_MOCK_CRM === '1';

export class CRMPageController {
	accounts = $state<CRMAccount[]>([]);
	contacts = $state<CRMContact[]>([]);
	opportunities = $state<CRMOpportunity[]>([]);
	activities = $state<CRMActivity[]>([]);
	nextActions = $state<CRMNextAction[]>([]);
	pipelines = $state<CRMPipeline[]>([]);
	stages = $state<CRMPipelineStage[]>([]);
	lostReasons = $state<CRMLostReason[]>([]);
	people = $state<UserRecord[]>([]);
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

	get businessOptions(): string[] {
		const values = this.opportunities.map((opportunity) => opportunity.business).filter(Boolean);
		return [...new Set(['general', ...values])];
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
			this.applyViewData(mapCRMViewData(data, this.people));
		} catch (error) {
			this.applyError(error);
		} finally {
			this.isLoading = false;
		}
	}

	async create(draft: CRMCreateDraft): Promise<void> {
		await this.mutate(async () => {
			const owner = resolveOwner(this.people, ownerHint(draft), this.currentEmail);
			if (draft.kind === 'relationship') {
				await createCRMAccount(accountPayloadFromDraft(draft, owner));
				return;
			}
			if (draft.kind === 'contact') {
				await createCRMContact(contactPayloadFromDraft(draft, owner));
				return;
			}
			if (draft.kind === 'progress') {
				const payload = opportunityPayloadFromDraft(draft, owner, browserTimeZone());
				await createCRMOpportunity({
					...payload,
					transition: {
						...this.transitionPayload('', draft.stage, null, draft.progressKind, {
							amountMinor: payload.amountMinor,
							currencyCode: payload.currencyCode,
							baseAmountMinor: null,
							baseCurrencyCode: '',
							lostReason: draft.lostReason
						}),
						stagePosition: 0
					}
				});
				return;
			}
			await createCRMActivity(activityPayloadFromDraft(draft));
		});
	}

	async saveAccount(account: CRMAccount): Promise<void> {
		await this.mutate(() => {
			const owner = resolveOwner(this.people, account.ownerName, this.currentEmail);
			const nextAccount = {
				...account,
				ownerPersonID: owner.userID,
				ownerCircleID: account.ownerPersonID === owner.userID ? account.ownerCircleID : undefined
			};
			return updateCRMAccount(account.id, accountPayload(nextAccount));
		});
	}

	async archiveAccount(accountID: string): Promise<void> {
		await this.mutate(() => archiveCRMAccount(accountID));
	}

	async saveContact(contact: CRMContact): Promise<void> {
		await this.mutate(() => updateCRMContact(contact.id, contactPayload(contact)));
	}

	async saveOpportunity(opportunity: CRMOpportunity): Promise<void> {
		await this.mutate(async () => {
			const existing = this.opportunities.find((candidate) => candidate.id === opportunity.id);
			const payload = opportunityPayload(opportunity);
			await updateCRMOpportunity(opportunity.id, {
				...payload,
				transition: existing && opportunity.stage !== existing.stage
					? this.transitionPayload(opportunity.id, opportunity.stage, null, opportunity.pipeline, {
						amountMinor: payload.amountMinor,
						currencyCode: payload.currencyCode,
						baseAmountMinor: opportunity.baseAmountMinor ?? null,
						baseCurrencyCode: opportunity.baseCurrencyCode ?? '',
						lostReason: opportunity.lostReason ?? ''
					})
					: undefined
			});
		});
	}

	async archiveOpportunity(opportunityID: string): Promise<void> {
		await this.mutate(() => archiveCRMOpportunity(opportunityID));
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
			accountID: draft.accountID,
			opportunityID: draft.opportunityID,
			business: draft.business,
			kind: draft.kind,
			title: draft.title,
			occurredAt: new Date(draft.occurredAt).toISOString(),
			summary: draft.summary
		};
		await this.mutate(() => updateCRMActivity(activity.id, activityPayload(updated)));
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
		this.applyViewData(mapCRMViewData(data, this.people));
	}

	private applyViewData(data: CRMViewData): void {
		this.accounts = data.accounts;
		this.contacts = data.contacts;
		this.opportunities = data.opportunities;
		this.activities = data.activities;
		this.nextActions = data.nextActions;
		this.pipelines = data.pipelines;
		this.stages = data.stages;
		this.lostReasons = data.lostReasons;
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
		const pipeline = opportunity?.pipeline ?? opportunity?.kind ?? pipelineHint ?? 'sales';
		return {
			stage,
			stagePosition: this.nextPosition(opportunity, beforeOpportunityID, stage, pipelineHint),
			beforeOpportunityID: beforeOpportunityID ?? '',
			occurredAt: new Date().toISOString(),
			...opportunityTransitionOutcome(this.stages, pipeline, stage, values)
		};
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
		this.people = [{ userID: 'fixture-user', handle: 'fixture', name: crmAccounts[0]?.ownerName ?? 'Fixture User', email: currentEmail || 'fixture@example.com' }];
		this.accounts = structuredClone(crmAccounts);
		this.contacts = structuredClone(crmContacts);
		this.opportunities = structuredClone(crmOpportunities);
		this.activities = structuredClone(crmActivities);
		this.nextActions = structuredClone(crmNextActions);
		const pipelineNames = [...new Set(this.opportunities.map((opportunity) => opportunity.kind ?? 'sales'))];
		this.pipelines = pipelineNames.map((pipeline) => ({ pipeline, label: this.text.progressKinds[pipeline], direction: 'outbound', isActive: true }));
		this.stages = pipelineNames.flatMap((pipeline) => [...new Set(this.opportunities
			.filter((opportunity) => (opportunity.kind ?? 'sales') === pipeline)
			.map((opportunity) => opportunity.stage))]
			.map((stage, index) => ({ pipeline, stage, position: index + 1, outcome: fixtureOutcome(stage) })));
		this.lostReasons = [];
	}
}

function ownerHint(draft: CRMCreateDraft): string {
	return draft.kind === 'relationship' || draft.kind === 'progress' ? draft.ownerName : '';
}

function fixtureOutcome(stage: string): CRMPipelineStage['outcome'] {
	if (stage === 'won' || stage === 'closed' || stage === 'contract_award') return 'won';
	if (stage === 'lost') return 'lost';
	if (stage === 'on_hold') return 'on_hold';
	return 'open';
}
