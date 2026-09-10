<script lang="ts">
	import { page } from '$app/state';
	import { taskRunDetailPathOf } from '$lib/app-shell';
	import { Button } from '$lib/components/ui/button';
	import { Spinner } from '$lib/components/ui/spinner';
	import RotateCcwIcon from '@lucide/svelte/icons/rotate-ccw';
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { retryTaskRun } from './runs-api';

	let { taskRunID, label, pendingLabel, successMessage, errorMessage }: {
		taskRunID: string;
		label: string;
		pendingLabel: string;
		successMessage: string;
		errorMessage: string;
	} = $props();
	let isRetrying = $state(false);

	async function retry(event: MouseEvent) {
		event.stopPropagation();
		if (isRetrying) return;
		isRetrying = true;
		try {
			const response = await retryTaskRun(taskRunID);
			toast.success(successMessage);
			await goto(taskRunDetailPathOf(page.url.pathname, response.taskRunID));
		} catch {
			toast.error(errorMessage);
		} finally {
			isRetrying = false;
		}
	}
</script>

<Button type="button" variant="outline" size="sm" disabled={isRetrying} onclick={(event) => void retry(event)}>
	{#if isRetrying}
		<Spinner data-icon="inline-start" />
		{pendingLabel}
	{:else}
		<RotateCcwIcon data-icon="inline-start" />
		{label}
	{/if}
</Button>
