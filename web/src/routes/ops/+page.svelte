<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Badge } from '$lib/components/ui/badge';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import { onMount } from 'svelte';
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import BrainCircuitIcon from '@lucide/svelte/icons/brain-circuit';
	import CloudUploadIcon from '@lucide/svelte/icons/cloud-upload';
	import FlaskConicalIcon from '@lucide/svelte/icons/flask-conical';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import SaveIcon from '@lucide/svelte/icons/save';
	import SearchCheckIcon from '@lucide/svelte/icons/search-check';
	import ServerIcon from '@lucide/svelte/icons/server';
	import ShieldIcon from '@lucide/svelte/icons/shield';
	import TerminalIcon from '@lucide/svelte/icons/terminal';
	import { fetchLocalFleetStatus, fetchTargets, fetchTargetStatus, startJob, startLocalFleetJob, streamJobEvents, updateTargetModel } from './ops-api';
	import type { Job, LLMModelStatus, LocalFleetJobRequest, LocalFleetStatus, OpsTarget, TargetStatus } from './ops-types';
	import { endpointStatusLabel, hasVersion, readableModelLabel, runtimeVersionDetail, shortRelease, shortVersion, statusBadgeVariant } from './ops-view';

	const actions = [
		{ id: 'check', label: 'Check', icon: SearchCheckIcon, variant: 'outline' },
		{ id: 'deploy-admind', label: 'Deploy admind', icon: CloudUploadIcon, variant: 'default' },
		{ id: 'deploy-runtime', label: 'Deploy runtime', icon: CloudUploadIcon, variant: 'secondary' },
		{ id: 'deploy-web', label: 'Deploy web', icon: CloudUploadIcon, variant: 'secondary' },
		{ id: 'apply-release', label: 'Apply release', icon: CloudUploadIcon, variant: 'default' },
		{ id: 'pilot-standard', label: 'Pilot standard', icon: ActivityIcon, variant: 'default' },
		{ id: 'restart-cloudflared-node-ssh', label: 'Restart tunnel', icon: RotateCwIcon, variant: 'outline' },
		{ id: 'restart-ssh', label: 'Restart SSH', icon: RotateCwIcon, variant: 'outline' }
	] as const;

	const modelPresets = ['google/gemini-3.5-flash', 'google/gemini-3.1-flash-lite', 'openai/gpt-5.4-nano', 'x-ai/grok-4.3'];

	let targets = $state<OpsTarget[]>([]);
	let statuses = $state<Record<string, TargetStatus>>({});
	let localFleetStatus = $state<LocalFleetStatus | undefined>();
	let jobs = $state<Record<string, Job>>({});
	let modelDrafts = $state<Record<string, string>>({});
	let modelErrors = $state<Record<string, string>>({});
	let modelSaving = $state<Record<string, boolean>>({});
	let selectedJobID = $state('');
	let selectedTargetID = $state('');
	let isLoading = $state(false);
	let errorMessage = $state('');

	const selectedJob = () => (selectedJobID ? jobs[selectedJobID] : undefined);
	const selectedJobEvents = () => selectedJob()?.events ?? [];
	const selectedTarget = () => targets.find((target) => target.id === selectedTargetID) ?? targets[0];
	const targetStatus = (targetID: string) => statuses[targetID];
	const displayStatus = (targetID: string) => targetStatus(targetID) ?? pendingTargetStatus(targetID);
	const releaseActionDisabled = (status: TargetStatus | undefined) => !status?.release.updateAllowed || status.release.state === 'current' || status.release.state === 'updating';

	onMount(() => {
		void loadTargets();
	});

	async function loadTargets() {
		isLoading = true;
		errorMessage = '';
		try {
			targets = await fetchTargets();
			selectedTargetID = selectedTargetID || targets[0]?.id || '';
			const [nextStatuses, nextLocalFleetStatus] = await Promise.all([fetchStatuses(targets), readLocalFleetStatus()]);
			statuses = nextStatuses;
			localFleetStatus = nextLocalFleetStatus;
			syncModelDrafts(nextStatuses, false);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'failed to load targets';
		} finally {
			isLoading = false;
		}
	}

	async function readLocalFleetStatus(): Promise<LocalFleetStatus | undefined> {
		try {
			return await fetchLocalFleetStatus();
		} catch {
			return undefined;
		}
	}

	async function fetchStatuses(nextTargets: OpsTarget[]): Promise<Record<string, TargetStatus>> {
		const entries = await Promise.all(nextTargets.map(async (target) => [target.id, await readTargetStatus(target.id)] as const));
		return Object.fromEntries(entries);
	}

	async function checkTarget(targetID: string) {
		const status = await readTargetStatus(targetID);
		statuses = { ...statuses, [targetID]: status };
		syncModelDrafts({ [targetID]: status }, true);
	}

	async function readTargetStatus(targetID: string): Promise<TargetStatus> {
		try {
			return await fetchTargetStatus(targetID);
		} catch (error) {
			const message = error instanceof Error ? error.message : 'status check failed';
			return {
				targetID,
				checkedAt: new Date().toISOString(),
				admin: { state: 'failed', message },
				release: { state: 'unknown' },
				recovery: { state: 'unknown' },
				llm: { state: 'unknown' },
				versions: {}
			};
		}
	}

	async function saveTargetModel(targetID: string) {
		const model = modelDraft(targetID).trim();
		if (!model) {
			modelErrors = { ...modelErrors, [targetID]: 'Model is required' };
			return;
		}
		modelSaving = { ...modelSaving, [targetID]: true };
		modelErrors = { ...modelErrors, [targetID]: '' };
		try {
			const llm = await updateTargetModel(targetID, model);
			modelDrafts = { ...modelDrafts, [targetID]: llm.model || model };
			mergeTargetModelStatus(targetID, llm);
		} catch (error) {
			modelErrors = { ...modelErrors, [targetID]: error instanceof Error ? error.message : 'failed to update model' };
		} finally {
			modelSaving = { ...modelSaving, [targetID]: false };
		}
	}

	function syncModelDrafts(nextStatuses: Record<string, TargetStatus>, shouldOverwrite: boolean) {
		const nextDrafts = { ...modelDrafts };
		for (const [targetID, status] of Object.entries(nextStatuses)) {
			const model = status.llm?.model ?? '';
			if (shouldOverwrite || !nextDrafts[targetID]) nextDrafts[targetID] = model;
		}
		modelDrafts = nextDrafts;
	}

	function mergeTargetModelStatus(targetID: string, llm: LLMModelStatus) {
		const currentStatus = statuses[targetID];
		if (!currentStatus) return;
		statuses = {
			...statuses,
			[targetID]: {
				...currentStatus,
				llm,
				checkedAt: new Date().toISOString()
			}
		};
	}

	function modelDraft(targetID: string, status?: TargetStatus) {
		return modelDrafts[targetID] ?? status?.llm?.model ?? '';
	}

	function updateModelDraft(targetID: string, event: Event) {
		if (!(event.currentTarget instanceof HTMLInputElement)) return;
		modelDrafts = { ...modelDrafts, [targetID]: event.currentTarget.value };
	}

	async function runTargetAction(targetID: string, action: string) {
		errorMessage = '';
		try {
			const job = await startJob(targetID, action);
			jobs = { ...jobs, [job.id]: job };
			selectedJobID = job.id;
			streamJobEvents(
				job.id,
				(event) => {
					const currentJob = jobs[job.id];
					jobs = {
						...jobs,
						[job.id]: {
							...currentJob,
							events: [...(currentJob?.events ?? []), event],
							state: event.message === 'completed' ? 'succeeded' : currentJob?.state ?? 'running'
						}
					};
					if (event.message === 'completed') void checkTarget(targetID);
				},
				(message) => {
					errorMessage = message;
				}
			);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'failed to start job';
		}
	}

	async function runLocalFleetAction(payload: LocalFleetJobRequest) {
		errorMessage = '';
		try {
			const job = await startLocalFleetJob(payload);
			jobs = { ...jobs, [job.id]: job };
			selectedJobID = job.id;
			streamJobEvents(
				job.id,
				(event) => {
					const currentJob = jobs[job.id];
					jobs = {
						...jobs,
						[job.id]: {
							...currentJob,
							events: [...(currentJob?.events ?? []), event],
							state: event.message === 'completed' ? 'succeeded' : currentJob?.state ?? 'running'
						}
					};
					if (event.message === 'completed') void refreshLocalFleetStatus();
				},
				(message) => {
					errorMessage = message;
				}
			);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'failed to start local fleet job';
		}
	}

	async function refreshLocalFleetStatus() {
		localFleetStatus = await readLocalFleetStatus();
	}

	function statusVariant(state: string) {
		return statusBadgeVariant(state);
	}

	function statusLabel(status?: { state?: string; code?: number }) {
		if (!status && isLoading) return 'Checking';
		return endpointStatusLabel(status);
	}

	function statusState(status?: { state?: string }) {
		if (!status && isLoading) return 'running';
		return status?.state ?? 'unknown';
	}

	function pendingTargetStatus(targetID: string): TargetStatus {
		const state = isLoading ? 'checking' : 'unknown';
		return {
			targetID,
			checkedAt: '',
			admin: { state },
			release: { state },
			recovery: { state },
			llm: { state },
			versions: {}
		};
	}

	function formatDate(value?: string) {
		if (!value) return '';
		return new Intl.DateTimeFormat('ko-KR', {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
			month: '2-digit',
			day: '2-digit'
		}).format(new Date(value));
	}

</script>

<svelte:head>
	<title>internkim Ops</title>
</svelte:head>

<main class="min-h-svh bg-background text-foreground">
	<datalist id="llm-model-presets">
		{#each modelPresets as model}
			<option value={model}></option>
		{/each}
	</datalist>
	<header class="border-b bg-muted/30">
		<div class="mx-auto flex max-w-[1500px] flex-col gap-3 px-4 py-4 lg:flex-row lg:items-center lg:justify-between">
			<div class="min-w-0">
				<h1 class="text-xl font-semibold tracking-normal">Personal Fleet Console</h1>
				<p class="mt-1 text-sm text-muted-foreground">127.0.0.1 only · signed HTTPS deploy first · SSH is recovery only</p>
			</div>
			<div class="flex flex-wrap items-center gap-2">
				<Button variant="outline" size="sm" onclick={loadTargets} disabled={isLoading}>
					<RefreshCwIcon />
					Refresh
				</Button>
				<Badge variant="outline">{targets.length} targets</Badge>
			</div>
		</div>
	</header>

	<div class="mx-auto grid max-w-[1500px] gap-4 px-4 py-4 xl:grid-cols-[minmax(0,1fr)_430px]">
		<section class="space-y-4">
			{#if errorMessage}
				<div class="rounded-md border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">{errorMessage}</div>
			{/if}

			<Card.Root class="overflow-hidden border-border/70 bg-card shadow-sm ring-1 ring-white/5">
				<Card.Header class="border-b bg-muted/20 px-4 py-3">
					<div class="flex items-start justify-between gap-3">
						<div class="min-w-0">
							<Card.Title class="flex items-center gap-2 text-base">
								<FlaskConicalIcon class="size-4 text-muted-foreground" />
								Local Fleet
							</Card.Title>
							<Card.Description class="mt-1 truncate">{localFleetStatus?.statePath ?? '.local/local-fleet'}</Card.Description>
						</div>
						<Badge variant={statusVariant(statusState(localFleetStatus?.virtualMachine))}>{statusLabel(localFleetStatus?.virtualMachine)}</Badge>
					</div>
				</Card.Header>
				<Card.Content class="space-y-3 p-4">
					<div class="grid gap-2 text-xs sm:grid-cols-2 xl:grid-cols-4">
						{@render StatusCell('VM', localFleetStatus?.virtualMachine)}
						{@render StatusCell('SSH', localFleetStatus?.ssh)}
						{@render StatusCell('Admin', localFleetStatus?.admin)}
						{@render StatusCell('Mattermost', localFleetStatus?.mattermost)}
					</div>
					<div class="grid gap-2 rounded-md border border-border/60 bg-background/40 px-3 py-2 text-xs text-muted-foreground sm:grid-cols-2">
						<a class="truncate text-primary underline-offset-4 hover:underline" href={localFleetStatus?.adminURL || undefined} target="_blank" rel="noreferrer">
							Admin: {localFleetStatus?.adminURL || 'not available'}
						</a>
						<a class="truncate text-primary underline-offset-4 hover:underline" href={localFleetStatus?.mattermostURL || undefined} target="_blank" rel="noreferrer">
							Mattermost: {localFleetStatus?.mattermostURL || 'not available'}
						</a>
					</div>
					<div class="grid gap-2 [grid-template-columns:repeat(auto-fit,minmax(148px,1fr))]">
						<Button variant="outline" size="sm" class="justify-start" onclick={() => runLocalFleetAction({ action: 'up' })}><ServerIcon /> Up</Button>
						<Button variant="outline" size="sm" class="justify-start" onclick={refreshLocalFleetStatus}><SearchCheckIcon /> Status</Button>
						<Button variant="default" size="sm" class="justify-start" onclick={() => runLocalFleetAction({ action: 'runRecipe', recipe: 'predeploy-gate' })}><ActivityIcon /> Predeploy gate</Button>
						<Button variant="secondary" size="sm" class="justify-start" onclick={() => runLocalFleetAction({ action: 'verifyRegression', base: 'main', scenario: 'regression-proof' })}><FlaskConicalIcon /> Verify regression</Button>
						<Button variant="outline" size="sm" class="justify-start" onclick={() => runLocalFleetAction({ action: 'reset' })}><RotateCwIcon /> Reset</Button>
						<Button variant="outline" size="sm" class="justify-start" onclick={() => runLocalFleetAction({ action: 'down' })}><RotateCwIcon /> Down</Button>
					</div>
					{#if localFleetStatus?.lastResult || localFleetStatus?.cleanupNeeded}
						<div class="text-xs text-muted-foreground">
							{localFleetStatus?.lastResult || 'No prior result'}{localFleetStatus?.cleanupNeeded ? ' · cleanup needed' : ''}
						</div>
					{/if}
				</Card.Content>
			</Card.Root>

			<div class="grid gap-3 2xl:grid-cols-2">
				{#each targets as target}
					{@const status = displayStatus(target.id)}
					{@const isStatusPending = targetStatus(target.id) === undefined && isLoading}
					<Card.Root class="overflow-hidden border-border/70 bg-card shadow-sm ring-1 ring-white/5">
						<Card.Header class="border-b bg-muted/20 px-4 py-3">
							<div class="flex items-start justify-between gap-3">
								<div class="min-w-0">
									<Card.Title class="flex items-center gap-2 text-base">
										<ServerIcon class="size-4 text-muted-foreground" />
										<button class="truncate text-left hover:underline" onclick={() => (selectedTargetID = target.id)}>{target.name}</button>
									</Card.Title>
									<Card.Description class="mt-1 truncate">{target.adminURL}</Card.Description>
								</div>
								<Badge variant={statusVariant(statusState(status?.admin))}>{statusLabel(status?.admin)}</Badge>
							</div>
						</Card.Header>
						<Card.Content class="space-y-3 p-4">
							<div class="grid gap-2 text-xs sm:grid-cols-2 xl:grid-cols-4">
								{@render StatusCell('Admin', status?.admin)}
								{@render StatusCell('Release', status?.release)}
								{@render StatusCell('Recovery', status?.recovery)}
							</div>
							<div class="grid gap-2 rounded-md border border-border/60 bg-background/40 px-3 py-2 text-xs text-muted-foreground sm:grid-cols-2">
								<div class="truncate">fleet: {target.fleetID || 'unnamed'}</div>
								<div class="truncate">ssh: {target.sshHostname || 'unnamed'}</div>
							</div>
							<div class="grid gap-2 rounded-md border border-border/70 bg-muted/10 p-3 text-xs [grid-template-columns:repeat(auto-fit,minmax(118px,1fr))]">
								{@render VersionCell('Admind', status.versions.admind, '', isStatusPending)}
								{@render VersionCell('Runtime', status.versions.runtime, runtimeVersionDetail(status.versions.current), isStatusPending)}
								{@render VersionCell('Web', status.versions.web, '', isStatusPending)}
								{@render ReleaseCell('Current', status.versions.current?.releaseID, isStatusPending)}
								{@render ReleaseCell('Latest', status.versions.latest?.releaseID, isStatusPending)}
							</div>
							{@render ModelEditor(target, status)}
							<div class="grid gap-2 [grid-template-columns:repeat(auto-fit,minmax(148px,1fr))]">
								{#each actions as action}
									{@const ActionIcon = action.icon}
									<Button
										variant={action.variant}
										size="sm"
										class="min-h-9 min-w-0 justify-start whitespace-normal px-2 text-left leading-tight"
										disabled={action.id === 'apply-release' && releaseActionDisabled(status)}
										onclick={() => runTargetAction(target.id, action.id)}
									>
										<ActionIcon class="shrink-0" />
										<span class="min-w-0 truncate">{action.label}</span>
									</Button>
								{/each}
							</div>
						</Card.Content>
					</Card.Root>
				{:else}
					<p class="text-sm text-muted-foreground">The vault names no device. Start the console as <code>./internkim @production ops serve</code>.</p>
				{/each}
			</div>
		</section>

		<aside class="space-y-4">
			<Card.Root>
				<Card.Header class="border-b">
					<Card.Title class="flex items-center gap-2 text-base"><TerminalIcon class="size-4" /> Job log</Card.Title>
					<Card.Description>{selectedJob()?.action ?? 'No job selected'}</Card.Description>
				</Card.Header>
				<Card.Content class="p-0">
					<div class="h-[520px] overflow-auto bg-zinc-950 p-3 font-mono text-xs text-zinc-100">
						{#if selectedJobEvents().length === 0}
							<div class="text-zinc-500">Run a target action to stream logs here.</div>
						{/if}
						{#each selectedJobEvents() as event}
							<div class="grid grid-cols-[70px_54px_minmax(0,1fr)] gap-2 border-b border-white/5 py-1">
								<span class="text-zinc-500">{formatDate(event.at)}</span>
								<span class={event.level === 'error' ? 'text-red-300' : 'text-emerald-300'}>{event.level}</span>
								<span class="break-words">{event.message}</span>
							</div>
						{/each}
					</div>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header class="border-b">
					<Card.Title class="flex items-center gap-2 text-base"><ActivityIcon class="size-4" /> Recent jobs</Card.Title>
				</Card.Header>
				<Card.Content class="p-0">
					<Table.Root>
						<Table.Header>
							<Table.Row>
								<Table.Head>Job</Table.Head>
								<Table.Head>Action</Table.Head>
								<Table.Head>State</Table.Head>
							</Table.Row>
						</Table.Header>
						<Table.Body>
							{#each Object.values(jobs).reverse() as job}
								<Table.Row class="cursor-pointer" onclick={() => (selectedJobID = job.id)}>
									<Table.Cell class="font-mono text-xs">{job.id}</Table.Cell>
									<Table.Cell>{job.action}</Table.Cell>
									<Table.Cell><Badge variant={statusVariant(job.state)}>{job.state}</Badge></Table.Cell>
								</Table.Row>
							{/each}
						</Table.Body>
					</Table.Root>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header class="border-b">
					<Card.Title class="flex items-center gap-2 text-base"><ShieldIcon class="size-4" /> Selected target</Card.Title>
				</Card.Header>
				<Card.Content class="space-y-2 p-3 text-sm">
					{@const target = selectedTarget()}
					{#if target}
						<div class="flex items-center justify-between gap-2">
							<span class="text-muted-foreground">Name</span>
							<span class="truncate font-medium">{target.name}</span>
						</div>
						<div class="flex items-center justify-between gap-2">
							<span class="text-muted-foreground">Admin</span>
							<a class="truncate text-primary underline-offset-4 hover:underline" href={target.adminURL} target="_blank" rel="noreferrer">{target.adminURL}</a>
						</div>
						<div class="flex items-center justify-between gap-2">
							<span class="text-muted-foreground">Last check</span>
							<span>{formatDate(displayStatus(target.id).checkedAt)}</span>
						</div>
					{/if}
				</Card.Content>
			</Card.Root>
		</aside>
	</div>
</main>

{#snippet StatusCell(label: string, status?: { state?: string; code?: number; message?: string })}
	<div class="min-w-0 rounded-md bg-muted/20 px-2.5 py-2 ring-1 ring-border/60">
		<div class="mb-1 truncate text-muted-foreground">{label}</div>
		<Badge variant={statusVariant(statusState(status))} class="max-w-full">{statusLabel(status)}</Badge>
		{#if status?.message}
			<div class="mt-1 truncate text-muted-foreground" title={status.message}>{status.message}</div>
		{/if}
	</div>
{/snippet}

{#snippet ModelEditor(target: OpsTarget, status?: TargetStatus)}
	<div class="rounded-md border border-border/70 bg-background/50 p-3">
		<div class="mb-2 flex items-center justify-between gap-2">
			<div class="flex min-w-0 items-center gap-2 text-xs font-medium">
				<BrainCircuitIcon class="size-4 text-muted-foreground" />
				<span>LLM model</span>
				<span class="truncate font-mono text-[11px] text-muted-foreground" title={status?.llm?.model || status?.llm?.message || ''}>{readableModelLabel(status?.llm)}</span>
			</div>
			<Badge variant={statusVariant(statusState(status?.llm))}>{statusLabel(status?.llm)}</Badge>
		</div>
		<div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_120px]">
			<Input
				class="font-mono text-xs"
				list="llm-model-presets"
				placeholder="Enter model id"
				value={modelDraft(target.id, status)}
				oninput={(event) => updateModelDraft(target.id, event)}
			/>
			<Button size="sm" variant="outline" class="justify-center" disabled={modelSaving[target.id] || !modelDraft(target.id, status).trim()} onclick={() => saveTargetModel(target.id)}>
				<SaveIcon />
				{modelSaving[target.id] ? 'Saving' : 'Save model'}
			</Button>
		</div>
		{#if modelErrors[target.id] || status?.llm?.message}
			<div class="mt-2 truncate text-xs text-muted-foreground" title={modelErrors[target.id] || status?.llm?.message}>
				{modelErrors[target.id] || status?.llm?.message}
			</div>
		{/if}
	</div>
{/snippet}

{#snippet VersionCell(label: string, value?: string, title?: string, isChecking = false)}
	<div class="min-w-0" title={title || value || ''}>
		<div class="text-muted-foreground">{label}</div>
		<div class={hasVersion(value) ? 'truncate font-mono text-[11px] font-semibold text-foreground' : 'truncate text-muted-foreground'}>
			{isChecking ? 'Checking' : shortVersion(value)}
		</div>
	</div>
{/snippet}

{#snippet ReleaseCell(label: string, value?: string, isChecking = false)}
	<div class="min-w-0" title={value || ''}>
		<div class="text-muted-foreground">{label}</div>
		<div class={hasVersion(value) ? 'truncate font-mono text-[11px] font-semibold text-foreground' : 'truncate text-muted-foreground'}>
			{isChecking ? 'Checking' : shortRelease(value)}
		</div>
	</div>
{/snippet}
