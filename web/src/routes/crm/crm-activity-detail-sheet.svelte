<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Textarea } from '$lib/components/ui/textarea';
	import { activityReferenceAfterAccountChange, activityReferenceAfterOpportunityChange } from './crm-activity-reference';
	import type { CRMAccount, CRMActivity, CRMActivityEditDraft, CRMActivityKind, CRMOpportunity } from './crm-types';
	import { findAccountByID } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = {
		open: boolean;
		activity: CRMActivity | undefined;
		accounts: CRMAccount[];
		opportunities: CRMOpportunity[];
		businessOptions: string[];
		text: CRMText;
		onSave: (activity: CRMActivity, draft: CRMActivityEditDraft) => Promise<void>;
	};

	let { open = $bindable(false), activity, accounts, opportunities, businessOptions, text, onSave }: Props = $props();
	const unlinkedOpportunityValue = 'unlinked';
	const activityKinds: Array<Exclude<CRMActivityKind, 'stage_change'>> = ['note', 'email', 'meeting', 'call', 'task', 'file', 'event'];
	let accountID = $state('');
	let opportunityID = $state(unlinkedOpportunityValue);
	let business = $state('');
	let title = $state('');
	let kind = $state<CRMActivityKind>('note');
	let occurredAt = $state('');
	let summary = $state('');
	let isSaving = $state(false);
	let errorMessage = $state('');
	let relatedOpportunities = $derived(opportunities.filter((opportunity) => opportunity.accountID === accountID));
	let isSystemActivity = $derived(activity?.kind === 'stage_change');

	function resetForm(selectedActivity: CRMActivity): void {
		accountID = selectedActivity.accountID;
		opportunityID = selectedActivity.opportunityID ?? unlinkedOpportunityValue;
		business = selectedActivity.business;
		title = selectedActivity.title;
		kind = selectedActivity.kind;
		occurredAt = dateTimeLocalValue(selectedActivity.occurredAt);
		summary = selectedActivity.summary;
		isSaving = false;
		errorMessage = '';
	}

	function selectAccount(value: string): void {
		const reference = activityReferenceAfterAccountChange(
			opportunities,
			value,
			opportunityID === unlinkedOpportunityValue ? '' : opportunityID,
			business
		);
		accountID = reference.accountID;
		opportunityID = reference.opportunityID || unlinkedOpportunityValue;
		business = reference.business;
	}

	function selectOpportunity(value: string): void {
		const reference = activityReferenceAfterOpportunityChange(
			opportunities,
			value === unlinkedOpportunityValue ? '' : value,
			accountID,
			business
		);
		accountID = reference.accountID;
		opportunityID = reference.opportunityID || unlinkedOpportunityValue;
		business = reference.business;
	}

	async function save(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (!activity || isSystemActivity || !title.trim()) return;
		isSaving = true;
		errorMessage = '';
		try {
			await onSave(activity, {
				accountID,
				opportunityID: opportunityID === unlinkedOpportunityValue ? undefined : opportunityID,
				business,
				kind,
				title: title.trim(),
				occurredAt,
				summary: summary.trim(),
				taskOwnerID: '',
				taskStatus: '',
				shouldUpdateLinkedCalendar: false
			});
			open = false;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.requiredField;
		} finally {
			isSaving = false;
		}
	}

	$effect(() => {
		if (open && activity) resetForm(activity);
	});

	function dateTimeLocalValue(value: string): string {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return '';
		return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" closeLabel={text.cancel} class="w-full gap-0 overflow-hidden p-0 sm:max-w-xl">
		<form onsubmit={save} class="flex min-h-0 flex-1 flex-col">
			<Sheet.Header class="border-b px-4 py-4 pr-12"><Sheet.Title>{text.editActivity}</Sheet.Title><Sheet.Description>{text.editActivityDescription}</Sheet.Description></Sheet.Header>
			<div class="min-h-0 flex-1 overflow-y-auto px-4 py-5"><Field.Group>
				{#if isSystemActivity}<p class="rounded-md border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">{text.systemActivityReadonly}</p>{/if}
					<Field.Field><Field.Label for="crm-activity-account">{text.accountName}</Field.Label><Select.Root type="single" value={accountID} onValueChange={selectAccount} disabled={isSystemActivity}><Select.Trigger id="crm-activity-account" class="w-full">{findAccountByID(accounts, accountID)?.name ?? text.selectRelationship}</Select.Trigger><Select.Content>{#each accounts as account (account.id)}<Select.Item value={account.id} label={account.name}>{account.name}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
					<Field.Field><Field.Label for="crm-activity-opportunity">{text.relatedProgress}</Field.Label><Select.Root type="single" value={opportunityID} onValueChange={selectOpportunity} disabled={isSystemActivity}><Select.Trigger id="crm-activity-opportunity" class="w-full">{relatedOpportunities.find((opportunity) => opportunity.id === opportunityID)?.name ?? text.noRelatedProgress}</Select.Trigger><Select.Content><Select.Item value={unlinkedOpportunityValue} label={text.noRelatedProgress}>{text.noRelatedProgress}</Select.Item>{#each relatedOpportunities as opportunity (opportunity.id)}<Select.Item value={opportunity.id} label={opportunity.name}>{opportunity.name}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
				<Field.Field><Field.Label for="crm-activity-title">{text.activityTitle}</Field.Label><Input id="crm-activity-title" bind:value={title} disabled={isSystemActivity} required /></Field.Field>
				<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label>{text.activityKind}</Field.Label>{#if isSystemActivity}<Input value={text.activityKinds.stage_change} disabled />{:else}<Select.Root type="single" value={kind} onValueChange={(value) => (kind = value as Exclude<CRMActivityKind, 'stage_change'>)}><Select.Trigger class="w-full">{text.activityKinds[kind]}</Select.Trigger><Select.Content>{#each activityKinds as option (option)}<Select.Item value={option} label={text.activityKinds[option]}>{text.activityKinds[option]}</Select.Item>{/each}</Select.Content></Select.Root>{/if}</Field.Field><Field.Field><Field.Label for="crm-activity-occurred">{text.occurredAt}</Field.Label><Input id="crm-activity-occurred" type="datetime-local" bind:value={occurredAt} disabled={isSystemActivity} /></Field.Field></div>
					<Field.Field><Field.Label for="crm-activity-business">{text.business}</Field.Label><Select.Root type="single" bind:value={business} disabled={isSystemActivity || opportunityID !== unlinkedOpportunityValue}><Select.Trigger id="crm-activity-business" class="w-full">{business}</Select.Trigger><Select.Content>{#each businessOptions as option (option)}<Select.Item value={option} label={option}>{option}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
				<Field.Field><Field.Label for="crm-activity-summary">{text.details}</Field.Label><Textarea id="crm-activity-summary" rows={8} bind:value={summary} disabled={isSystemActivity} /></Field.Field>
				{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
			</Field.Group></div>
			<Sheet.Footer class="flex-row justify-end border-t bg-background px-4 py-3"><Button type="button" variant="outline" onclick={() => (open = false)} disabled={isSaving}>{text.cancel}</Button>{#if !isSystemActivity}<Button type="submit" disabled={isSaving}>{isSaving ? text.creating : text.save}</Button>{/if}</Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>
