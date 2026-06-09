<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Badge } from '$lib/components/ui/badge';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import { onMount } from 'svelte';
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import CloudUploadIcon from '@lucide/svelte/icons/cloud-upload';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import SearchCheckIcon from '@lucide/svelte/icons/search-check';
	import ServerIcon from '@lucide/svelte/icons/server';
	import ShieldIcon from '@lucide/svelte/icons/shield';
	import TerminalIcon from '@lucide/svelte/icons/terminal';
	import { createTarget, fetchTargets, fetchTargetStatus, startJob, streamJobEvents } from './ops-api';
	import type { Job, NewTarget, OpsTarget, TargetStatus } from './ops-types';
	import { endpointStatusLabel, statusBadgeVariant } from './ops-view';

	const actions = [
		{ id: 'check', label: 'Check', icon: SearchCheckIcon, variant: 'outline' },
		{ id: 'deploy-admind', label: 'Deploy admind', icon: CloudUploadIcon, variant: 'default' },
		{ id: 'deploy-runtime', label: 'Deploy runtime', icon: CloudUploadIcon, variant: 'secondary' },
		{ id: 'deploy-web', label: 'Deploy web', icon: CloudUploadIcon, variant: 'secondary' },
		{ id: 'pilot-standard', label: 'Pilot standard', icon: ActivityIcon, variant: 'default' },
		{ id: 'restart-cloudflared-node-ssh', label: 'Restart tunnel', icon: RotateCwIcon, variant: 'outline' },
		{ id: 'restart-ssh', label: 'Restart SSH', icon: RotateCwIcon, variant: 'outline' },
		{ id: 'mattermost-smoke', label: 'Mattermost smoke', icon: ShieldIcon, variant: 'outline' }
	] as const;

	let targets = $state<OpsTarget[]>([]);
	let statuses = $state<Record<string, TargetStatus>>({});
	let jobs = $state<Record<string, Job>>({});
	let selectedJobID = $state('');
	let selectedTargetID = $state('');
	let isLoading = $state(false);
	let errorMessage = $state('');
	let newTarget = $state<NewTarget>({
		name: '',
		adminURL: '',
		profile: '',
		nodeArgument: '',
		secretSource: ''
	});

	const selectedJob = () => (selectedJobID ? jobs[selectedJobID] : undefined);
	const selectedJobEvents = () => selectedJob()?.events ?? [];
	const selectedTarget = () => targets.find((target) => target.id === selectedTargetID) ?? targets[0];
	const targetStatus = (targetID: string) => statuses[targetID];

	onMount(() => {
		void loadTargets();
	});

	async function loadTargets() {
		isLoading = true;
		errorMessage = '';
		try {
			targets = await fetchTargets();
			selectedTargetID = selectedTargetID || targets[0]?.id || '';
			await Promise.all(targets.map((target) => checkTarget(target.id)));
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'failed to load targets';
		} finally {
			isLoading = false;
		}
	}

	async function addTarget() {
		errorMessage = '';
		try {
			const target = await createTarget(newTarget);
			targets = await fetchTargets();
			selectedTargetID = target.id;
			newTarget = { name: '', adminURL: '', profile: '', nodeArgument: '', secretSource: '' };
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : 'failed to add target';
		}
	}

	async function checkTarget(targetID: string) {
		try {
			statuses = { ...statuses, [targetID]: await fetchTargetStatus(targetID) };
		} catch (error) {
			const message = error instanceof Error ? error.message : 'status check failed';
			statuses = {
				...statuses,
				[targetID]: {
					targetID,
					checkedAt: new Date().toISOString(),
					admin: { state: 'failed', message },
					mattermost: { state: 'unknown' },
					release: { state: 'unknown' },
					recovery: { state: 'unknown' }
				}
			};
		}
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

	function statusVariant(state: string) {
		return statusBadgeVariant(state);
	}

	function statusLabel(status?: { state?: string; code?: number }) {
		return endpointStatusLabel(status);
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
	<title>InternKim Ops</title>
</svelte:head>

<main class="min-h-svh bg-background text-foreground">
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

			<div class="grid gap-3 lg:grid-cols-2">
				{#each targets as target}
					{@const status = targetStatus(target.id)}
					<Card.Root class="overflow-hidden border-border/80">
						<Card.Header class="border-b bg-card/80 pb-3">
							<div class="flex items-start justify-between gap-3">
								<div class="min-w-0">
									<Card.Title class="flex items-center gap-2 text-base">
										<ServerIcon class="size-4 text-muted-foreground" />
										<button class="truncate text-left hover:underline" onclick={() => (selectedTargetID = target.id)}>{target.name}</button>
									</Card.Title>
									<Card.Description class="mt-1 truncate">{target.adminURL}</Card.Description>
								</div>
								<Badge variant={statusVariant(status?.admin.state ?? 'unknown')}>{statusLabel(status?.admin)}</Badge>
							</div>
						</Card.Header>
						<Card.Content class="space-y-3 p-3">
							<div class="grid grid-cols-2 gap-2 text-xs md:grid-cols-4">
								{@render StatusCell('Admin', status?.admin)}
								{@render StatusCell('Mattermost', status?.mattermost)}
								{@render StatusCell('Release', status?.release)}
								{@render StatusCell('Recovery', status?.recovery)}
							</div>
							<div class="grid gap-2 text-xs text-muted-foreground md:grid-cols-3">
								<div class="truncate">profile: {target.profile || 'default'}</div>
								<div class="truncate">node: {target.nodeID || target.nodeArgument || 'default'}</div>
								<div class="truncate">secret: {target.secretSource ? 'local reference' : 'state'}</div>
							</div>
							<div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
								{#each actions as action}
									{@const ActionIcon = action.icon}
									<Button
										variant={action.variant}
										size="sm"
										class="justify-start"
										onclick={() => runTargetAction(target.id, action.id)}
									>
										<ActionIcon />
										{action.label}
									</Button>
								{/each}
							</div>
						</Card.Content>
					</Card.Root>
				{/each}
			</div>

			<Card.Root>
				<Card.Header class="border-b">
					<Card.Title class="flex items-center gap-2 text-base"><PlusIcon class="size-4" /> Add target</Card.Title>
					<Card.Description>Secret values stay in local files; this form stores references and routing metadata only.</Card.Description>
				</Card.Header>
				<Card.Content class="grid gap-3 p-3 md:grid-cols-5">
					<Input placeholder="name" bind:value={newTarget.name} />
					<Input class="md:col-span-2" placeholder="https://pilot-01.intern.kim" bind:value={newTarget.adminURL} />
					<Input placeholder="profile" bind:value={newTarget.profile} />
					<Input placeholder="node" bind:value={newTarget.nodeArgument} />
					<Input class="md:col-span-4" placeholder="secret source path" bind:value={newTarget.secretSource} />
					<Button onclick={addTarget} disabled={!newTarget.adminURL}>
						<PlusIcon />
						Add
					</Button>
				</Card.Content>
			</Card.Root>
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
							<span>{formatDate(targetStatus(target.id)?.checkedAt)}</span>
						</div>
					{/if}
				</Card.Content>
			</Card.Root>
		</aside>
	</div>
</main>

{#snippet StatusCell(label: string, status?: { state?: string; code?: number; message?: string })}
	<div class="rounded-md border bg-muted/20 p-2">
		<div class="mb-1 text-muted-foreground">{label}</div>
		<Badge variant={statusVariant(status?.state ?? 'unknown')}>{statusLabel(status)}</Badge>
		{#if status?.message}
			<div class="mt-1 truncate text-muted-foreground" title={status.message}>{status.message}</div>
		{/if}
	</div>
{/snippet}
