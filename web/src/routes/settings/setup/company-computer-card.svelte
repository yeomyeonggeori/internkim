<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Code from '$lib/components/ui/code';
	import * as Collapsible from '$lib/components/ui/collapsible';
	import { isCompanyAppRunning } from '$lib/host-bridge';
	import { hostInstallCommands, type HostConfiguration, type HostSetupStatus } from '$lib/company/host-setup';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import BoxConnection from './box-connection.svelte';
	import { downloadHostConfiguration, fetchHostSetupStatus, requestHostConfiguration } from './host-setup-client';
	import { stepHeading } from './step-heading';
	import { hostSetupText } from './text';

	let { status = $bindable(), errorMessage = $bindable(), step }: { status: HostSetupStatus; errorMessage: string; step?: number } = $props();

	const text = createPageText(hostSetupText);
	let configuration = $state<HostConfiguration | null>(null);
	let isDownloading = $state(false);
	let isChecking = $state(false);
	let isReplacing = $state(false);
	let connection = $state<'unchecked' | 'connected' | 'offline' | 'unavailable'>('unchecked');
	let isCompanyComputerOnline = $state(false);

	onMount(async () => {
		isCompanyComputerOnline = await isCompanyAppRunning().catch(() => false);
	});

	function askForDownload() {
		if (configuration) return downloadHostConfiguration(configuration);
		if (status.hasConfiguration) { isReplacing = true; return; }
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
			isCompanyComputerOnline = await isCompanyAppRunning();
			connection = isCompanyComputerOnline ? 'connected' : 'offline';
		} catch {
			connection = 'unavailable';
		} finally {
			isChecking = false;
		}
	}
</script>

<Card.Root>
	<Card.Header>
		<Card.Title>{stepHeading(text.computer, step)}</Card.Title>
		<Card.Description>{text.computerDescription}</Card.Description>
	</Card.Header>
	<Card.Content class="grid min-w-0 gap-6">
		<BoxConnection {isCompanyComputerOnline} />
		<Collapsible.Root class="grid gap-4">
			<Collapsible.Trigger class="inline-flex min-h-11 items-center justify-self-start text-sm underline">{text.otherComputer}</Collapsible.Trigger>
			<Collapsible.Content class="grid min-w-0 gap-4">
				<p class="text-sm text-muted-foreground">{text.otherComputerDescription}</p>
				<div class="grid justify-items-start gap-2">
					<Button variant="outline" onclick={askForDownload} disabled={isDownloading}>
						{isDownloading ? text.downloading : configuration ? text.downloadAgain : text.download}
					</Button>
					<p class="text-sm text-muted-foreground">{text.fileHint}</p>
				</div>
				<div class="grid gap-2 rounded-md border border-dashed p-4">
					<p class="text-sm font-medium">{text.install}</p>
					<p class="text-sm text-muted-foreground">{text.installDescription}</p>
					<Code.Root lang="bash" hideLines code={hostInstallCommands} class="text-xs">
						<Code.CopyButton />
					</Code.Root>
					<a class="inline-flex min-h-11 items-center justify-self-start text-sm underline" href={currentLocale.value === 'ko' ? 'https://docs.intern.kim/ko/quickstart' : 'https://docs.intern.kim/quickstart'}>{text.guide}</a>
				</div>
				<p class="text-sm text-muted-foreground">{text.installHint}</p>
			</Collapsible.Content>
		</Collapsible.Root>
	</Card.Content>
	<Card.Footer class="flex-col items-start gap-3">
		<Button variant="outline" onclick={checkConnection} disabled={isChecking}>{isChecking ? text.checking : text.check}</Button>
		{#if connection !== 'unchecked'}<p role="status" class="text-sm">{text[connection]}</p>{/if}
	</Card.Footer>
</Card.Root>

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
