<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Textarea } from '$lib/components/ui/textarea';
	import type { OrgGroup, UserRecord } from '$lib/organization/types';
	import { activityReferenceAfterOrganizationChange, activityReferenceAfterOpportunityChange } from './crm-activity-reference';
	import { crmLabel } from './crm-labels';
	import { centralTaskStatusOptions, taskStatus } from '$lib/task/central-task';
	import CRMOwnerSelect from './crm-owner-select.svelte';
	import CRMContactSelect from './crm-contact-select.svelte';
	import type { CRMOrganization, CRMActivity, CRMActivityEditDraft, CRMActivityKind, CRMContact, CRMOpportunity } from './crm-types';
	import { findOrganizationByID } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = {
		open: boolean;
		activity: CRMActivity | undefined;
		organizations: CRMOrganization[];
		opportunities: CRMOpportunity[];
		contacts: CRMContact[];
		businessOptions: string[];
		activityKindOptions: CRMActivityKind[];
		people: UserRecord[];
		groups: OrgGroup[];
		text: CRMText;
		onSave: (activity: CRMActivity, draft: CRMActivityEditDraft) => Promise<void>;
	};

	let { open = $bindable(false), activity, organizations, opportunities, contacts, businessOptions, activityKindOptions, people, groups, text, onSave }: Props = $props();
	const unlinkedOpportunityValue = 'unlinked';
	const noContactValue = 'unlinked-contact';
	let activityKinds = $derived(activityKindOptions.filter((kind) => kind !== 'stage_change'));
	let organizationID = $state('');
	let contactID = $state('');
	let opportunityID = $state(unlinkedOpportunityValue);
	let business = $state('');
	let title = $state('');
	let kind = $state<CRMActivityKind>('note');
	let occurredAt = $state('');
	let summary = $state('');
	let taskOwnerID = $state('');
	let activityStatus = $state<string>(taskStatus.planned);
	let isEvent = $state(false);
	let isWholeDay = $state(false);
	let startsAt = $state('');
	let endsAt = $state('');
	let location = $state('');
	let isSaving = $state(false);
	let errorMessage = $state('');
	let relatedOpportunities = $derived(opportunities.filter((opportunity) => opportunity.organizationID === organizationID));
	let organizationContacts = $derived(contacts.filter((contact) => contact.organizationID === organizationID));

	function resetForm(selectedActivity: CRMActivity): void {
		organizationID = selectedActivity.organizationID;
		contactID = selectedActivity.contactID ?? '';
		opportunityID = selectedActivity.opportunityID ?? unlinkedOpportunityValue;
		business = selectedActivity.business;
		title = selectedActivity.title;
		kind = selectedActivity.kind;
		occurredAt = dateTimeLocalValue(selectedActivity.occurredAt);
		summary = selectedActivity.summary;
		taskOwnerID = selectedActivity.taskOwnerID ?? '';
		activityStatus = selectedActivity.taskStatus ?? taskStatus.planned;
		isEvent = Boolean(selectedActivity.calendarEventID);
		isWholeDay = selectedActivity.isWholeDay ?? false;
		startsAt = dateTimeLocalValue(selectedActivity.calendarEventDate ?? selectedActivity.occurredAt);
		endsAt = dateTimeLocalValue(selectedActivity.calendarEndsAt ?? selectedActivity.calendarEventDate ?? selectedActivity.occurredAt);
		location = selectedActivity.calendarLocation ?? '';
		isSaving = false;
		errorMessage = '';
	}

	function selectOrganization(value: string): void {
		const reference = activityReferenceAfterOrganizationChange(
			opportunities,
			value,
			opportunityID === unlinkedOpportunityValue ? '' : opportunityID,
			business
		);
		organizationID = reference.organizationID;
		opportunityID = reference.opportunityID || unlinkedOpportunityValue;
		business = reference.business;
		if (!contacts.some((contact) => contact.id === contactID && contact.organizationID === organizationID)) contactID = '';
	}

	function selectOpportunity(value: string): void {
		const reference = activityReferenceAfterOpportunityChange(
			opportunities,
			value === unlinkedOpportunityValue ? '' : value,
			organizationID,
			business
		);
		organizationID = reference.organizationID;
		opportunityID = reference.opportunityID || unlinkedOpportunityValue;
		business = reference.business;
		if (!contacts.some((contact) => contact.id === contactID && contact.organizationID === organizationID)) contactID = '';
	}

	async function save(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (!activity || !title.trim()) return;
		if (isEvent && startsAt && endsAt && endsAt < startsAt) {
			errorMessage = text.endTimeAfterStart;
			return;
		}
		isSaving = true;
		errorMessage = '';
		try {
			await onSave(activity, {
				organizationID,
				contactID: contactID || undefined,
				opportunityID: opportunityID === unlinkedOpportunityValue ? undefined : opportunityID,
				business,
				kind,
				title: title.trim(),
				occurredAt,
				summary: summary.trim(),
				taskOwnerID,
				taskStatus: activityStatus,
				isEvent,
				isWholeDay,
				startsAt,
				endsAt,
				location: location.trim()
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
						<Field.Field><Field.Label for="crm-activity-organization">{text.organizationName}</Field.Label><Select.Root type="single" value={organizationID} onValueChange={selectOrganization}><Select.Trigger id="crm-activity-organization" class="w-full">{findOrganizationByID(organizations, organizationID)?.name ?? text.selectRelationship}</Select.Trigger><Select.Content>{#each organizations as organization (organization.id)}<Select.Item value={organization.id} label={organization.name}>{organization.name}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
						<Field.Field><Field.Label for="crm-activity-opportunity">{text.relatedProgress}</Field.Label><Select.Root type="single" value={opportunityID} onValueChange={selectOpportunity}><Select.Trigger id="crm-activity-opportunity" class="w-full">{relatedOpportunities.find((opportunity) => opportunity.id === opportunityID)?.name ?? text.noRelatedProgress}</Select.Trigger><Select.Content><Select.Item value={unlinkedOpportunityValue} label={text.noRelatedProgress}>{text.noRelatedProgress}</Select.Item>{#each relatedOpportunities as opportunity (opportunity.id)}<Select.Item value={opportunity.id} label={opportunity.name}>{opportunity.name}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
					<Field.Field><Field.Label for="crm-activity-title">{text.activityTitle}</Field.Label><Input id="crm-activity-title" bind:value={title} required /></Field.Field>
					<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label>{text.activityKind}</Field.Label><Select.Root type="single" value={kind} onValueChange={(value) => (kind = value as Exclude<CRMActivityKind, 'stage_change'>)}><Select.Trigger class="w-full">{crmLabel(text.activityKinds, kind)}</Select.Trigger><Select.Content>{#each activityKinds as option (option)}<Select.Item value={option} label={crmLabel(text.activityKinds, option)}>{crmLabel(text.activityKinds, option)}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field><Field.Field><Field.Label for="crm-activity-occurred">{text.occurredAt}</Field.Label><Input id="crm-activity-occurred" type="datetime-local" bind:value={occurredAt} /></Field.Field></div>
						<Field.Field><Field.Label for="crm-activity-business">{text.business}</Field.Label><Select.Root type="single" bind:value={business} disabled={opportunityID !== unlinkedOpportunityValue}><Select.Trigger id="crm-activity-business" class="w-full">{business}</Select.Trigger><Select.Content>{#each businessOptions as option (option)}<Select.Item value={option} label={option}>{option}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
						<Field.Field><Field.Label for="crm-activity-contact">{text.externalContact}</Field.Label><CRMContactSelect id="crm-activity-contact" bind:value={contactID} contacts={organizationContacts} {text} /></Field.Field>
						<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label for="crm-activity-owner">{text.internalOwner}</Field.Label><CRMOwnerSelect id="crm-activity-owner" bind:value={taskOwnerID} {people} {groups} {text} /></Field.Field><Field.Field><Field.Label>{text.activityStatus}</Field.Label><Select.Root type="single" bind:value={activityStatus}><Select.Trigger class="w-full">{crmLabel(text.taskStatuses, activityStatus)}</Select.Trigger><Select.Content>{#each centralTaskStatusOptions as option (option)}<Select.Item value={option} label={crmLabel(text.taskStatuses, option)}>{crmLabel(text.taskStatuses, option)}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field></div>
						<Field.Field orientation="horizontal"><Checkbox id="crm-activity-calendar" bind:checked={isEvent} /><Field.Content><Field.Label for="crm-activity-calendar">{text.registerCalendar}</Field.Label><Field.Description>{text.registerCalendarDescription}</Field.Description></Field.Content></Field.Field>
						{#if isEvent}<Field.Field orientation="horizontal"><Checkbox id="crm-activity-all-day" bind:checked={isWholeDay} /><Field.Content><Field.Label for="crm-activity-all-day">{text.allDay}</Field.Label></Field.Content></Field.Field><div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label for="crm-activity-start">{text.startTime}</Field.Label><Input id="crm-activity-start" type="datetime-local" bind:value={startsAt} /></Field.Field><Field.Field><Field.Label for="crm-activity-end">{text.endTime}</Field.Label><Input id="crm-activity-end" type="datetime-local" bind:value={endsAt} /></Field.Field></div><Field.Field><Field.Label for="crm-activity-location">{text.location}</Field.Label><Input id="crm-activity-location" bind:value={location} /></Field.Field>{/if}
					<Field.Field><Field.Label for="crm-activity-summary">{text.details}</Field.Label><Textarea id="crm-activity-summary" rows={8} bind:value={summary} /></Field.Field>
				{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
			</Field.Group></div>
				<Sheet.Footer class="flex-row justify-end border-t bg-background px-4 py-3"><Button type="button" variant="outline" onclick={() => (open = false)} disabled={isSaving}>{text.cancel}</Button><Button type="submit" disabled={isSaving}>{isSaving ? text.creating : text.save}</Button></Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>
