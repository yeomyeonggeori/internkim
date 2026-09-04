<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Textarea } from '$lib/components/ui/textarea';
	import type { OrgGroup, UserRecord } from '$lib/organization/types';
	import { untrack } from 'svelte';
	import { activityReferenceAfterOrganizationChange, activityReferenceAfterOpportunityChange } from './crm-activity-reference';
	import { crmActivityCalendarDefaults, localDateTimeValue } from './crm-activity-calendar-defaults';
	import { hasCRMContactMethod } from './crm-contact-validation';
	import { currentCRMDate } from './crm-date';
	import type { CRMDefinition } from './crm-api-types';
	import { crmLabel } from './crm-labels';
	import { parseAmountInput } from './crm-money';
	import CRMConversionPreview from './crm-conversion-preview.svelte';
	import CRMMoneyField from './crm-money-field.svelte';
	import CRMOwnerSelect from './crm-owner-select.svelte';
	import CRMContactSelect from './crm-contact-select.svelte';
	import { CRMRelationshipContactCreateError } from './crm-relationship-create';
	import CRMRelationshipCreateForm from './crm-relationship-create-form.svelte';
	import type {
		CRMOrganization,
		CRMOrganizationStatus,
		CRMOrganizationType,
		CRMActivityKind,
		CRMContact,
		CRMCurrency,
		CRMCreateDraft,
		CRMImportance,
		CRMOpportunity,
		CRMOpportunityStage,
		CRMPipeline,
		CRMPipelineStage,
		CRMProgressKind,
		CRMRecordKind
	} from './crm-types';
	import { findOrganizationByID, opportunityStageLabel } from './crm-view-model';
	import { minorAmountOf, type CurrencyCatalogue } from '$lib/currency/currency-catalogue';
	import type { CRMText } from './text';

	type Props = {
		open: boolean;
		initialKind: CRMRecordKind;
		initialOrganizationID?: string;
		organizations: CRMOrganization[];
		contacts: CRMContact[];
		opportunities: CRMOpportunity[];
		pipelines: CRMPipeline[];
		stages: CRMPipelineStage[];
		businessOptions: string[];
		organizationTypeOptions: CRMOrganizationType[];
		organizationTypeDefinitions: CRMDefinition[];
		activityKindOptions: CRMActivityKind[];
		defaultOwnerPersonID: string;
		people: UserRecord[];
		groups: OrgGroup[];
		text: CRMText;
		currencyCatalogue: CurrencyCatalogue;
		companyBaseCurrency: string;
		onCreate: (draft: CRMCreateDraft) => Promise<void>;
	};

	let { open = $bindable(false), initialKind, initialOrganizationID = '', organizations, contacts, opportunities, pipelines, stages, businessOptions, organizationTypeOptions, organizationTypeDefinitions, activityKindOptions, defaultOwnerPersonID, people, groups, text, currencyCatalogue, companyBaseCurrency, onCreate }: Props = $props();
	const importanceOptions: CRMImportance[] = ['high', 'medium', 'low'];
	let activityKinds = $derived(activityKindOptions.filter((kind) => kind !== 'stage_change'));
	const noOrganizationValue = '__no_organization__';
	const noContactValue = '__no_contact__';
	let kind = $state<CRMRecordKind>('relationship');
	let name = $state('');
	let organizationID = $state('');
	let organizationTypes = $state<CRMOrganizationType[]>([]);
	let status = $state<CRMOrganizationStatus>('prospect');
	let importance = $state<CRMImportance>('medium');
	let ownerPersonID = $state('');
	let address = $state('');
	let tags = $state<string[]>([]);
	let description = $state('');
	let contactTitle = $state('');
	let email = $state('');
	let phone = $state('');
	let addExternalContact = $state(false);
	let externalContactName = $state('');
	let externalContactTitle = $state('');
	let externalContactEmail = $state('');
	let externalContactPhone = $state('');
	let externalContactNote = $state('');
	let createdOrganizationID = $state('');
	let progressContactID = $state('');
	let business = $state('general');
	let pipeline = $state<CRMProgressKind>('sales');
	let stage = $state(defaultStage());
	let lostReason = $state('');
	let amount = $state('');
	let currency = $state<CRMCurrency>('');
	let typedAmountMinor = $derived.by(() => {
		const typedAmount = parseAmountInput(amount);
		return typedAmount === undefined ? null : minorAmountOf(typedAmount, currency, currencyCatalogue);
	});
	let targetDate = $state('');
	let opportunityID = $state('');
	let activityContactID = $state('');
	let activityKind = $state<Exclude<CRMActivityKind, 'stage_change'>>('note');
	let occurredAt = $state('');
	let taskOwnerID = $state('');
	let taskStatus = $state('todo');
	let registerCalendar = $state(false);
	let isAllDay = $state(false);
	let calendarStart = $state('');
	let calendarEnd = $state('');
	let calendarLocation = $state('');
	let errorMessage = $state('');
	let isSaving = $state(false);
	let sortedStages = $derived([...stages].sort((left, right) => left.position - right.position));
	let relatedOpportunities = $derived(opportunities.filter((opportunity) => opportunity.organizationID === organizationID));
	let organizationContacts = $derived(contacts.filter((contact) => contact.organizationID === organizationID));
	let stageOutcome = $derived(sortedStages.find((candidate) => candidate.stage === stage)?.outcome ?? 'open');

	function defaultStage(): CRMOpportunityStage {
		return stages.find((candidate) => candidate.outcome === 'open')?.stage ?? 'waiting';
	}

	function resetForm(): void {
		kind = initialKind;
		name = '';
		organizationID = initialOrganizationID || organizations[0]?.id || '';
		organizationTypes = [];
		status = 'prospect';
		importance = 'medium';
		ownerPersonID = defaultOwnerPersonID;
		address = '';
		tags = [];
		description = '';
		contactTitle = '';
		email = '';
		phone = '';
		addExternalContact = false;
		externalContactName = '';
		externalContactTitle = '';
		externalContactEmail = '';
		externalContactPhone = '';
		externalContactNote = '';
		createdOrganizationID = '';
		progressContactID = '';
		business = businessOptions[0] ?? 'general';
		pipeline = pipelines[0]?.pipeline ?? 'sales';
		stage = defaultStage();
		lostReason = '';
		amount = '';
		currency = companyBaseCurrency;
		targetDate = currentCRMDate();
		opportunityID = '';
		activityContactID = '';
		activityKind = activityKinds[0] ?? 'note';
		occurredAt = localDateTimeValue(new Date());
		taskOwnerID = defaultOwnerPersonID;
		taskStatus = 'todo';
		registerCalendar = false;
		isAllDay = false;
		({ calendarStart, calendarEnd } = crmActivityCalendarDefaults(occurredAt));
		calendarLocation = '';
		errorMessage = '';
		isSaving = false;
	}

	function selectPipeline(value: string): void {
		pipeline = value as CRMProgressKind;
	}

	function selectOrganization(value: string): void {
		const nextOrganizationID = value === noOrganizationValue ? '' : value;
		if (kind === 'contact') {
			organizationID = nextOrganizationID;
			return;
		}
		if (kind === 'progress') {
			organizationID = nextOrganizationID;
			if (!contacts.some((contact) => contact.id === progressContactID && contact.organizationID === organizationID)) progressContactID = '';
			return;
		}
		const reference = activityReferenceAfterOrganizationChange(opportunities, nextOrganizationID, opportunityID, business);
		organizationID = reference.organizationID;
		opportunityID = reference.opportunityID;
		business = reference.business;
		if (!contacts.some((contact) => contact.id === activityContactID && contact.organizationID === organizationID)) activityContactID = '';
	}

	function selectOpportunity(value: string): void {
		const reference = activityReferenceAfterOpportunityChange(opportunities, value, organizationID, business);
		organizationID = reference.organizationID;
		opportunityID = reference.opportunityID;
		business = reference.business;
		if (!contacts.some((contact) => contact.id === activityContactID && contact.organizationID === organizationID)) activityContactID = '';
	}

	async function submit(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (!name.trim()) {
			errorMessage = text.requiredField;
			return;
		}
		if (kind === 'contact' && !hasCRMContactMethod(email, phone)) {
			errorMessage = text.contactMethodRequired;
			return;
		}
		if (kind === 'relationship' && !ownerPersonID) {
			errorMessage = text.selectInternalOwner;
			return;
		}
		if (kind === 'relationship' && addExternalContact && (!externalContactName.trim() || !hasCRMContactMethod(externalContactEmail, externalContactPhone))) {
			errorMessage = !externalContactName.trim() ? text.requiredField : text.contactMethodRequired;
			return;
		}
		if (kind === 'progress' && !organizationID && !progressContactID) {
			errorMessage = text.opportunityCustomerRequired;
			return;
		}
		if (kind === 'progress' && !ownerPersonID) {
			errorMessage = text.selectInternalOwner;
			return;
		}
		if (kind === 'progress' && stageOutcome === 'lost' && !lostReason) {
			errorMessage = text.lostReasonRequired;
			return;
		}
		if (kind === 'activity' && registerCalendar && calendarStart && calendarEnd && calendarEnd < calendarStart) {
			errorMessage = text.endTimeAfterStart;
			return;
		}
		isSaving = true;
		errorMessage = '';
		try {
			if (kind === 'relationship') {
				await onCreate({
					kind,
					name: name.trim(),
					types: organizationTypes,
					status,
					importance,
					ownerPersonID,
					address: address.trim(),
					tags,
					description: description.trim(),
					createdOrganizationID: createdOrganizationID || undefined,
					contact: addExternalContact ? {
						name: externalContactName.trim(),
						title: externalContactTitle.trim(),
						email: externalContactEmail.trim(),
						phone: externalContactPhone.trim(),
						note: externalContactNote.trim()
					} : undefined
				});
			} else if (kind === 'contact') {
				await onCreate({ kind, organizationID, name: name.trim(), title: contactTitle.trim(), email: email.trim(), phone: phone.trim(), note: description.trim() });
			} else if (kind === 'progress') {
				await onCreate({ kind, organizationID, contacts: progressContactID ? [{ contactID: progressContactID }] : [], business, name: name.trim(), progressKind: pipeline, stage, lostReason: stageOutcome === 'lost' ? lostReason : '', ownerPersonID, amount: parseAmountInput(amount), currency, importance, targetDate, description: description.trim(), calendar: { isRequested: false, isAllDay: true, startTime: '', endTime: '', location: '' } });
			} else {
				await onCreate({ kind, organizationID, contactID: activityContactID || undefined, opportunityID: opportunityID || undefined, business, activityKind, title: name.trim(), occurredAt, summary: description.trim(), taskOwnerID, taskStatus, calendar: { isRequested: registerCalendar, isAllDay, startTime: calendarStart, endTime: calendarEnd, location: calendarLocation.trim() } });
			}
			open = false;
		} catch (error) {
			if (error instanceof CRMRelationshipContactCreateError) createdOrganizationID = error.organizationID;
			errorMessage = error instanceof Error ? error.message : text.requiredField;
		} finally {
			isSaving = false;
		}
	}

	$effect(() => {
		if (open) untrack(resetForm);
	});
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" closeLabel={text.cancel} class="w-full gap-0 overflow-hidden p-0">
		<form onsubmit={submit} class="flex min-h-0 flex-1 flex-col">
			<Sheet.Header class="border-b px-4 py-4 pr-12"><Sheet.Title>{text.createTitle}</Sheet.Title><Sheet.Description>{text.createDescription}</Sheet.Description></Sheet.Header>
			<div class="min-h-0 flex-1 overflow-y-auto px-4 py-5">
				<Field.Group>
					<Field.Field><Field.Label for="crm-record-kind">{text.recordKind}</Field.Label><Select.Root type="single" value={kind} onValueChange={(value) => (kind = value as CRMRecordKind)}><Select.Trigger id="crm-record-kind" class="w-full">{kind === 'relationship' ? text.newRelationship : kind === 'contact' ? text.newContact : kind === 'progress' ? text.newOpportunity : text.logActivity}</Select.Trigger><Select.Content><Select.Item value="relationship" label={text.newRelationship}>{text.newRelationship}</Select.Item><Select.Item value="contact" label={text.newContact}>{text.newContact}</Select.Item><Select.Item value="progress" label={text.newOpportunity}>{text.newOpportunity}</Select.Item><Select.Item value="activity" label={text.logActivity}>{text.logActivity}</Select.Item></Select.Content></Select.Root></Field.Field>
					{#if kind !== 'relationship'}
							<Field.Field><Field.Label for="crm-record-organization">{text.organizationName}</Field.Label><Select.Root type="single" value={organizationID || noOrganizationValue} onValueChange={selectOrganization}><Select.Trigger id="crm-record-organization" class="w-full">{organizationID ? findOrganizationByID(organizations, organizationID)?.name ?? text.selectRelationship : text.none}</Select.Trigger><Select.Content>{#if kind === 'contact' || kind === 'progress'}<Select.Item value={noOrganizationValue} label={text.none}>{text.none}</Select.Item>{/if}{#each organizations as organization (organization.id)}<Select.Item value={organization.id} label={organization.name}>{organization.name}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
					{/if}
					{#if kind === 'relationship'}
						<CRMRelationshipCreateForm
							bind:name bind:organizationTypes bind:status bind:importance bind:ownerPersonID bind:address bind:tags bind:description
							bind:addExternalContact bind:contactName={externalContactName} bind:contactTitle={externalContactTitle}
							bind:contactEmail={externalContactEmail} bind:contactPhone={externalContactPhone}
							bind:contactNote={externalContactNote}
							{createdOrganizationID} {people} {groups} {text}
							{organizationTypeOptions}
							{organizationTypeDefinitions}
						/>
					{:else if kind === 'contact'}
						<Field.Field><Field.Label for="crm-record-name">{text.contactName}</Field.Label><Input id="crm-record-name" bind:value={name} required /></Field.Field>
						<Field.Field><Field.Label for="crm-record-contact-title">{text.contactTitle}</Field.Label><Input id="crm-record-contact-title" bind:value={contactTitle} /></Field.Field>
						<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label for="crm-record-email">{text.email}</Field.Label><Input id="crm-record-email" type="email" bind:value={email} /></Field.Field><Field.Field><Field.Label for="crm-record-phone">{text.phone}</Field.Label><Input id="crm-record-phone" type="tel" bind:value={phone} /></Field.Field></div>
					{:else if kind === 'progress'}
						<Field.Field><Field.Label for="crm-record-name">{text.name}</Field.Label><Input id="crm-record-name" bind:value={name} required /></Field.Field>
						<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label>{text.progressKind}</Field.Label><Select.Root type="single" value={pipeline} onValueChange={selectPipeline}><Select.Trigger class="w-full">{pipelines.find((candidate) => candidate.pipeline === pipeline)?.label ?? pipeline}</Select.Trigger><Select.Content>{#each pipelines as option (option.pipeline)}<Select.Item value={option.pipeline} label={option.label}>{option.label}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field><Field.Field><Field.Label for="crm-record-stage">{text.stage}</Field.Label><Select.Root type="single" bind:value={stage}><Select.Trigger id="crm-record-stage" class="w-full">{opportunityStageLabel(sortedStages, stage, text)}</Select.Trigger><Select.Content>{#each sortedStages as option (option.stage)}<Select.Item value={option.stage} label={opportunityStageLabel(sortedStages, option.stage, text)}>{opportunityStageLabel(sortedStages, option.stage, text)}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field></div>
						{#if stageOutcome === 'lost'}<Field.Field><Field.Label for="crm-record-lost-reason">{text.lostReasonPrompt}</Field.Label><Input id="crm-record-lost-reason" bind:value={lostReason} required /></Field.Field>{/if}
						<Field.Field><Field.Label>{text.business}</Field.Label><Select.Root type="single" bind:value={business}><Select.Trigger class="w-full">{business}</Select.Trigger><Select.Content>{#each businessOptions as option (option)}<Select.Item value={option} label={option}>{option}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
						<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label>{text.importance}</Field.Label><Select.Root type="single" value={importance} onValueChange={(value) => (importance = value as CRMImportance)}><Select.Trigger class="w-full">{text.importanceLabels[importance]}</Select.Trigger><Select.Content>{#each importanceOptions as option (option)}<Select.Item value={option} label={text.importanceLabels[option]}>{text.importanceLabels[option]}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field><Field.Field><Field.Label for="crm-record-target">{text.targetDate}</Field.Label><Input id="crm-record-target" type="date" bind:value={targetDate} /></Field.Field></div>
							<CRMMoneyField {currencyCatalogue} id="crm-record-amount" label={text.amount} currencyLabel={text.currency} bind:value={amount} bind:currency />
							<CRMConversionPreview
								amountMinor={typedAmountMinor}
								{currency}
								baseCurrency={companyBaseCurrency}
								settledAmountMinor={null}
								settledCurrency=""
								{currencyCatalogue}
								{text}
							/>
							<Field.Field><Field.Label for="crm-record-progress-contact">{text.externalContact}</Field.Label><CRMContactSelect id="crm-record-progress-contact" bind:value={progressContactID} contacts={organizationContacts} {text} /></Field.Field>
						<Field.Field><Field.Label for="crm-record-progress-owner">{text.internalOwner}</Field.Label><CRMOwnerSelect id="crm-record-progress-owner" bind:value={ownerPersonID} {people} {groups} {text} /></Field.Field>
					{:else}
						<Field.Field><Field.Label for="crm-record-name">{text.activityTitle}</Field.Label><Input id="crm-record-name" bind:value={name} required /></Field.Field>
							<Field.Field><Field.Label for="crm-record-opportunity">{text.relatedProgress}</Field.Label><Select.Root type="single" value={opportunityID} onValueChange={selectOpportunity}><Select.Trigger id="crm-record-opportunity" class="w-full">{relatedOpportunities.find((opportunity) => opportunity.id === opportunityID)?.name ?? text.noRelatedProgress}</Select.Trigger><Select.Content><Select.Item value="" label={text.noRelatedProgress}>{text.noRelatedProgress}</Select.Item>{#each relatedOpportunities as opportunity (opportunity.id)}<Select.Item value={opportunity.id} label={opportunity.name}>{opportunity.name}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
						<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label>{text.activityKind}</Field.Label><Select.Root type="single" value={activityKind} onValueChange={(value) => (activityKind = value as Exclude<CRMActivityKind, 'stage_change'>)}><Select.Trigger class="w-full">{crmLabel(text.activityKinds, activityKind)}</Select.Trigger><Select.Content>{#each activityKinds as option (option)}<Select.Item value={option} label={crmLabel(text.activityKinds, option)}>{crmLabel(text.activityKinds, option)}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field><Field.Field><Field.Label for="crm-record-occurred">{text.occurredAt}</Field.Label><Input id="crm-record-occurred" type="datetime-local" bind:value={occurredAt} /></Field.Field></div>
							<Field.Field><Field.Label for="crm-record-activity-business">{text.business}</Field.Label><Select.Root type="single" bind:value={business} disabled={opportunityID !== ''}><Select.Trigger id="crm-record-activity-business" class="w-full">{business}</Select.Trigger><Select.Content>{#each businessOptions as option (option)}<Select.Item value={option} label={option}>{option}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
							<Field.Field><Field.Label for="crm-record-activity-contact">{text.externalContact}</Field.Label><CRMContactSelect id="crm-record-activity-contact" bind:value={activityContactID} contacts={organizationContacts} {text} /></Field.Field>
							<div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label for="crm-record-task-owner">{text.internalOwner}</Field.Label><CRMOwnerSelect id="crm-record-task-owner" bind:value={taskOwnerID} {people} {groups} {text} /></Field.Field><Field.Field><Field.Label>{text.activityStatus}</Field.Label><Select.Root type="single" bind:value={taskStatus}><Select.Trigger class="w-full">{crmLabel(text.nextActionStatuses, taskStatus)}</Select.Trigger><Select.Content>{#each ['todo', 'in_progress', 'done', 'paused', 'cancelled'] as option (option)}<Select.Item value={option} label={crmLabel(text.nextActionStatuses, option)}>{crmLabel(text.nextActionStatuses, option)}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field></div>
							<Field.Field orientation="horizontal"><Checkbox id="crm-record-calendar" bind:checked={registerCalendar} /><Field.Content><Field.Label for="crm-record-calendar">{text.registerCalendar}</Field.Label><Field.Description>{text.registerCalendarDescription}</Field.Description></Field.Content></Field.Field>
							{#if registerCalendar}<Field.Field orientation="horizontal"><Checkbox id="crm-record-all-day" bind:checked={isAllDay} /><Field.Content><Field.Label for="crm-record-all-day">{text.allDay}</Field.Label></Field.Content></Field.Field><div class="grid gap-4 sm:grid-cols-2"><Field.Field><Field.Label for="crm-record-calendar-start">{text.startTime}</Field.Label><Input id="crm-record-calendar-start" type="datetime-local" bind:value={calendarStart} /></Field.Field><Field.Field><Field.Label for="crm-record-calendar-end">{text.endTime}</Field.Label><Input id="crm-record-calendar-end" type="datetime-local" bind:value={calendarEnd} /></Field.Field></div><Field.Field><Field.Label for="crm-record-calendar-location">{text.location}</Field.Label><Input id="crm-record-calendar-location" bind:value={calendarLocation} /></Field.Field>{/if}
					{/if}
					{#if kind !== 'relationship'}<Field.Field><Field.Label for="crm-record-details">{text.details}</Field.Label><Textarea id="crm-record-details" rows={6} bind:value={description} /></Field.Field>{/if}
					{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
				</Field.Group>
			</div>
			<Sheet.Footer class="flex-row justify-end border-t bg-background px-4 py-3"><Button type="button" variant="outline" onclick={() => (open = false)} disabled={isSaving}>{text.cancel}</Button><Button type="submit" disabled={isSaving}>{isSaving ? text.creating : text.create}</Button></Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>
