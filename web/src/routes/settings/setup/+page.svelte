<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { ensureAgentConversation } from '$lib/components/channel/channel-api';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { isCompanyAppRunning } from '$lib/host-bridge';
	import { companyPathOf } from '$lib/company-path';
	import type { HostConfiguration, HostSetupStatus } from '$lib/company/host-setup';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import CompanyMembers from '../company-members.svelte';
	import { downloadHostConfiguration, fetchHostSetupStatus, requestHostConfiguration } from './host-setup-client';
	import { hostSetupText } from './text';

	const text = createPageText(hostSetupText);
	let status = $state<HostSetupStatus | null>(null);
	let configuration = $state<HostConfiguration | null>(null);
	let isLoading = $state(true);
	let isDownloading = $state(false);
	let isChecking = $state(false);
	let isReplacing = $state(false);
	let isOpeningMessenger = $state(false);
	let errorMessage = $state('');
	let connection = $state<'unchecked' | 'connected' | 'offline' | 'unavailable'>('unchecked');

	onMount(async () => {
		try {
			status = await fetchHostSetupStatus();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failed;
		} finally {
			isLoading = false;
		}
	});

	function askForDownload() {
		if (configuration) return downloadHostConfiguration(configuration);
		if (status?.hasConfiguration) { isReplacing = true; return; }
		void download(false);
	}

	async function download(replaceExisting: boolean) {
		isDownloading = true;
		errorMessage = '';
		try {
			configuration = await requestHostConfiguration(replaceExisting);
			downloadHostConfiguration(configuration);
			status = await fetchHostSetupStatus();
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.failed;
		} finally {
			isDownloading = false;
		}
	}

	async function checkConnection() {
		isChecking = true;
		try {
			connection = await isCompanyAppRunning() ? 'connected' : 'offline';
		} catch {
			connection = 'unavailable';
		} finally {
			isChecking = false;
		}
	}

	async function openMessenger() {
		if (!status) return;
		isOpeningMessenger = true;
		try {
			const channelID = await ensureAgentConversation();
			await goto(`${companyPathOf(status.company.slug, '/messenger')}?channel=${encodeURIComponent(channelID)}`);
		} catch {
			errorMessage = text.unavailable;
		} finally {
			isOpeningMessenger = false;
		}
	}
</script>

<svelte:head><title>{text.title}</title></svelte:head>

<main class="min-h-0 flex-1 overflow-y-auto">
	<div class="mx-auto grid max-w-3xl gap-6 px-4 py-8 sm:px-6">
		<header class="grid gap-2">
			<h1 class="text-2xl font-semibold">{text.title}</h1>
			<p class="text-muted-foreground">{text.description}</p>
		</header>
		{#if errorMessage}<p role="alert" class="text-sm text-destructive">{errorMessage}</p>{/if}
		{#if isLoading}
			<p role="status">{text.loading}</p>
		{:else if status}
			<p class="text-sm">{status.company.name} · {text.accountReady}</p>
			<Card.Root>
				<Card.Header>
					<Card.Title>{text.computer}</Card.Title>
					<Card.Description>{text.computerDescription}</Card.Description>
				</Card.Header>
				<Card.Content class="grid min-w-0 gap-4">
					<div class="grid justify-items-start gap-2">
						<Button onclick={askForDownload} disabled={isDownloading}>
							{isDownloading ? text.downloading : configuration ? text.downloadAgain : text.download}
						</Button>
						<p class="text-sm text-muted-foreground">{text.fileHint}</p>
					</div>
					<div class="grid gap-2 rounded-md border border-dashed p-4">
						<p class="text-sm font-medium">{text.install}</p>
						<p class="text-sm text-muted-foreground">{text.installMeantime}</p>
						<a class="justify-self-start text-sm underline" href={currentLocale.value === 'ko' ? 'https://docs.intern.kim/ko/docs/quickstart' : 'https://docs.intern.kim/docs/quickstart'}>{text.guide}</a>
					</div>
					<p class="text-sm text-muted-foreground">{text.installHint}</p>
				</Card.Content>
				<Card.Footer class="flex-col items-start gap-3">
					<Button variant="outline" onclick={checkConnection} disabled={isChecking}>{isChecking ? text.checking : text.check}</Button>
					{#if connection !== 'unchecked'}<p role="status" class="text-sm">{text[connection]}</p>{/if}
				</Card.Footer>
			</Card.Root>
			<section class="grid gap-3">
				<h2 class="text-xl font-semibold">{text.invite}</h2>
				<p class="text-sm text-muted-foreground">{text.inviteDescription}</p>
				<CompanyMembers />
			</section>
			<section class="grid justify-items-start gap-3">
				<h2 class="text-xl font-semibold">{text.tryIt}</h2>
				<p class="text-sm text-muted-foreground">{text.tryItDescription}</p>
				<blockquote class="border-l-2 pl-4 text-sm">{text.firstMessage}</blockquote>
				<Button onclick={openMessenger} disabled={isOpeningMessenger}>{text.openMessenger}</Button>
			</section>
		{/if}
	</div>
</main>

<AlertDialog.Root bind:open={isReplacing}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{text.replaceTitle}</AlertDialog.Title>
			<AlertDialog.Description>{text.replaceDescription}</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel>{text.cancel}</AlertDialog.Cancel>
			<AlertDialog.Action onclick={() => download(true)}>{text.replace}</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
