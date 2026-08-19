<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import { TagsInput } from '$lib/components/ui/tags-input';
	import { Textarea } from '$lib/components/ui/textarea';
	import type { OrgGroup, UserRecord } from '$lib/organization/types';
	import CRMOwnerSelect from './crm-owner-select.svelte';
	import CRMRelationshipContactManager from './crm-relationship-contact-manager.svelte';
	import { type CRMOrganization, type CRMOrganizationStatus, type CRMOrganizationType, type CRMContact, type CRMImportance } from './crm-types';
	import { crmLabel } from './crm-labels';
	import type { CRMText } from './text';

	type Props = {
		open: boolean;
		organization: CRMOrganization | undefined;
		people: UserRecord[];
		groups: OrgGroup[];
		contacts: CRMContact[];
		organizationTypeOptions: CRMOrganizationType[];
		text: CRMText;
		onSave: (organization: CRMOrganization) => Promise<void>;
		onArchive: (organizationID: string) => Promise<void>;
		onEditContact: (contactID: string) => void;
		onCreateContact: (organizationID: string) => void;
	};

	let { open = $bindable(false), organization, people, groups, contacts, organizationTypeOptions, text, onSave, onArchive, onEditContact, onCreateContact }: Props = $props();
	const organizationStatuses: CRMOrganizationStatus[] = ['prospect', 'active', 'paused'];
	const importanceOptions: CRMImportance[] = ['high', 'medium', 'low'];
	let name = $state('');
	let types = $state<CRMOrganizationType[]>([]);
	let status = $state<CRMOrganizationStatus>('prospect');
	let importance = $state<CRMImportance>('medium');
	let ownerName = $state('');
	let ownerPersonID = $state('');
	let ownerEmail = $state('');
	let team = $state('');
	let address = $state('');
	let tags = $state<string[]>([]);
	let description = $state('');
	let lastContactDate = $state('');
	let nextActionDate = $state('');
	let isSaving = $state(false);
	let errorMessage = $state('');
	let selectedOwner = $derived(people.find((person) => person.userID === ownerPersonID));
	let selectedTeam = $derived(groups.find((group) => group.id === selectedOwner?.groupID)?.name ?? '');

	function resetForm(selectedOrganization: CRMOrganization): void {
		const owner = people.find((person) => person.userID === selectedOrganization.ownerPersonID);
		const ownerTeam = groups.find((group) => group.id === owner?.groupID)?.name;
		name = selectedOrganization.name;
		types = [...selectedOrganization.types];
		status = selectedOrganization.status;
		importance = selectedOrganization.importance;
		ownerName = selectedOrganization.ownerName;
		ownerPersonID = selectedOrganization.ownerPersonID ?? '';
		ownerEmail = owner?.email ?? selectedOrganization.ownerEmail;
		team = ownerTeam || selectedOrganization.team;
		address = selectedOrganization.address ?? '';
		tags = [...selectedOrganization.tags];
		description = selectedOrganization.description;
		lastContactDate = selectedOrganization.lastContactDate;
		nextActionDate = selectedOrganization.nextActionDate;
		isSaving = false;
		errorMessage = '';
	}

	function setType(organizationType: CRMOrganizationType, checked: boolean): void {
		if (checked) {
			if (!types.includes(organizationType)) types = [...types, organizationType];
			return;
		}
		types = types.filter((type) => type !== organizationType);
	}

	async function save(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (!organization || name.trim() === '') return;
		isSaving = true;
		errorMessage = '';
		try {
			await onSave({
				...organization,
				name: name.trim(),
				types,
				status,
				importance,
				ownerPersonID,
				ownerCircleID: selectedOwner?.groupID,
				ownerName: selectedOwner?.name || selectedOwner?.email || ownerName.trim(),
				ownerEmail: selectedOwner?.email ?? ownerEmail,
				team: selectedTeam || team,
				address: address.trim() || undefined,
				tags,
				description: description.trim()
			});
			open = false;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.requiredField;
		} finally {
			isSaving = false;
		}
	}

	function archive(): void {
		if (!organization) return;
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
					await onArchive(organization.id);
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

	function editContact(contactID: string): void {
		open = false;
		onEditContact(contactID);
	}

	function createContact(organizationID: string): void {
		open = false;
		onCreateContact(organizationID);
	}

	$effect(() => {
		if (open && organization) resetForm(organization);
	});
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" closeLabel={text.cancel} class="w-full gap-0 overflow-hidden p-0 sm:max-w-xl">
		<form onsubmit={save} class="flex min-h-0 flex-1 flex-col">
			<Sheet.Header class="border-b px-4 py-4 pr-12">
				<Sheet.Title>{text.editRelationship}</Sheet.Title>
				<Sheet.Description>{text.editRelationshipDescription}</Sheet.Description>
			</Sheet.Header>
			<div class="min-h-0 flex-1 overflow-y-auto px-4 py-5">
				<Field.Group>
					<Field.Field><Field.Label for="crm-edit-organization-name">{text.organizationName}</Field.Label><Input id="crm-edit-organization-name" bind:value={name} required /></Field.Field>
					<Field.Field>
						<Field.Label>{text.type}</Field.Label>
						<div class="grid gap-3 rounded-md border p-3 sm:grid-cols-2">
							{#each organizationTypeOptions as organizationType (organizationType)}
								<label class="flex items-center gap-2 text-sm"><Checkbox checked={types.includes(organizationType)} onCheckedChange={(checked) => setType(organizationType, checked)} />{crmLabel(text.organizationTypes, organizationType)}</label>
							{/each}
						</div>
					</Field.Field>
					<div class="grid gap-4 sm:grid-cols-2">
						<Field.Field><Field.Label for="crm-edit-organization-status">{text.status}</Field.Label><Select.Root type="single" value={status} onValueChange={(value) => (status = value as CRMOrganizationStatus)}><Select.Trigger id="crm-edit-organization-status" class="w-full">{text.organizationStatuses[status]}</Select.Trigger><Select.Content>{#each organizationStatuses as option (option)}<Select.Item value={option} label={text.organizationStatuses[option]}>{text.organizationStatuses[option]}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
						<Field.Field><Field.Label for="crm-edit-organization-importance">{text.importance}</Field.Label><Select.Root type="single" value={importance} onValueChange={(value) => (importance = value as CRMImportance)}><Select.Trigger id="crm-edit-organization-importance" class="w-full">{text.importanceLabels[importance]}</Select.Trigger><Select.Content>{#each importanceOptions as option (option)}<Select.Item value={option} label={text.importanceLabels[option]}>{text.importanceLabels[option]}</Select.Item>{/each}</Select.Content></Select.Root></Field.Field>
					</div>
					<div class="grid gap-4 sm:grid-cols-2">
						<Field.Field><Field.Label for="crm-edit-organization-owner">{text.internalOwner}</Field.Label><CRMOwnerSelect id="crm-edit-organization-owner" bind:value={ownerPersonID} {people} {groups} {text} /></Field.Field>
						<Field.Field><Field.Label for="crm-edit-organization-team">{text.team}</Field.Label><Input id="crm-edit-organization-team" value={selectedTeam || team} disabled /></Field.Field>
					</div>
					<Field.Field><Field.Label for="crm-edit-organization-email">{text.ownerEmail}</Field.Label><Input id="crm-edit-organization-email" type="email" value={selectedOwner?.email ?? ownerEmail} disabled /></Field.Field>
					{#if organization}<CRMRelationshipContactManager organizationID={organization.id} {contacts} {text} onEdit={editContact} onCreate={createContact} />{/if}
					<Field.Field><Field.Label for="crm-edit-organization-address">{text.address}</Field.Label><Input id="crm-edit-organization-address" bind:value={address} placeholder={text.addressPlaceholder} /></Field.Field>
					<Field.Field><Field.Label for="crm-edit-organization-tags">{text.tags}</Field.Label><TagsInput id="crm-edit-organization-tags" bind:value={tags} placeholder={tags.length === 0 ? text.tagsPlaceholder : undefined} /></Field.Field>
					<div class="grid gap-4 sm:grid-cols-2">
						<Field.Field><Field.Label for="crm-edit-organization-last-contact">{text.lastContact}</Field.Label><Input id="crm-edit-organization-last-contact" type="date" bind:value={lastContactDate} disabled /></Field.Field>
						<Field.Field><Field.Label for="crm-edit-organization-next-action">{text.nextContactDate}</Field.Label><Input id="crm-edit-organization-next-action" type="date" bind:value={nextActionDate} disabled /></Field.Field>
					</div>
					<Field.Field><Field.Label for="crm-edit-organization-details">{text.details}</Field.Label><Textarea id="crm-edit-organization-details" rows={6} bind:value={description} /></Field.Field>
					{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
				</Field.Group>
			</div>
			<Sheet.Footer class="flex-row justify-between border-t bg-background px-4 py-3">
				<Button type="button" variant="destructive" onclick={archive} disabled={isSaving}>{text.archive}</Button>
				<div class="flex gap-2"><Button type="button" variant="outline" onclick={() => (open = false)} disabled={isSaving}>{text.cancel}</Button><Button type="submit" disabled={isSaving}>{isSaving ? text.creating : text.save}</Button></div>
			</Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>
