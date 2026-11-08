<script lang="ts">
	import { z } from 'zod';
	import { toast } from 'svelte-sonner';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Empty from '$lib/components/ui/empty';
	import * as Field from '$lib/components/ui/field';
	import * as ToggleGroup from '$lib/components/ui/toggle-group';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Spinner } from '$lib/components/ui/spinner';
	import { invokeTool } from '$lib/public-api-call';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { taskInvitationText } from '$lib/task-invitation/text';
	import { taskInvitationListResultSchema } from '$lib/task-invitation/invitations';

	type Invitation = z.infer<typeof taskInvitationListResultSchema>['invitations'][number];
	type Props = { businesses: string[]; businessColor: (business: string) => string };

	let { businesses, businessColor }: Props = $props();
	const text = createPageText(taskInvitationText);
	const fieldID = $props.id();
	let isOpen = $state(false);
	let isBusy = $state(false);
	let email = $state('');
	let chosenBusinesses = $state<string[]>([]);
	let invitations = $state<Invitation[]>([]);
	let errorMessage = $state('');
	const canSend = $derived(email.trim() !== '' && chosenBusinesses.length > 0 && !isBusy);

	function failureOf(error: unknown): string {
		return error instanceof Error ? error.message : text.failure;
	}

	async function refresh() {
		invitations = taskInvitationListResultSchema.parse(await invokeTool('task_invitation_list', {})).invitations;
	}

	async function act(action: () => Promise<void>) {
		isBusy = true;
		errorMessage = '';
		try {
			await action();
		} catch (error) {
			errorMessage = failureOf(error);
		} finally {
			isBusy = false;
		}
	}

	function open() {
		isOpen = true;
		void act(refresh);
	}

	function invite(recipient: string, invited: string[]) {
		return act(async () => {
			await invokeTool('task_invite', { email: recipient, businesses: invited });
			toast.success(text.sent.replace('{email}', recipient));
			await refresh();
		});
	}

	async function send(event: SubmitEvent) {
		event.preventDefault();
		await invite(email.trim(), chosenBusinesses);
		if (errorMessage) return;
		email = '';
		chosenBusinesses = [];
	}

	function revoke(invitation: Invitation) {
		return act(async () => {
			await invokeTool('task_invitation_delete', { invitationID: invitation.invitationID });
			toast.success(text.revoked.replace('{email}', invitation.email));
			await refresh();
		});
	}

	function statusOf(invitation: Invitation): string {
		if (!invitation.acceptedAt) return text.pending;
		return `${text.accepted} · ${new Date(invitation.acceptedAt).toLocaleDateString(currentLocale.value)}`;
	}
</script>

<Button size="sm" variant="outline" onclick={open}>{text.open}</Button>
<Dialog.Root bind:open={isOpen}>
	<Dialog.Content class="max-h-[85vh] overflow-auto sm:max-w-xl">
		<Dialog.Header>
			<Dialog.Title>{text.title}</Dialog.Title>
			<Dialog.Description>{text.description}</Dialog.Description>
		</Dialog.Header>
		{#if businesses.length}
			<form onsubmit={send} class="grid gap-4">
				<Field.Group>
					<Field.Field>
						<Field.Label for="{fieldID}-email">{text.email}</Field.Label>
						<Input id="{fieldID}-email" type="email" required autocomplete="off" bind:value={email} disabled={isBusy} />
					</Field.Field>
					<Field.Field>
						<Field.Label>{text.businesses}</Field.Label>
						<ToggleGroup.Root type="multiple" bind:value={chosenBusinesses} variant="outline" size="sm" spacing={2}
							disabled={isBusy} aria-label={text.businesses} class="flex-wrap justify-start">
							{#each businesses as business (business)}
								<ToggleGroup.Item value={business}>
									<span class="size-2 rounded-full" style:background-color={businessColor(business)}></span>{business}
								</ToggleGroup.Item>
							{/each}
						</ToggleGroup.Root>
						<Field.Description>{text.scope}</Field.Description>
					</Field.Field>
				</Field.Group>
				<Button type="submit" disabled={!canSend}>{#if isBusy}<Spinner />{/if}{text.send}</Button>
			</form>
		{:else}
			<p class="text-sm text-muted-foreground">{text.noBusinesses}</p>
		{/if}
		{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
		<section class="grid gap-3 border-t pt-4">
			<h2 class="text-sm font-semibold">{text.invitations}</h2>
			{#each invitations as invitation (invitation.invitationID)}
				<div class="flex items-center justify-between gap-3 text-sm">
					<div class="min-w-0">
						<p class="truncate font-medium">{invitation.email}</p>
						<p class="truncate text-xs text-muted-foreground">{invitation.businesses.join(' · ')} · {statusOf(invitation)}</p>
					</div>
					<div class="flex shrink-0 gap-2">
						<Button variant="ghost" size="sm" disabled={isBusy}
							onclick={() => invite(invitation.email, invitation.businesses)}>{text.resend}</Button>
						<Button variant="outline" size="sm" disabled={isBusy} onclick={() => revoke(invitation)}>{text.revoke}</Button>
					</div>
				</div>
			{:else}
				{#if !isBusy}<Empty.Root class="p-3"><Empty.Header><Empty.Title>{text.noInvitations}</Empty.Title></Empty.Header></Empty.Root>{/if}
			{/each}
		</section>
	</Dialog.Content>
</Dialog.Root>
