<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { z } from 'zod';
	import { toast } from 'svelte-sonner';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import RefreshButton from '$lib/components/refresh-button.svelte';
	import { supabase } from '$lib/supabase';
	import { isPasskeySupported, listPasskeys, refusalOf, registerPasskey } from '$lib/supabase-passkey';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import GuestSignIn from '$lib/task-invitation/guest-sign-in.svelte';
	import { sharedTaskBoardSchema, sharedTasks, sharedTaskTotal, type SharedTaskBoard } from '$lib/task-invitation/board';
	import { SharedTasksRefused, sharedTasksRequest, signedInEmail } from '$lib/task-invitation/guest';
	import { sharedTasksText } from '$lib/task-invitation/text';
	import TaskListView from '../../../task/task-list-view.svelte';
	import { taskText } from '../../../task/text';
	import { taskBusinessColor, taskTypeColor } from '../../../task/task-definition-colors';
	import type { TaskDefinitions } from '../../../task/task-types';

	const text = createPageText(sharedTasksText);
	const pageText = createPageText(taskText);
	const companyID = $derived(z.guid().parse(page.params.companyID));
	let board = $state<SharedTaskBoard | null>(null);
	let viewer = $state<string | null>(null);
	let isLoading = $state(true);
	let failure = $state('');
	let offersPasskey = $state(false);
	const tasks = $derived(board ? sharedTasks(board) : []);
	const hiddenCount = $derived(board ? sharedTaskTotal(board) - tasks.length : 0);
	const definitions = $derived(definitionsOf(board));

	function definitionsOf(shown: SharedTaskBoard | null): TaskDefinitions {
		const businesses = shown?.businesses ?? [];
		const types = shown?.types ?? [];
		return {
			categories: businesses.map((business) => business.name),
			categoryColors: colorsOf(businesses),
			types: types.map((type) => type.name),
			typeColors: colorsOf(types),
			sizes: []
		};
	}

	function colorsOf(named: { name: string; color?: string }[]): Record<string, string> {
		return Object.fromEntries(named.flatMap((item) => (item.color ? [[item.name, item.color]] : [])));
	}

	function statusLabel(status: string): string {
		const labels: Record<string, string> = pageText.status;
		return labels[status] ?? status;
	}

	async function load() {
		failure = '';
		viewer = await signedInEmail();
		if (!viewer) {
			isLoading = false;
			return;
		}
		try {
			board = sharedTaskBoardSchema.parse(await sharedTasksRequest(`/api/v1/tasks/shared/${companyID}`));
		} catch (error) {
			board = null;
			failure = error instanceof SharedTasksRefused && error.status === 401 ? '' : error instanceof Error ? error.message : text.failure;
		} finally {
			isLoading = false;
		}
		if (board) offersPasskey = isPasskeySupported() && (await listPasskeys()).length === 0;
	}

	async function addPasskey() {
		try {
			await registerPasskey();
			offersPasskey = false;
			toast.success(text.passkeyRegistered);
		} catch (error) {
			if (refusalOf(error) !== 'cancelled') toast.error(error instanceof Error ? error.message : text.failure);
		}
	}

	async function signOut() {
		await supabase().auth.signOut();
		board = null;
		viewer = null;
		offersPasskey = false;
	}

	function cannotUpdate(): boolean {
		return false;
	}

	function noStatusOptions(): [] {
		return [];
	}

	function noEmail(): string {
		return '';
	}

	async function ignoreStatusChange(): Promise<void> {}

	onMount(load);
</script>

<svelte:head><title>{board ? `${board.company.name} · ${text.pageTitle}` : text.pageTitle}</title><meta name="robots" content="noindex" /></svelte:head>
<main class="mx-auto grid max-w-5xl grid-cols-[minmax(0,1fr)] gap-6 px-4 py-10 md:px-6">
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div class="grid gap-1">
			{#if isLoading}<Skeleton class="h-7 w-48" />{:else}<h1 class="text-2xl font-semibold">{board?.company.name ?? text.pageTitle}</h1>{/if}
			{#if viewer}<p class="text-sm text-muted-foreground">{text.viewer.replace('{email}', viewer)}</p>{/if}
		</div>
		{#if viewer}
			<div class="flex items-center gap-2">
				<RefreshButton label={text.refresh} variant="ghost" size="icon-sm" refresh={load} />
				<Button variant="ghost" size="sm" onclick={signOut}>{text.signOut}</Button>
			</div>
		{/if}
	</header>
	{#if offersPasskey}
		<div class="flex flex-wrap items-center justify-between gap-3 rounded-lg border p-4">
			<p class="text-sm">{text.passkeyOffer}</p>
			<Button size="sm" variant="outline" onclick={addPasskey}>{text.registerPasskey}</Button>
		</div>
	{/if}
	{#if failure}<p role="alert" class="text-sm text-destructive">{failure}</p>{/if}
	{#if !isLoading && !viewer}
		<section class="grid max-w-md gap-4">
			<p class="text-sm text-muted-foreground">{text.signInAgain}</p>
			<GuestSignIn onSignedIn={load} />
		</section>
	{:else if isLoading}
		<div class="grid gap-2">{#each [0, 1, 2] as row (row)}<Skeleton class="h-10 w-full" />{/each}</div>
	{:else if board}
		<div class="grid gap-2">
			<TaskListView
				{tasks}
				emptyLabel={text.empty}
				showEmpty={true}
				text={pageText}
				statusOptionsForTask={noStatusOptions}
				pendingStatusTaskID=""
				{statusLabel}
				updateTaskStatus={ignoreStatusChange}
				canUpdateTask={cannotUpdate}
				focusedTaskID=""
				memberEmail={noEmail}
				businessColor={(business) => taskBusinessColor(business, definitions)}
				taskTypeColor={(type) => taskTypeColor(type, definitions)}
			/>
			{#if hiddenCount > 0}<p class="text-xs text-muted-foreground tabular-nums">{text.moreTasks.replace('{count}', String(hiddenCount))}</p>{/if}
		</div>
	{/if}
</main>
