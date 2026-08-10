<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import { TagsInput } from '$lib/components/ui/tags-input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { activityReferenceAfterAccountChange, activityReferenceAfterOpportunityChange } from './crm-activity-reference';
	import { currentCRMDate } from './crm-date';
	import { parseAmountInput } from './crm-money';
	import CRMMoneyField from './crm-money-field.svelte';
	import type {
		CRMAccount,
		CRMAccountStatus,
		CRMAccountType,
		CRMActivityKind,
		CRMCurrency,
		CRMCreateDraft,
		CRMImportance,
		CRMLostReason,
		CRMOpportunity,
		CRMPipeline,
		CRMPipelineStage,
		CRMProgressKind,
		CRMRecordKind
	} from './crm-types';
	import { findAccountByID, opportunityStageLabel } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = {
		open: boolean;
		initialKind: CRMRecordKind;
		accounts: CRMAccount[];
		opportunities: CRMOpportunity[];
		pipelines: CRMPipeline[];
		stages: CRMPipelineStage[];
		lostReasons: CRMLostReason[];
		businessOptions: string[];
		defaultOwnerName: string;
		text: CRMText;
		onCreate: (draft: CRMCreateDraft) => Promise<void>;
	};

	let { open = $bindable(false), initialKind, accounts, opportunities, pipelines, stages, lostReasons, businessOptions, defaultOwnerName, text, onCreate }: Props = $props();
	const accountTypes: CRMAccountType[] = ['customer', 'partner', 'sponsor', 'vendor', 'investor', 'other'];
	const accountStatuses: CRMAccountStatus[] = ['prospect', 'active', 'paused'];
	const importanceOptions: CRMImportance[] = ['high', 'medium', 'low'];
	const activityKinds: Array<Exclude<CRMActivityKind, 'stage_change'>> = ['note', 'email', 'meeting', 'call', 'task', 'file', 'event'];
	const noLostReasonValue = '__none__';
	let kind = $state<CRMRecordKind>('relationship');
	let name = $state('');
	let accountID = $state('');
	let accountType = $state<CRMAccountType>('customer');
	let status = $state<CRMAccountStatus>('prospect');
	let importance = $state<CRMImportance>('medium');
	let ownerName = $state('');
	let team = $state('');
	let address = $state('');
	let tags = $state<string[]>([]);
	let description = $state('');
	let contactTitle = $state('');
	let email = $state('');
	let phone = $state('');
	let isPrimary = $state(false);
	let business = $state('general');
	let pipeline = $state<CRMProgressKind>('sales');
	let stage = $state('lead');
	let lostReason = $state('');
	let amount = $state('');
	let currency = $state<CRMCurrency>('KRW');
	let targetDate = $state('');
	let opportunityID = $state('');
	let activityKind = $state<Exclude<CRMActivityKind, 'stage_change'>>('note');
	let occurredAt = $state('');
	let errorMessage = $state('');
	let isSaving = $state(false);
	let pipelineStages = $derived(stages.filter((candidate) => candidate.pipeline === pipeline).sort((left, right) => left.position - right.position));
	let relatedOpportunities = $derived(opportunities.filter((opportunity) => opportunity.accountID === accountID));
	let stageOutcome = $derived(pipelineStages.find((candidate) => candidate.stage === stage)?.outcome ?? 'open');

	function resetForm(): void {
		kind = initialKind;
		name = '';
		accountID = accounts[0]?.id ?? '';
		accountType = 'customer';
		status = 'prospect';
		importance = 'medium';
		ownerName = defaultOwnerName;
		team = '';
		address = '';
		tags = [];
		description = '';
		contactTitle = '';
		email = '';
		phone = '';
		isPrimary = false;
		business = businessOptions[0] ?? 'general';
		pipeline = pipelines[0]?.pipeline ?? 'sales';
		stage = stages.find((candidate) => candidate.pipeline === pipeline)?.stage ?? 'lead';
		lostReason = '';
		amount = '';
		currency = 'KRW';
		targetDate = currentCRMDate();
		opportunityID = '';
		activityKind = 'note';
		occurredAt = localDateTimeValue(new Date());
		errorMessage = '';
		isSaving = false;
	}

	function selectPipeline(value: string): void {
		pipeline = value as CRMProgressKind;
		stage = stages.find((candidate) => candidate.pipeline === pipeline)?.stage ?? '';
	}

	function selectAccount(value: string): void {
		if (kind !== 'activity') {
			accountID = value;
			return;
		}
		const reference = activityReferenceAfterAccountChange(opportunities, value, opportunityID, business);
		accountID = reference.accountID;
		opportunityID = reference.opportunityID;
		business = reference.business;
	}

	function selectOpportunity(value: string): void {
		const reference = activityReferenceAfterOpportunityChange(opportunities, value, accountID, business);
		accountID = reference.accountID;
		opportunityID = reference.opportunityID;
		business = reference.business;
	}

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (!name.trim()) {
			errorMessage = text.requiredField;
			return;
		}
		if (kind === 'progress' && stageOutcome === 'lost' && !lostReason) {
			errorMessage = text.lostReasonRequired;
			return;
		}
		isSaving = true;
		errorMessage = '';
		try {
			if (kind === 'relationship') {
				await onCreate({ kind, name: name.trim(), accountType, status, importance, ownerName: ownerName.trim(), team: team.trim(), address: address.trim(), tags, description: description.trim(), lastContactDate: '', nextActionDate: '' });
			} else if (kind === 'contact') {
				await onCreate({ kind, accountID, name: name.trim(), title: contactTitle.trim(), email: email.trim(), phone: phone.trim(), isPrimary, note: description.trim() });
			} else if (kind === 'progress') {
				await onCreate({ kind, accountID, business, name: name.trim(), progressKind: pipeline, stage, lostReason: stageOutcome === 'lost' ? lostReason : '', ownerName: ownerName.trim(), amount: parseAmountInput(amount), currency, importance, targetDate, description: description.trim(), calendar: { isRequested: false, isAllDay: true, startTime: '', endTime: '', location: '' } });
			} else {
				await onCreate({ kind, accountID, opportunityID: opportunityID || undefined, business, activityKind, title: name.trim(), occurredAt, summary: description.trim(), taskOwnerID: '', taskStatus: '', calendar: { isRequested: false, isAllDay: false, startTime: '', endTime: '', location: '' } });
			}
			open = false;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.requiredField;
		} finally {
			isSaving = false;
		}
	}

	$effect(() => {
		if (open) resetForm();
	});

	function localDateTimeValue(date: Date): string {
		const offset = date.getTimezoneOffset() * 60000;
		return new Date(date.getTime() - offset).toISOString().slice(0, 16);
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" closeLabel={text.cancel} class="w-full gap-0 overflow-hidden p-0 sm:max-w-xl">
		<form onsubmit={submit} class="flex min-h-0 flex-1 flex-col">
			<Sheet.Header class="border-b px-4 py-4 pr-12"><Sheet.Title>{text.createTitle}</Sheet.Title><Sheet.Description>{text.createDescription}</Sheet.Description></Sheet.Header>
			<div class="min-h-0 flex-1 overflow-y-auto px-4 py-5">
				<Field.Group>
					<Field.Field><Field.Label for="crm-record-kind">{text.recordKind}</Field.Label><Select.Root type="single" value={kind} onValueChange={(value) => (kind = value as CRMRecordKind)}><Select.Trigger id="crm-record-kind" class="w-full">{kind === 'relationship' ? text.newRelationship : kind === 'contact' ? text.newContact : kind === 'progress' ? text.newOpportunity : text.logActivity}</Select.Trigger><Select.Content><Select.Item value="relationship" label={text.newRelationship}>{text.newRelationship}</Select.Item><Select.Item value="contact" label={text.newContact}>{text.newContact}</Select.Item><Select.Item value="progress" label={text.newOpportunity}>{text.newOpportunity}</Select.Item><Select.Item value="activity" label={text.logActivity}>{text.logActivity}</Select.Item></Select.Content></Select.Root></Field.Field>
					{#if kind !== 'relationship'}
							<Field.Field><Field.Label for="crm-record-account">{text.accountName}</Field.Label><Select.Root type="single" value={accountID} onValueChange={selectAccount}><Select.Trigger id="crm-record-account" class="w-full">{findAccountByID(accounts, accountID)?.name ?? text.selectRelationship}</Select.Trigger><Select.Content>{#each accounts as account (account.id)}<Select.Item value={account.id} label={account.name}>{account.name}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
					{/if}
					<Field.Field><Field.Label for="crm-record-name">{kind === 'contact' ? text.contactName : kind === 'activity' ? text.activityTitle : text.name}</Field.Label><Input id="crm-record-name" bind:value={name} required /></Field.Field>
					{#if kind === 'relationship'}
						<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label>{text.type}</Field.Label><Select.Root type="single" value={accountType} onValueChange={(value) => (accountType = value as CRMAccountType)}><Select.Trigger class="w-full">{text.accountTypes[accountType]}</Select.Trigger><Select.Content>{#each accountTypes as option (option)}<Select.Item value={option} label={text.accountTypes[option]}>{text.accountTypes[option]}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field><Field.Field><Field.Label>{text.status}</Field.Label><Select.Root type="single" value={status} onValueChange={(value) => (status = value as CRMAccountStatus)}><Select.Trigger class="w-full">{text.accountStatuses[status]}</Select.Trigger><Select.Content>{#each accountStatuses as option (option)}<Select.Item value={option} label={text.accountStatuses[option]}>{text.accountStatuses[option]}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field></div>
						<Field.Field><Field.Label for="crm-record-owner">{text.owner}</Field.Label><Input id="crm-record-owner" bind:value={ownerName} /></Field.Field>
						<Field.Field><Field.Label for="crm-record-address">{text.address}</Field.Label><Input id="crm-record-address" bind:value={address} /></Field.Field>
						<Field.Field><Field.Label for="crm-record-tags">{text.tags}</Field.Label><TagsInput id="crm-record-tags" bind:value={tags} /></Field.Field>
					{:else if kind === 'contact'}
						<Field.Field><Field.Label for="crm-record-contact-title">{text.contactTitle}</Field.Label><Input id="crm-record-contact-title" bind:value={contactTitle} /></Field.Field>
						<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label for="crm-record-email">{text.email}</Field.Label><Input id="crm-record-email" type="email" bind:value={email} /></Field.Field><Field.Field><Field.Label for="crm-record-phone">{text.phone}</Field.Label><Input id="crm-record-phone" type="tel" bind:value={phone} /></Field.Field></div>
						<Field.Field orientation="horizontal"><Checkbox id="crm-record-primary" bind:checked={isPrimary} /><Field.Content><Field.Label for="crm-record-primary">{text.markAsPrimaryContact}</Field.Label></Field.Content></Field.Field>
					{:else if kind === 'progress'}
						<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label>{text.progressKind}</Field.Label><Select.Root type="single" value={pipeline} onValueChange={selectPipeline}><Select.Trigger class="w-full">{pipelines.find((candidate) => candidate.pipeline === pipeline)?.label ?? pipeline}</Select.Trigger><Select.Content>{#each pipelines as option (option.pipeline)}<Select.Item value={option.pipeline} label={option.label}>{option.label}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field><Field.Field><Field.Label for="crm-record-stage">{text.stage}</Field.Label><Select.Root type="single" bind:value={stage}><Select.Trigger id="crm-record-stage" class="w-full">{opportunityStageLabel(stage, text)}</Select.Trigger><Select.Content>{#each pipelineStages as option (option.stage)}<Select.Item value={option.stage} label={opportunityStageLabel(option.stage, text)}>{opportunityStageLabel(option.stage, text)}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field></div>
						{#if stageOutcome === 'lost'}<Field.Field><Field.Label for="crm-record-lost-reason">{text.lostReason}</Field.Label><Select.Root type="single" value={lostReason || noLostReasonValue} onValueChange={(value) => (lostReason = value === noLostReasonValue ? '' : value)}><Select.Trigger id="crm-record-lost-reason" class="w-full">{lostReasons.find((reason) => reason.reason === lostReason)?.label ?? text.selectLostReason}</Select.Trigger><Select.Content><Select.Item value={noLostReasonValue} label={text.selectLostReason}>{text.selectLostReason}</Select.Item>{#each lostReasons.filter((reason) => reason.isActive) as reason (reason.reason)}<Select.Item value={reason.reason} label={reason.label}>{reason.label}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>{/if}
						<Field.Field><Field.Label>{text.business}</Field.Label><Select.Root type="single" bind:value={business}><Select.Trigger class="w-full">{business}</Select.Trigger><Select.Content>{#each businessOptions as option (option)}<Select.Item value={option} label={option}>{option}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
						<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label>{text.importance}</Field.Label><Select.Root type="single" value={importance} onValueChange={(value) => (importance = value as CRMImportance)}><Select.Trigger class="w-full">{text.importanceLabels[importance]}</Select.Trigger><Select.Content>{#each importanceOptions as option (option)}<Select.Item value={option} label={text.importanceLabels[option]}>{text.importanceLabels[option]}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field><Field.Field><Field.Label for="crm-record-target">{text.targetDate}</Field.Label><Input id="crm-record-target" type="date" bind:value={targetDate} /></Field.Field></div>
						<CRMMoneyField id="crm-record-amount" label={text.amount} currencyLabel={text.currency} bind:value={amount} bind:currency />
						<Field.Field><Field.Label for="crm-record-progress-owner">{text.progressOwner}</Field.Label><Input id="crm-record-progress-owner" bind:value={ownerName} /></Field.Field>
					{:else}
							<Field.Field><Field.Label for="crm-record-opportunity">{text.relatedProgress}</Field.Label><Select.Root type="single" value={opportunityID} onValueChange={selectOpportunity}><Select.Trigger id="crm-record-opportunity" class="w-full">{relatedOpportunities.find((opportunity) => opportunity.id === opportunityID)?.name ?? text.noRelatedProgress}</Select.Trigger><Select.Content><Select.Item value="" label={text.noRelatedProgress}>{text.noRelatedProgress}</Select.Item>{#each relatedOpportunities as opportunity (opportunity.id)}<Select.Item value={opportunity.id} label={opportunity.name}>{opportunity.name}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
						<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label>{text.activityKind}</Field.Label><Select.Root type="single" value={activityKind} onValueChange={(value) => (activityKind = value as Exclude<CRMActivityKind, 'stage_change'>)}><Select.Trigger class="w-full">{text.activityKinds[activityKind]}</Select.Trigger><Select.Content>{#each activityKinds as option (option)}<Select.Item value={option} label={text.activityKinds[option]}>{text.activityKinds[option]}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field><Field.Field><Field.Label for="crm-record-occurred">{text.occurredAt}</Field.Label><Input id="crm-record-occurred" type="datetime-local" bind:value={occurredAt} /></Field.Field></div>
							<Field.Field><Field.Label for="crm-record-activity-business">{text.business}</Field.Label><Select.Root type="single" bind:value={business} disabled={opportunityID !== ''}><Select.Trigger id="crm-record-activity-business" class="w-full">{business}</Select.Trigger><Select.Content>{#each businessOptions as option (option)}<Select.Item value={option} label={option}>{option}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
					{/if}
					<Field.Field><Field.Label for="crm-record-details">{text.details}</Field.Label><Textarea id="crm-record-details" rows={6} bind:value={description} /></Field.Field>
					{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
				</Field.Group>
			</div>
			<Sheet.Footer class="flex-row justify-end border-t bg-background px-4 py-3"><Button type="button" variant="outline" onclick={() => (open = false)} disabled={isSaving}>{text.cancel}</Button><Button type="submit" disabled={isSaving}>{isSaving ? text.creating : text.create}</Button></Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>
