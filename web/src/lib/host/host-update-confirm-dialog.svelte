<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Spinner } from '$lib/components/ui/spinner';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { hostUpdate } from './host-update.svelte';
	import { filledText, hostUpdateText } from './host-update-text';

	const text = createPageText(hostUpdateText);
	const confirmation = $derived(hostUpdate.confirmation);
	const downtimeSeconds = $derived(hostUpdate.version?.expectedDowntimeSeconds);
</script>

<AlertDialog.Root open={confirmation !== null} onOpenChange={(open) => !open && hostUpdate.dismiss()}>
	<AlertDialog.Content>
		{#if confirmation}
			<AlertDialog.Header>
				<AlertDialog.Title>
					{filledText(confirmation.isGoingBack ? text.confirmGoBackTitle : text.confirmUpdateTitle, { version: confirmation.release.version })}
				</AlertDialog.Title>
				<AlertDialog.Description>
					{text.confirmDescription}
					{#if downtimeSeconds}
						{filledText(text.confirmDuration, { seconds: String(downtimeSeconds) })}
					{/if}
				</AlertDialog.Description>
			</AlertDialog.Header>
			{#if hostUpdate.refusal}
				<p role="alert" class="text-sm text-destructive">{hostUpdate.refusal}</p>
			{/if}
			<AlertDialog.Footer>
				<AlertDialog.Cancel type="button" disabled={hostUpdate.isStarting}>{text.cancel}</AlertDialog.Cancel>
				<AlertDialog.Action type="button" disabled={hostUpdate.isStarting} onclick={() => hostUpdate.start(text)}>
					{#if hostUpdate.isStarting}<Spinner data-icon="inline-start" />{/if}
					{confirmation.isGoingBack ? text.confirmGoBack : text.confirmUpdate}
				</AlertDialog.Action>
			</AlertDialog.Footer>
		{/if}
	</AlertDialog.Content>
</AlertDialog.Root>
