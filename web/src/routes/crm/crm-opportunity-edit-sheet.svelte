<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Textarea } from '$lib/components/ui/textarea';
	import type { OrgGroup, UserRecord } from '$lib/organization/types';
	import { formatAmountInput, parseAmountInput } from './crm-money';
	import CRMMoneyField from './crm-money-field.svelte';
	import CRMOwnerSelect from './crm-owner-select.svelte';
	import { opportunityAvailableTransitionStages } from './crm-opportunity-transition';
	import type {
		CRMOrganization,
		CRMContact,
		CRMCurrency,
		CRMImportance,
		CRMLostReason,
		CRMOpportunity,
		CRMPipeline,
		CRMPipelineStage
	} from './crm-types';
	import { findOrganizationByID, opportunityStageLabel } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = {
		open: boolean;
		opportunity: CRMOpportunity | undefined;
		organizations: CRMOrganization[];
		contacts: CRMContact[];
		pipelines: CRMPipeline[];
		stages: CRMPipelineStage[];
		lostReasons: CRMLostReason[];
		businessOptions: string[];
		people: UserRecord[];
		groups: OrgGroup[];
		requestedStage?: string;
		text: CRMText;
		onSave: (opportunity: CRMOpportunity) => Promise<void>;
		onArchive: (opportunityID: string) => Promise<void>;
	};

	let { open = $bindable(false), opportunity, organizations, contacts, pipelines, stages, lostReasons, businessOptions, people, groups, requestedStage, text, onSave, onArchive }: Props = $props();
	const importanceOptions: CRMImportance[] = ['high', 'medium', 'low'];
	const noContactValue = '__none__';
	let organizationID = $state('');
	let pipeline = $state('');
	let business = $state('');
	let name = $state('');
	let stage = $state('');
	let amount = $state('');
	let currency = $state<CRMCurrency>('KRW');
	let importance = $state<CRMImportance>('medium');
	let targetDate = $state('');
	let description = $state('');
	let contactID = $state('');
	let lostReason = $state('');
	let ownerPersonID = $state('');
	let errorMessage = $state('');
	let isSaving = $state(false);
	let pipelineStages = $derived(stages.filter((candidate) => candidate.pipeline === pipeline).sort((left, right) => left.position - right.position));
	let availableStages = $derived(opportunityAvailableTransitionStages(pipelineStages, opportunity?.stage ?? ''));
	let organizationContacts = $derived(contacts.filter((contact) => contact.organizationID === organizationID));
	let stageOutcome = $derived(pipelineStages.find((candidate) => candidate.stage === stage)?.outcome ?? 'open');
	let existingStageOutcome = $derived(pipelineStages.find((candidate) => candidate.stage === opportunity?.stage)?.outcome ?? 'open');
	let isRealized = $derived(existingStageOutcome === 'won' || existingStageOutcome === 'lost');
	let selectedOwner = $derived(people.find((person) => person.userID === ownerPersonID));

	function resetForm(selectedOpportunity: CRMOpportunity): void {
		organizationID = selectedOpportunity.organizationID;
		pipeline = selectedOpportunity.pipeline ?? selectedOpportunity.kind ?? pipelines[0]?.pipeline ?? '';
		business = selectedOpportunity.business;
		name = selectedOpportunity.name;
		stage = requestedStage ?? selectedOpportunity.stage;
		amount = selectedOpportunity.expectedValue === undefined ? '' : formatAmountInput(String(selectedOpportunity.expectedValue));
		currency = selectedOpportunity.currency;
		importance = selectedOpportunity.importance;
		targetDate = selectedOpportunity.targetDate;
		description = selectedOpportunity.description ?? '';
		contactID = selectedOpportunity.contacts?.[0]?.contactID ?? '';
		lostReason = selectedOpportunity.lostReason ?? '';
		ownerPersonID = selectedOpportunity.ownerPersonID ?? '';
		errorMessage = '';
		isSaving = false;
	}

	function selectOrganization(value: string): void {
		organizationID = value;
		if (!contacts.some((contact) => contact.id === contactID && contact.organizationID === value)) contactID = '';
	}

	async function save(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (!opportunity || !name.trim()) return;
		if (!organizationID && !contactID) {
			errorMessage = text.opportunityCustomerRequired;
			return;
		}
		if (!ownerPersonID) {
			errorMessage = text.selectInternalOwner;
			return;
		}
		if (stageOutcome === 'lost' && !lostReason) {
			errorMessage = text.lostReasonRequired;
			return;
		}
		isSaving = true;
		errorMessage = '';
		try {
			await onSave({
				...opportunity,
				organizationID,
				pipeline,
				business,
				name: name.trim(),
				stage,
				expectedValue: parseAmountInput(amount),
				currency,
				importance,
				targetDate,
				description: description.trim(),
				lostReason: stageOutcome === 'lost' ? lostReason : undefined,
				ownerPersonID,
				ownerCircleID: selectedOwner?.groupID,
				ownerName: selectedOwner?.name || selectedOwner?.email || opportunity.ownerName,
				contacts: contactID ? [{ contactID }] : []
			});
			open = false;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.requiredField;
		} finally {
			isSaving = false;
		}
	}

	function archive(): void {
		if (!opportunity) return;
		open = false;
		confirmDelete({
			title: text.archive,
			description: text.archiveConfirm,
			confirm: { text: text.archive },
			cancel: { text: text.cancel },
			onCancel: () => (open = true),
			onConfirm: async () => {
				isSaving = true;
				errorMessage = '';
				try {
					await onArchive(opportunity.id);
					open = false;
				} catch (error) {
					errorMessage = error instanceof Error ? error.message : text.requiredField;
					open = true;
				} finally {
					isSaving = false;
				}
			}
		});
	}

	$effect(() => {
		if (open && opportunity) resetForm(opportunity);
	});
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" closeLabel={text.cancel} class="w-full gap-0 overflow-hidden p-0 sm:max-w-xl">
		<form onsubmit={save} class="flex min-h-0 flex-1 flex-col">
			<Sheet.Header class="border-b px-4 py-4 pr-12"><Sheet.Title>{text.editOpportunity}</Sheet.Title><Sheet.Description>{text.editOpportunityDescription}</Sheet.Description></Sheet.Header>
			<div class="min-h-0 flex-1 overflow-y-auto px-4 py-5"><Field.Group>
				<Field.Field><Field.Label for="crm-edit-opportunity-organization">{text.organizationName}</Field.Label><Select.Root type="single" value={organizationID} onValueChange={selectOrganization}><Select.Trigger id="crm-edit-opportunity-organization" class="w-full">{findOrganizationByID(organizations, organizationID)?.name ?? text.selectRelationship}</Select.Trigger><Select.Content>{#each organizations as organization (organization.id)}<Select.Item value={organization.id} label={organization.name}>{organization.name}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
				<Field.Field><Field.Label for="crm-edit-opportunity-name">{text.opportunity}</Field.Label><Input id="crm-edit-opportunity-name" bind:value={name} required /></Field.Field>
					<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label>{text.progressKind}</Field.Label><Select.Root type="single" bind:value={pipeline}><Select.Trigger class="w-full">{pipelines.find((candidate) => candidate.pipeline === pipeline)?.label ?? pipeline}</Select.Trigger><Select.Content>{#each pipelines as option (option.pipeline)}<Select.Item value={option.pipeline} label={option.label}>{option.label}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field><Field.Field><Field.Label for="crm-edit-opportunity-stage">{text.stage}</Field.Label><Select.Root type="single" bind:value={stage}><Select.Trigger id="crm-edit-opportunity-stage" class="w-full">{opportunityStageLabel(pipelineStages, stage, text)}</Select.Trigger><Select.Content>{#each availableStages as option (option.stage)}<Select.Item value={option.stage} label={opportunityStageLabel(availableStages, option.stage, text)}>{opportunityStageLabel(availableStages, option.stage, text)}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field></div>
				{#if stageOutcome === 'lost'}<Field.Field><Field.Label for="crm-edit-opportunity-lost-reason">{text.lostReason}</Field.Label><Select.Root type="single" value={lostReason || noContactValue} onValueChange={(value) => (lostReason = value === noContactValue ? '' : value)}><Select.Trigger id="crm-edit-opportunity-lost-reason" class="w-full">{lostReasons.find((reason) => reason.reason === lostReason)?.label ?? text.selectLostReason}</Select.Trigger><Select.Content><Select.Item value={noContactValue} label={text.selectLostReason}>{text.selectLostReason}</Select.Item>{#each lostReasons.filter((reason) => reason.isActive) as reason (reason.reason)}<Select.Item value={reason.reason} label={reason.label}>{reason.label}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>{/if}
				<Field.Field><Field.Label>{text.business}</Field.Label><Select.Root type="single" bind:value={business}><Select.Trigger class="w-full">{business}</Select.Trigger><Select.Content>{#each businessOptions as option (option)}<Select.Item value={option} label={option}>{option}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
				<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label>{text.importance}</Field.Label><Select.Root type="single" value={importance} onValueChange={(value) => (importance = value as CRMImportance)}><Select.Trigger class="w-full">{text.importanceLabels[importance]}</Select.Trigger><Select.Content>{#each importanceOptions as option (option)}<Select.Item value={option} label={text.importanceLabels[option]}>{text.importanceLabels[option]}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field><Field.Field><Field.Label for="crm-edit-opportunity-target">{text.targetDate}</Field.Label><Input id="crm-edit-opportunity-target" type="date" bind:value={targetDate} /></Field.Field></div>
				<CRMMoneyField id="crm-edit-opportunity-amount" label={text.amount} currencyLabel={text.currency} bind:value={amount} bind:currency disabled={isRealized} />
				{#if isRealized}<p class="text-sm text-muted-foreground">{text.realizedAmountReadonly}</p>{/if}
				<Field.Field><Field.Label for="crm-edit-opportunity-contact">{text.externalContact}</Field.Label><Select.Root type="single" value={contactID || noContactValue} onValueChange={(value) => (contactID = value === noContactValue ? '' : value)}><Select.Trigger id="crm-edit-opportunity-contact" class="w-full">{organizationContacts.find((contact) => contact.id === contactID)?.name ?? text.none}</Select.Trigger><Select.Content><Select.Item value={noContactValue} label={text.none}>{text.none}</Select.Item>{#each organizationContacts as contact (contact.id)}<Select.Item value={contact.id} label={contact.name}>{contact.name}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
				<Field.Field><Field.Label for="crm-edit-opportunity-owner">{text.internalOwner}</Field.Label><CRMOwnerSelect id="crm-edit-opportunity-owner" bind:value={ownerPersonID} {people} {groups} {text} /></Field.Field>
				<Field.Field><Field.Label for="crm-edit-opportunity-details">{text.details}</Field.Label><Textarea id="crm-edit-opportunity-details" rows={8} bind:value={description} /></Field.Field>
				{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
			</Field.Group></div>
			<Sheet.Footer class="flex-row justify-between border-t bg-background px-4 py-3"><Button type="button" variant="destructive" onclick={archive} disabled={isSaving}>{text.archive}</Button><div class="flex gap-2"><Button type="button" variant="outline" onclick={() => (open = false)} disabled={isSaving}>{text.cancel}</Button><Button type="submit" disabled={isSaving}>{isSaving ? text.creating : text.save}</Button></div></Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>
