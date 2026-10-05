<script lang="ts">
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import { TagsInput } from '$lib/components/ui/tags-input';
	import { Textarea } from '$lib/components/ui/textarea';
	import type { OrgGroup, UserRecord } from '$lib/organization/types';
	import CRMOwnerSelect from './crm-owner-select.svelte';
	import {
		type CRMOrganizationStatus,
		type CRMOrganizationType,
		type CRMImportance
	} from './crm-types';
	import type { CRMDefinition } from './crm-api-types';
	import { crmDefinitionLabel, crmLabel } from './crm-labels';
	import type { CRMText } from './text';

	type Props = {
		name: string;
		organizationTypes: CRMOrganizationType[];
		status: CRMOrganizationStatus;
		importance: CRMImportance;
		ownerPersonID: string;
		address: string;
		tags: string[];
		description: string;
		addExternalContact: boolean;
		contactName: string;
		contactTitle: string;
		contactEmail: string;
		contactPhone: string;
		contactNote: string;
		createdOrganizationID: string;
		people: UserRecord[];
		groups: OrgGroup[];
		text: CRMText;
		organizationTypeOptions: CRMOrganizationType[];
		organizationTypeDefinitions: CRMDefinition[];
	};

	let {
		name = $bindable(''),
		organizationTypes = $bindable([]),
		status = $bindable('prospect'),
		importance = $bindable('medium'),
		ownerPersonID = $bindable(''),
		address = $bindable(''),
		tags = $bindable([]),
		description = $bindable(''),
		addExternalContact = $bindable(false),
		contactName = $bindable(''),
		contactTitle = $bindable(''),
		contactEmail = $bindable(''),
		contactPhone = $bindable(''),
		contactNote = $bindable(''),
		createdOrganizationID,
		people,
		groups,
		text,
		organizationTypeOptions, organizationTypeDefinitions
	}: Props = $props();
	const organizationStatuses: CRMOrganizationStatus[] = ['prospect', 'active', 'paused'];
	const importanceOptions: CRMImportance[] = ['high', 'medium', 'low'];
	let selectedOwner = $derived(people.find((person) => person.memberID === ownerPersonID));

	function setOrganizationType(organizationType: CRMOrganizationType, checked: boolean): void {
		if (checked) {
			if (!organizationTypes.includes(organizationType)) organizationTypes = [...organizationTypes, organizationType];
			return;
		}
		organizationTypes = organizationTypes.filter((type) => type !== organizationType);
	}
</script>

{#if createdOrganizationID}
	<p class="rounded-md border bg-muted/40 px-3 py-2 text-sm text-muted-foreground">{text.relationshipAlreadyCreated}</p>
{/if}

<fieldset disabled={createdOrganizationID !== ''} class="grid gap-6 disabled:opacity-70">
	<Field.Field><Field.Label for="crm-record-name">{text.name}</Field.Label><Input id="crm-record-name" bind:value={name} required /></Field.Field>
	<Field.Field>
		<Field.Label>{text.type}</Field.Label>
		<div class="grid gap-3 rounded-md border p-3 sm:grid-cols-2">
			{#each organizationTypeOptions as organizationType (organizationType)}
				<label class="flex items-center gap-2 text-sm"><Checkbox checked={organizationTypes.includes(organizationType)} onCheckedChange={(checked) => setOrganizationType(organizationType, checked)} />{crmDefinitionLabel(organizationTypeDefinitions, text.organizationTypes, organizationType)}</label>
			{/each}
		</div>
	</Field.Field>
	<div class="grid gap-4 sm:grid-cols-2">
		<Field.Field><Field.Label>{text.status}</Field.Label><Select.Root type="single" value={status} onValueChange={(value) => (status = value as CRMOrganizationStatus)}><Select.Trigger class="w-full">{text.organizationStatuses[status]}</Select.Trigger><Select.Content><Select.Group>{#each organizationStatuses as option (option)}<Select.Item value={option} label={text.organizationStatuses[option]}>{text.organizationStatuses[option]}</Select.Item>{/each}</Select.Group></Select.Content></Select.Root></Field.Field>
		<Field.Field><Field.Label>{text.importance}</Field.Label><Select.Root type="single" value={importance} onValueChange={(value) => (importance = value as CRMImportance)}><Select.Trigger class="w-full">{text.importanceLabels[importance]}</Select.Trigger><Select.Content><Select.Group>{#each importanceOptions as option (option)}<Select.Item value={option} label={text.importanceLabels[option]}>{text.importanceLabels[option]}</Select.Item>{/each}</Select.Group></Select.Content></Select.Root></Field.Field>
	</div>
	<Field.Field><Field.Label for="crm-record-owner">{text.internalOwner}</Field.Label><CRMOwnerSelect id="crm-record-owner" bind:value={ownerPersonID} {people} {groups} {text} disabled={createdOrganizationID !== ''} /></Field.Field>
	<div class="grid gap-4 sm:grid-cols-2">
		<Field.Field><Field.Label for="crm-record-owner-email">{text.ownerEmail}</Field.Label><Input id="crm-record-owner-email" value={selectedOwner?.email ?? ''} disabled /></Field.Field>
	</div>
	<Field.Field><Field.Label for="crm-record-address">{text.address}</Field.Label><Input id="crm-record-address" bind:value={address} /></Field.Field>
	<Field.Field><Field.Label for="crm-record-tags">{text.tags}</Field.Label><TagsInput id="crm-record-tags" bind:value={tags} /></Field.Field>
	<Field.Field><Field.Label for="crm-record-details">{text.details}</Field.Label><Textarea id="crm-record-details" rows={6} bind:value={description} /></Field.Field>
</fieldset>

<Field.Field orientation="horizontal">
	<Checkbox id="crm-record-add-contact" bind:checked={addExternalContact} disabled={createdOrganizationID !== ''} />
	<Field.Content><Field.Label for="crm-record-add-contact">{text.addExternalContact}</Field.Label><Field.Description>{text.addExternalContactDescription}</Field.Description></Field.Content>
</Field.Field>

{#if addExternalContact}
	<div class="grid gap-6 rounded-md border p-3">
		<div class="grid gap-4 sm:grid-cols-2">
			<Field.Field><Field.Label for="crm-record-external-name">{text.contactName}</Field.Label><Input id="crm-record-external-name" bind:value={contactName} required /></Field.Field>
			<Field.Field><Field.Label for="crm-record-external-title">{text.contactTitle}</Field.Label><Input id="crm-record-external-title" bind:value={contactTitle} /></Field.Field>
		</div>
		<div class="grid gap-4 sm:grid-cols-2">
			<Field.Field><Field.Label for="crm-record-external-email">{text.email}</Field.Label><Input id="crm-record-external-email" type="email" bind:value={contactEmail} /></Field.Field>
			<Field.Field><Field.Label for="crm-record-external-phone">{text.phone}</Field.Label><Input id="crm-record-external-phone" type="tel" bind:value={contactPhone} /></Field.Field>
		</div>
		<Field.Field><Field.Label for="crm-record-external-note">{text.details}</Field.Label><Textarea id="crm-record-external-note" rows={4} bind:value={contactNote} /></Field.Field>
	</div>
{/if}
