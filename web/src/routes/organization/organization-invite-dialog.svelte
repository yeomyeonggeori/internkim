<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Dialog from '$lib/components/ui/dialog';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { inviteMemberToCompany, type MemberInvitation } from '$lib/organization/invite-member';
	import { organizationDirectoryText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';

	let { isOpen = $bindable(false), onInvited }: { isOpen?: boolean; onInvited: () => void | Promise<void> } = $props();

	const text = createPageText(organizationDirectoryText);
	const fieldID = $props.id();
	let email = $state('');
	let isSaving = $state(false);
	let errorMessage = $state('');
	let invitation = $state<MemberInvitation | null>(null);

	async function invite(event: SubmitEvent) {
		event.preventDefault();
		isSaving = true;
		errorMessage = '';
		try {
			invitation = await inviteMemberToCompany(email);
			email = '';
			await onInvited();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.inviteFailed;
		} finally {
			isSaving = false;
		}
	}

	function close() {
		isOpen = false;
		invitation = null;
		errorMessage = '';
	}
</script>

<Dialog.Root bind:open={isOpen} onOpenChange={(open) => !open && close()}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{text.inviteMember}</Dialog.Title>
			<Dialog.Description>{text.inviteDescription}</Dialog.Description>
		</Dialog.Header>
		{#if invitation}
			<div class="grid gap-2">
				<p class="text-sm">{invitation.email}</p>
				<p class="rounded-lg border bg-muted p-3 font-mono text-lg select-all">{invitation.temporaryPassword}</p>
				<p class="text-xs text-muted-foreground">{text.inviteHandOver}</p>
			</div>
			<Dialog.Footer>
				<Button onclick={close}>{text.done}</Button>
			</Dialog.Footer>
		{:else}
			<form class="grid gap-4" onsubmit={invite}>
				<div class="grid gap-1.5">
					<Label for="invite-email-{fieldID}">{text.inviteEmail}</Label>
					<Input
						id="invite-email-{fieldID}"
						type="email"
						autocomplete="off"
						bind:value={email}
						disabled={isSaving}
					/>
				</div>
				{#if errorMessage}
					<p class="text-sm text-destructive">{errorMessage}</p>
				{/if}
				<Dialog.Footer>
					<Button type="button" variant="outline" disabled={isSaving} onclick={close}>{text.cancel}</Button>
					<Button type="submit" disabled={isSaving || !email.trim()}>{text.invite}</Button>
				</Dialog.Footer>
			</form>
		{/if}
	</Dialog.Content>
</Dialog.Root>
