<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { changeOwnPassword, WrongCurrentPasswordError } from '$lib/account-password';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	const text = createPageText(companySettingsText);
	const fieldID = $props.id();

	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmedPassword = $state('');
	let isSaving = $state(false);
	let errorMessage = $state('');

	const isReady = $derived(
		currentPassword.length > 0 && newPassword.length > 0 && confirmedPassword.length > 0
	);

	function forget() {
		currentPassword = '';
		newPassword = '';
		confirmedPassword = '';
	}

	async function change(event: SubmitEvent) {
		event.preventDefault();
		if (newPassword !== confirmedPassword) {
			errorMessage = text.passwordMismatch;
			return;
		}
		isSaving = true;
		errorMessage = '';
		try {
			await changeOwnPassword(currentPassword, newPassword);
			forget();
			toast.success(text.passwordChanged);
		} catch (error) {
			errorMessage = messageOf(error);
		} finally {
			isSaving = false;
		}
	}

	function messageOf(error: unknown): string {
		if (error instanceof WrongCurrentPasswordError) return text.currentPasswordWrong;
		return error instanceof Error ? error.message : text.passwordChangeFailed;
	}
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{text.password}</Card.Title>
		<Card.Description>{text.passwordDescription}</Card.Description>
	</Card.Header>
	<Card.Content>
		<form class="grid gap-4" onsubmit={change}>
			<div class="grid gap-1.5">
				<Label for="current-password-{fieldID}">{text.currentPassword}</Label>
				<Input
					id="current-password-{fieldID}"
					type="password"
					autocomplete="current-password"
					bind:value={currentPassword}
					disabled={isSaving}
				/>
			</div>
			<div class="grid gap-1.5">
				<Label for="new-password-{fieldID}">{text.newPassword}</Label>
				<Input
					id="new-password-{fieldID}"
					type="password"
					autocomplete="new-password"
					bind:value={newPassword}
					disabled={isSaving}
				/>
			</div>
			<div class="grid gap-1.5">
				<Label for="confirm-password-{fieldID}">{text.confirmNewPassword}</Label>
				<Input
					id="confirm-password-{fieldID}"
					type="password"
					autocomplete="new-password"
					bind:value={confirmedPassword}
					disabled={isSaving}
				/>
			</div>
			{#if errorMessage}
				<p class="text-sm text-destructive">{errorMessage}</p>
			{/if}
			<Button type="submit" class="w-full" disabled={isSaving || !isReady}>{text.changePassword}</Button>
		</form>
	</Card.Content>
</Card.Root>
