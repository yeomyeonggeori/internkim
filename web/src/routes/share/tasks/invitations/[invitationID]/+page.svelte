<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import GuestSignIn from '$lib/task-invitation/guest-sign-in.svelte';
	import { acceptedTaskInvitationSchema } from '$lib/task-invitation/board';
	import { sharedTasksRequest, signedInEmail } from '$lib/task-invitation/guest';
	import { sharedTasksText } from '$lib/task-invitation/text';

	const text = createPageText(sharedTasksText);
	let isCheckingSession = $state(true);
	let failure = $state('');

	async function accept() {
		const answer = await sharedTasksRequest(`/api/v1/tasks/invitations/${page.params.invitationID}`, 'POST');
		await goto(`/share/tasks/${acceptedTaskInvitationSchema.parse(answer).companyID}`);
	}

	onMount(async () => {
		try {
			if (await signedInEmail()) await accept();
		} catch (error) {
			failure = error instanceof Error ? error.message : String(error);
		} finally {
			isCheckingSession = false;
		}
	});
</script>

<svelte:head><title>{text.invitationTitle}</title><meta name="robots" content="noindex" /></svelte:head>
<main class="mx-auto grid max-w-md gap-6 px-6 py-16">
	<div class="grid gap-2">
		<h1 class="text-2xl font-semibold">{text.invitationTitle}</h1>
		<p class="text-sm text-muted-foreground">{text.invitationDescription}</p>
	</div>
	{#if failure}<p role="alert" class="text-sm text-destructive">{failure}</p>{/if}
	{#if !isCheckingSession}<GuestSignIn onSignedIn={accept} />{/if}
</main>
