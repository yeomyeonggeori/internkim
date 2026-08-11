<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import { Textarea } from '$lib/components/ui/textarea';
	import { hasCRMContactMethod } from './crm-contact-validation';
	import type { CRMAccount, CRMContact } from './crm-types';
	import { findAccountByID } from './crm-view-model';
	import type { CRMText } from './text';

	type Props = {
		open: boolean;
		contact: CRMContact | undefined;
		accounts: CRMAccount[];
		text: CRMText;
		onSave: (contact: CRMContact) => Promise<void>;
	};

	let { open = $bindable(false), contact, accounts, text, onSave }: Props = $props();
	const noAccountValue = '__no_account__';
	let accountID = $state('');
	let name = $state('');
	let title = $state('');
	let email = $state('');
	let phone = $state('');
	let isPrimary = $state(false);
	let note = $state('');
	let isSaving = $state(false);
	let errorMessage = $state('');

	function resetForm(selectedContact: CRMContact): void {
		accountID = selectedContact.accountID;
		name = selectedContact.name;
		title = selectedContact.title;
		email = selectedContact.email;
		phone = selectedContact.phone ?? '';
		isPrimary = selectedContact.isPrimary;
		note = selectedContact.note ?? '';
		isSaving = false;
		errorMessage = '';
	}

	async function save(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		if (!contact || name.trim() === '') return;
		if (!hasCRMContactMethod(email, phone)) {
			errorMessage = text.contactMethodRequired;
			return;
		}
		isSaving = true;
		errorMessage = '';
		try {
			await onSave({
				...contact,
				accountID,
				name: name.trim(),
				title: title.trim(),
				email: email.trim(),
				phone: phone.trim() || undefined,
				isPrimary,
				note: note.trim() || undefined
			});
			open = false;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.requiredField;
		} finally {
			isSaving = false;
		}
	}

	$effect(() => {
		if (open && contact) resetForm(contact);
	});
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" closeLabel={text.cancel} class="w-full gap-0 overflow-hidden p-0 sm:max-w-xl">
		<form onsubmit={save} class="flex min-h-0 flex-1 flex-col">
			<Sheet.Header class="border-b px-4 py-4 pr-12">
				<Sheet.Title>{text.editContact}</Sheet.Title>
				<Sheet.Description>{text.editContactDescription}</Sheet.Description>
			</Sheet.Header>
			<div class="min-h-0 flex-1 overflow-y-auto px-4 py-5">
				<Field.Group>
					<Field.Field>
						<Field.Label for="crm-edit-contact-account">{text.accountName}</Field.Label>
						<Select.Root type="single" value={accountID || noAccountValue} onValueChange={(value) => { accountID = value === noAccountValue ? '' : value; if (!accountID) isPrimary = false; }}>
							<Select.Trigger id="crm-edit-contact-account" class="w-full">{accountID ? findAccountByID(accounts, accountID)?.name ?? text.selectRelationship : text.none}</Select.Trigger>
							<Select.Content><Select.Item value={noAccountValue} label={text.none}>{text.none}</Select.Item>{#each accounts as account (account.id)}<Select.Item value={account.id} label={account.name}>{account.name}</Select.Item>{/each}</Select.Content>
						</Select.Root>
					</Field.Field>
					<div class="grid gap-4 sm:grid-cols-2">
						<Field.Field><Field.Label for="crm-edit-contact-name">{text.contactName}</Field.Label><Input id="crm-edit-contact-name" bind:value={name} required /></Field.Field>
						<Field.Field><Field.Label for="crm-edit-contact-title">{text.contactTitle}</Field.Label><Input id="crm-edit-contact-title" bind:value={title} /></Field.Field>
					</div>
					<div class="grid gap-4 sm:grid-cols-2">
						<Field.Field><Field.Label for="crm-edit-contact-email">{text.email}</Field.Label><Input id="crm-edit-contact-email" type="email" bind:value={email} /></Field.Field>
						<Field.Field><Field.Label for="crm-edit-contact-phone">{text.phone}</Field.Label><Input id="crm-edit-contact-phone" type="tel" bind:value={phone} /></Field.Field>
					</div>
					<Field.Field orientation="horizontal">
						<Checkbox id="crm-edit-contact-primary" bind:checked={isPrimary} disabled={!accountID} />
						<Field.Content><Field.Label for="crm-edit-contact-primary">{text.markAsPrimaryContact}</Field.Label></Field.Content>
					</Field.Field>
					<Field.Field><Field.Label for="crm-edit-contact-note">{text.details}</Field.Label><Textarea id="crm-edit-contact-note" rows={8} bind:value={note} /></Field.Field>
					{#if errorMessage}<p class="text-sm text-destructive">{errorMessage}</p>{/if}
				</Field.Group>
			</div>
			<Sheet.Footer class="flex-row justify-end border-t bg-background px-4 py-3">
				<Button type="button" variant="outline" onclick={() => (open = false)} disabled={isSaving}>{text.cancel}</Button>
				<Button type="submit" disabled={isSaving}>{isSaving ? text.creating : text.save}</Button>
			</Sheet.Footer>
		</form>
	</Sheet.Content>
</Sheet.Root>
