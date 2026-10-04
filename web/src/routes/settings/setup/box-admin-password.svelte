<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { minimumAdminPasswordLength, type AdminPasswordStatus, type ConnectedBox } from '$lib/company/box';
	import { fetchAdminPasswordStatus, setBoxAdminPassword } from './box-admin-password-client';
	import { hostSetupText } from './text';

	let { box }: { box: ConnectedBox } = $props();

	const text = createPageText(hostSetupText);
	const pollMilliseconds = 5000;

	let status = $state<AdminPasswordStatus | null>(null);
	let isFormOpen = $state(false);
	let password = $state('');
	let confirmation = $state('');
	let isSending = $state(false);
	let errorMessage = $state('');
	let pollTimer: ReturnType<typeof setInterval> | undefined;

	const hasEverBeenSet = $derived(status !== null && (status.pendingSettingID !== null || status.outcome !== null));
	const isWaitingForTheBox = $derived(Boolean(status?.pendingSettingID));

	onMount(() => {
		void refresh();
		pollTimer = setInterval(() => void refreshWhileUnsettled(), pollMilliseconds);
	});
	onDestroy(() => clearInterval(pollTimer));

	async function refresh() {
		try {
			status = await fetchAdminPasswordStatus();
			if (status.hasAdminAccount && !hasEverBeenSet) isFormOpen = true;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		}
	}

	async function refreshWhileUnsettled() {
		if (status?.hasAdminAccount && !isWaitingForTheBox) return;
		await refresh();
	}

	function problemWith(typed: string, confirmed: string): string {
		if (typed.length < minimumAdminPasswordLength) return text.adminPasswordTooShort.replace('{count}', String(minimumAdminPasswordLength));
		if (typed !== confirmed) return text.adminPasswordMismatch;
		return '';
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errorMessage = problemWith(password, confirmation);
		if (errorMessage) return;
		isSending = true;
		try {
			status = (await setBoxAdminPassword(box, password)).status;
			isFormOpen = false;
			password = '';
			confirmation = '';
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.boxFailed;
		} finally {
			isSending = false;
		}
	}

	function closeForm() {
		isFormOpen = false;
		password = '';
		confirmation = '';
		errorMessage = '';
	}
</script>

{#if status?.hasAdminAccount}
	<div class="grid gap-2">
		{#if isFormOpen}
			<form class="grid gap-2" onsubmit={submit}>
				<p class="text-sm font-medium">{text.adminPasswordTitle}</p>
				<p class="text-muted-foreground text-sm">{text.adminPasswordDescription}</p>
				<Field.Field>
					<Field.Label for="box-admin-password">{text.adminPassword}</Field.Label>
					<Input id="box-admin-password" type="password" bind:value={password} autocomplete="new-password" />
				</Field.Field>
				<Field.Field>
					<Field.Label for="box-admin-password-confirmation">{text.adminPasswordConfirm}</Field.Label>
					<Input id="box-admin-password-confirmation" type="password" bind:value={confirmation} autocomplete="new-password" />
				</Field.Field>
				<div class="flex gap-2">
					<Button type="submit" disabled={isSending}>{isSending ? text.adminPasswordApplying : text.adminPasswordApply}</Button>
					{#if hasEverBeenSet}
						<Button type="button" variant="ghost" onclick={closeForm} disabled={isSending}>{text.cancel}</Button>
					{/if}
				</div>
			</form>
		{:else}
			<Button variant="outline" onclick={() => (isFormOpen = true)}>{text.adminPasswordChange}</Button>
		{/if}

		{#if isWaitingForTheBox}
			<p role="status" class="text-sm">{text.adminPasswordApplying}</p>
		{:else if status.outcome?.result === 'applied'}
			<p role="status" class="text-sm">{text.adminPasswordApplied}</p>
		{:else if status.outcome?.result === 'failed'}
			<p role="status" class="text-sm">{text.adminPasswordFailed}</p>
		{/if}

		{#if errorMessage}<Field.Error>{errorMessage}</Field.Error>{/if}
	</div>
{/if}
