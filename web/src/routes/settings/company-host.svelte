<script lang="ts">
	import { Skeleton } from '$lib/components/ui/skeleton';
	import SvelteMarkdown from '@humanspeak/svelte-markdown';
	import { onMount } from 'svelte';
	import * as Alert from '$lib/components/ui/alert';
	import * as Card from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Spinner } from '$lib/components/ui/spinner';
	import { updatePatienceMilliseconds } from '$lib/host/host-update-model';
	import { hostUpdate } from '$lib/host/host-update.svelte';
	import { filledText as filled } from '$lib/host/host-update-text';
	import { companySettingsText } from './text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';

	let { isAdmin }: { isAdmin: boolean } = $props();

	const text = createPageText(companySettingsText);
	const hostText = text.companyHost;

	let hasRead = $state(false);
	const version = $derived(hostUpdate.version);
	const isReading = $derived(hostUpdate.isReading || !hasRead);
	const actions = $derived(hostUpdate.actions);
	const watch = $derived(hostUpdate.watch);
	const stage = $derived(hostUpdate.stage);
	const settled = $derived(hostUpdate.settled);
	const refusal = $derived(hostUpdate.confirmation ? '' : hostUpdate.refusal);

	onMount(async () => {
		await hostUpdate.read();
		hasRead = true;
	});

	function dateOf(moment: string): string {
		return new Date(moment).toLocaleDateString(currentLocale.value);
	}
</script>

{#snippet versionText(tag: string)}
	<span class="tabular-nums">{tag}</span>
{/snippet}

<Card.Root>
	<Card.Header>
		<Card.Title>{hostText.title}</Card.Title>
		<Card.Description>{hostText.description}</Card.Description>
	</Card.Header>
	<Card.Content class="grid gap-4">
		{#if !version}
			{#if isReading}
				<div role="status" aria-busy="true">
					<span class="sr-only">{hostText.checking}</span>
					<dl aria-hidden="true" class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-6 gap-y-2 text-sm">
						{#each [hostText.installed, hostText.latest, hostText.channel] as label (label)}
							<dt class="text-muted-foreground">{label}</dt>
							<dd class="flex h-5 items-center"><Skeleton class="h-3.5 w-40 max-w-full" /></dd>
						{/each}
					</dl>
				</div>
			{:else}
				<div class="flex flex-wrap items-center justify-between gap-3">
					<p class="text-sm text-muted-foreground">{hostText.unreachable}</p>
					<Button variant="outline" size="sm" onclick={hostUpdate.read}>{hostText.checkAgain}</Button>
				</div>
			{/if}
		{:else}
			<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-6 gap-y-2 text-sm">
				<dt class="text-muted-foreground">{hostText.installed}</dt>
				<dd class="flex flex-wrap gap-x-2">
					{@render versionText(version.installedVersion)}
					{#if version.channel === 'stable' && version.latestStable && !watch}
						<span class="text-muted-foreground">· {version.isUpdateAvailable ? hostText.updateAvailable : hostText.upToDate}</span>
					{/if}
				</dd>
				<dt class="text-muted-foreground">{hostText.latest}</dt>
				<dd class="flex flex-wrap gap-x-2">
					{#if version.latestStable}
						{@render versionText(version.latestStable.version)}
						<span class="text-muted-foreground">· {filled(hostText.released, { date: dateOf(version.latestStable.publishedAt) })}</span>
					{:else}
						<span class="text-muted-foreground">{hostText.noStableRelease}</span>
					{/if}
				</dd>
				<dt class="text-muted-foreground">{hostText.channel}</dt>
				<dd>{hostText.channels[version.channel]}</dd>
			</dl>

			{#if version.latestStable?.notes}
				<section class="grid gap-2">
					<h3 class="text-sm font-medium">{filled(hostText.notesFor, { version: version.latestStable.version })}</h3>
					<div class="prose prose-sm max-h-56 max-w-none overflow-y-auto rounded-md border px-4 py-3 break-words dark:prose-invert prose-headings:mt-0 prose-headings:mb-2 prose-headings:text-sm prose-headings:font-medium">
						<SvelteMarkdown source={version.latestStable.notes} options={{ gfm: true }} />
					</div>
				</section>
			{/if}

			{#if watch && stage}
				{#if stage.kind === 'overdue'}
					<Alert.Root>
						<Spinner />
						<Alert.Title>{filled(hostText.overdue, { minutes: String(updatePatienceMilliseconds / 60000), version: watch.fromVersion })}</Alert.Title>
					</Alert.Root>
				{:else}
					<p role="status" class="flex items-center gap-2 text-sm">
						<Spinner />
						{stage.kind === 'restarting' ? hostText.restarting : filled(hostText.installing, { version: watch.toVersion })}
					</p>
				{/if}
			{:else if settled?.stage.kind === 'finished'}
				<Alert.Root>
					<Alert.Title>{filled(hostText.finished, { version: settled.watch.toVersion })}</Alert.Title>
				</Alert.Root>
			{:else if settled?.stage.kind === 'failed'}
				<Alert.Root variant="destructive">
					<Alert.Title>{filled(hostText.failed, { version: settled.watch.fromVersion })}</Alert.Title>
					{#if settled.stage.reason}<Alert.Description>{settled.stage.reason}</Alert.Description>{/if}
				</Alert.Root>
			{:else if version.lastUpdate}
				{@const lastUpdate = version.lastUpdate}
				{#if lastUpdate.succeeded}
					<p class="text-sm text-muted-foreground">{filled(hostText.lastUpdateSucceeded, { version: lastUpdate.toVersion, date: dateOf(lastUpdate.finishedAt) })}</p>
				{:else}
					<Alert.Root variant="destructive">
						<Alert.Title>{filled(hostText.lastUpdateFailed, { version: lastUpdate.toVersion, date: dateOf(lastUpdate.finishedAt) })}</Alert.Title>
						{#if lastUpdate.error}<Alert.Description>{lastUpdate.error}</Alert.Description>{/if}
					</Alert.Root>
				{/if}
			{/if}

			{#if refusal}
				<Alert.Root variant="destructive">
					<Alert.Title>{refusal}</Alert.Title>
				</Alert.Root>
			{/if}

			{#if isAdmin && actions && !watch}
				{#if actions.kind === 'brew'}
					<div class="grid gap-2 border-t pt-4">
						<p class="text-sm text-muted-foreground">{hostText.macHost}</p>
						<div class="flex items-center justify-between gap-2 rounded-md border bg-muted px-3 py-1.5">
							<code class="font-mono text-sm">{hostText.macCommand}</code>
							<CopyButton text={hostText.macCommand} size="sm" />
						</div>
					</div>
				{:else if actions.kind === 'offStable'}
					<p class="border-t pt-4 text-sm text-muted-foreground">{hostText.testingHost}</p>
				{:else if actions.kind === 'unpackaged'}
					<p class="border-t pt-4 text-sm text-muted-foreground">{hostText.unpackagedHost}</p>
				{:else if actions.kind === 'choices' && (actions.update || actions.goBack)}
					<div class="flex flex-wrap justify-end gap-2 border-t pt-4">
						{#if actions.goBack}
							{@const goBack = actions.goBack}
							<Button variant="outline" onclick={() => hostUpdate.confirm(goBack, true)}>
								{hostText.goBack}
							</Button>
						{/if}
						{#if actions.update}
							{@const update = actions.update}
							<Button onclick={() => hostUpdate.confirm(update, false)}>{hostText.updateNow}</Button>
						{/if}
					</div>
				{/if}
			{/if}
		{/if}
	</Card.Content>
</Card.Root>
