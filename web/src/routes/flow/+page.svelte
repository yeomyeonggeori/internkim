<script lang="ts">
	import Identicon from '$lib/components/identicon.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { ConfirmDeleteDialog, confirmDelete } from '$lib/components/ui/confirm-delete-dialog';
	import { FlexRender, createSvelteTable, renderSnippet } from '$lib/components/ui/data-table';
	import { Input } from '$lib/components/ui/input';
	import { Meter } from '$lib/components/ui/meter';
	import * as Select from '$lib/components/ui/select';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import * as Table from '$lib/components/ui/table';
	import { TagsInput } from '$lib/components/ui/tags-input';
	import { Textarea } from '$lib/components/ui/textarea';
	import { cn } from '$lib/utils';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import ArrowDownIcon from '@lucide/svelte/icons/arrow-down';
	import ArrowUpDownIcon from '@lucide/svelte/icons/arrow-up-down';
	import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import SearchIcon from '@lucide/svelte/icons/search';
	import SparklesIcon from '@lucide/svelte/icons/sparkles';
	import Trash2Icon from '@lucide/svelte/icons/trash-2';
	import {
		type ColumnDef,
		type PaginationState,
		type SortingState,
		getCoreRowModel,
		getPaginationRowModel,
		getSortedRowModel
	} from '@tanstack/table-core';
	import { onMount } from 'svelte';
	import { flowText } from './text';

	type FlowWeek = {
		code: string;
		startISO: string;
		endISO: string;
		previous: string;
		next: string;
		isCurrent: boolean;
	};

	type FlowMember = {
		id: string;
		name: string;
		email: string;
		role: string;
		mattermostStatus: string;
		score: number;
		activeTaskCount: number;
		completeTaskCount: number;
	};

	type FlowTask = {
		id: string;
		ownerID: string;
		ownerName: string;
		participantIDs: string[];
		participantNames: string[];
		business: string;
		type: string;
		content: string;
		goal: string;
		size: string;
		status: string;
		startDate?: string;
		endDate?: string;
		weekCode: string;
		flag: number;
		requestReason?: string;
		decisionReason?: string;
	};

	type FlowMetrics = {
		totalTasks: number;
		completedTasks: number;
		requestedTasks: number;
		pausedTasks: number;
		stoppedTasks: number;
		totalScore: number;
		statusCounts: Record<string, number>;
		businessCounts: Record<string, number>;
		typeCounts: Record<string, number>;
		memberScores: Record<string, number>;
	};

	type FlowSizeDefinition = {
		name: string;
		distanceKm: number;
		maxHours: number;
		developmentExample: string;
		otherExample: string;
		note: string;
		score: number;
		label: string;
	};

	type FlowDefinitions = {
		categories: string[];
		types: string[];
		sizes: FlowSizeDefinition[];
	};

	type FlowSummary = {
		week: FlowWeek;
		members: FlowMember[];
		tasks: FlowTask[];
		metrics: FlowMetrics;
		definitions: FlowDefinitions;
		statusOptions: string[];
		currentUserEmail: string;
		currentUserName: string;
		isAdmin: boolean;
		source: string;
	};

	const emptyMetrics: FlowMetrics = {
		totalTasks: 0,
		completedTasks: 0,
		requestedTasks: 0,
		pausedTasks: 0,
		stoppedTasks: 0,
		totalScore: 0,
		statusCounts: {},
		businessCounts: {},
		typeCounts: {},
		memberScores: {}
	};

	let summary = $state<FlowSummary | null>(null);
	let activeTab = $state('report');
	let selectedTask = $state<FlowTask | null>(null);
	let taskDraft = $state<FlowTask | null>(null);
	let searchText = $state('');
	let statusFilter = $state('all');
	let ownerFilter = $state('all');
	let businessFilter = $state('all');
	let typeFilter = $state('all');
	let isLoading = $state(false);
	let isSavingTask = $state(false);
	let errorMessage = $state('');
	let taskErrorMessage = $state('');
	let quickTaskText = $state('');
	let isCreatingQuickTask = $state(false);
	let categoryDrafts = $state<string[]>([]);
	let typeDrafts = $state<string[]>([]);
	let sizeDrafts = $state<FlowSizeDefinition[]>([]);
	let newCategoryText = $state('');
	let newTypeText = $state('');
	let isSavingDefinitions = $state(false);
	let definitionErrorMessage = $state('');

	const currentWeek = () => summary?.week.code ?? '';
	const members = () => summary?.members ?? [];
	const tasks = () => summary?.tasks ?? [];
	const metrics = () => summary?.metrics ?? emptyMetrics;
	const definitions = () =>
		summary?.definitions ?? {
			categories: [],
			types: [],
			sizes: []
		};
	const statusOptions = () => summary?.statusOptions ?? [];
	const isMemberTab = () => activeTab.startsWith('member:');
	const text = createPageText(flowText);
	const categoryOptions = () => definitions().categories.map((category) => ({ value: category, label: category }));
	const typeOptions = () => definitions().types.map((type) => ({ value: type, label: type }));
	const sizeOptions = () =>
		definitions().sizes.map((size) => ({ value: size.name, label: `${size.name} · ${size.distanceKm}km · ${size.maxHours}h` }));
	const statusFilterOptions = () => [
		{ value: 'all', label: text.filters.all },
		...statusOptions().map((status) => ({ value: status, label: statusLabel(status) }))
	];
	const memberFilterOptions = () => [
		{ value: 'all', label: text.filters.all },
		...members().map((member) => ({ value: member.id, label: member.name }))
	];
	const categoryFilterOptions = () => [{ value: 'all', label: text.filters.all }, ...categoryOptions()];
	const typeFilterOptions = () => [{ value: 'all', label: text.filters.all }, ...typeOptions()];
	const statusSelectOptions = () => statusOptions().map((status) => ({ value: status, label: statusLabel(status) }));
	const memberSelectOptions = () => members().map((member) => ({ value: member.id, label: member.name }));
	const hasFlowData = () => summary !== null;
	const canEditDefinitions = () => hasFlowData() && definitions().sizes.length > 0;

	const filteredTasks = () => {
		const normalizedSearch = searchText.trim().toLowerCase();
		return tasks().filter((task) => {
			if (statusFilter !== 'all' && task.status !== statusFilter) return false;
			if (ownerFilter !== 'all' && !task.participantIDs.includes(ownerFilter)) return false;
			if (businessFilter !== 'all' && task.business !== businessFilter) return false;
			if (typeFilter !== 'all' && task.type !== typeFilter) return false;
			if (isMemberTab() && !task.participantIDs.includes(activeTab.replace('member:', ''))) return false;
			if (!normalizedSearch) return true;
			return [task.content, task.goal, task.ownerName, task.business, task.type]
				.join(' ')
				.toLowerCase()
				.includes(normalizedSearch);
		});
	};

	onMount(() => {
		const week = new URLSearchParams(location.search).get('week') ?? '';
		loadFlow(week);
	});

	async function loadFlow(week: string) {
		isLoading = true;
		errorMessage = '';
		try {
			const query = week ? `?week=${encodeURIComponent(week)}` : '';
			const response = await fetch(`/flow/api/summary${query}`, { credentials: 'include' });
			if (!response.ok) throw new Error(await responseErrorMessage(response, text.loadError));
			summary = (await response.json()) as FlowSummary;
			syncDefinitionDrafts();
			if (isMemberTab()) {
				const memberID = activeTab.replace('member:', '');
				if (!members().some((member) => member.id === memberID)) activeTab = 'tasks';
			}
			if (summary.week.code) replaceWeekQuery(summary.week.code);
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.loadError;
			summary = null;
			activeTab = 'report';
			syncDefinitionDrafts();
		} finally {
			isLoading = false;
		}
	}

	function replaceWeekQuery(week: string) {
		const url = new URL(location.href);
		url.searchParams.set('week', week);
		history.replaceState({}, '', url);
	}

	function openTask(task: FlowTask) {
		selectedTask = task;
		taskDraft = cloneTask(task);
		taskErrorMessage = '';
	}

	function createTask() {
		const owner = defaultTaskOwner();
		if (!owner || !summary) return;
		const task: FlowTask = {
			id: '',
			ownerID: owner.id,
			ownerName: owner.name,
			participantIDs: [owner.id],
			participantNames: [owner.name],
			business: definitions().categories[0] ?? '',
			type: definitions().types[0] ?? '기타',
			content: '',
			goal: '',
			size: 'M',
			status: '예정',
			weekCode: summary.week.code,
			flag: 0
		};
		selectedTask = task;
		taskDraft = cloneTask(task);
		taskErrorMessage = '';
	}

	async function createQuickTask() {
		const prompt = quickTaskText.trim();
		const owner = defaultTaskOwner();
		if (!prompt || !owner || !summary) return;
		isCreatingQuickTask = true;
		taskErrorMessage = '';
		try {
			const response = await fetch('/flow/api/tasks/quick', {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					prompt,
					ownerID: owner.id,
					participantIDs: [owner.id],
					weekCode: summary.week.code
				})
			});
			if (!response.ok) throw new Error(await responseErrorMessage(response, text.task.quickAddError));
			quickTaskText = '';
			await loadFlow(currentWeek());
		} catch (error) {
			taskErrorMessage = error instanceof Error ? error.message : text.task.quickAddError;
		} finally {
			isCreatingQuickTask = false;
		}
	}

	function defaultTaskOwner() {
		if (isMemberTab()) {
			const memberID = activeTab.replace('member:', '');
			const member = members().find((candidate) => candidate.id === memberID);
			if (member) return member;
		}
		const currentUserEmail = summary?.currentUserEmail ?? '';
		return members().find((member) => member.email === currentUserEmail) ?? members()[0];
	}

	function cloneTask(task: FlowTask): FlowTask {
		return {
			...task,
			participantIDs: [...task.participantIDs],
			participantNames: [...task.participantNames]
		};
	}

	async function saveTask() {
		if (!taskDraft) return;
		isSavingTask = true;
		taskErrorMessage = '';
		try {
			const method = taskDraft.id ? 'PUT' : 'POST';
			const path = taskDraft.id ? `/flow/api/tasks/${encodeURIComponent(taskDraft.id)}` : '/flow/api/tasks';
			const response = await fetch(path, {
				method,
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(taskDraft)
			});
			if (!response.ok) throw new Error(await responseErrorMessage(response, text.task.saveError));
			selectedTask = null;
			taskDraft = null;
			await loadFlow(currentWeek());
		} catch (error) {
			taskErrorMessage = error instanceof Error ? error.message : text.task.saveError;
		} finally {
			isSavingTask = false;
		}
	}

	let pendingStatusTaskID = $state('');
	let taskSorting = $state<SortingState>([]);
	let taskPagination = $state<PaginationState>({ pageIndex: 0, pageSize: 20 });

	async function updateTaskStatus(task: FlowTask, nextStatus: string) {
		if (!task.id || nextStatus === task.status) return;
		pendingStatusTaskID = task.id;
		errorMessage = '';
		try {
			const response = await fetch(`/flow/api/tasks/${encodeURIComponent(task.id)}`, {
				method: 'PUT',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ ...task, status: nextStatus })
			});
			if (!response.ok) throw new Error(await responseErrorMessage(response, text.task.saveError));
			await loadFlow(currentWeek());
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : text.task.saveError;
		} finally {
			pendingStatusTaskID = '';
		}
	}

	function selectCurrentWeek() {
		loadFlow('');
	}

	function selectWeek(week: string) {
		loadFlow(week);
	}

	function resetFilters() {
		searchText = '';
		statusFilter = 'all';
		ownerFilter = 'all';
		businessFilter = 'all';
		typeFilter = 'all';
	}

	function syncDefinitionDrafts() {
		categoryDrafts = [...definitions().categories];
		typeDrafts = [...definitions().types];
		sizeDrafts = definitions().sizes.map((size) => ({ ...size }));
	}

	function addCategory() {
		const value = newCategoryText.trim();
		if (!value || categoryDrafts.includes(value)) return;
		categoryDrafts = [...categoryDrafts, value];
		newCategoryText = '';
	}

	function addType() {
		const value = newTypeText.trim();
		if (!value || typeDrafts.includes(value)) return;
		typeDrafts = [...typeDrafts, value];
		newTypeText = '';
	}

	function updateCategory(index: number, value: string) {
		categoryDrafts = categoryDrafts.map((item, itemIndex) => (itemIndex === index ? value : item));
	}

	function updateType(index: number, value: string) {
		typeDrafts = typeDrafts.map((item, itemIndex) => (itemIndex === index ? value : item));
	}

	function removeCategory(index: number) {
		categoryDrafts = categoryDrafts.filter((_item, itemIndex) => itemIndex !== index);
	}

	function removeType(index: number) {
		typeDrafts = typeDrafts.filter((_item, itemIndex) => itemIndex !== index);
	}

	async function saveDefinitions() {
		if (!summary?.isAdmin || !canEditDefinitions()) return;
		isSavingDefinitions = true;
		definitionErrorMessage = '';
		try {
			const sizes = sizeDrafts.map((size) => ({
				...size,
				distanceKm: Math.max(1, Number(size.distanceKm) || 1),
				maxHours: Math.max(1, Number(size.maxHours) || 1),
				score: Math.max(1, Number(size.distanceKm) || 1),
				label: `${Math.max(1, Number(size.distanceKm) || 1)}km · ${Math.max(1, Number(size.maxHours) || 1)}h`
			}));
			const response = await fetch('/flow/api/definitions', {
				method: 'PUT',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					categories: categoryDrafts,
					types: typeDrafts,
					sizes
				})
			});
			if (!response.ok) throw new Error(await responseErrorMessage(response, text.definitions.saveError));
			await loadFlow(currentWeek());
		} catch (error) {
			definitionErrorMessage = error instanceof Error ? error.message : text.definitions.saveError;
		} finally {
			isSavingDefinitions = false;
		}
	}

	function confirmRemoveCategory(index: number) {
		const value = categoryDrafts[index];
		confirmDelete({
			title: text.definitions.removeCategoryTitle,
			description: text.definitions.removeCategoryDescription.replace('{value}', value),
			confirm: { text: text.definitions.removeAction },
			cancel: { text: text.definitions.cancel },
			onConfirm: async () => {
				removeCategory(index);
			}
		});
	}

	function confirmRemoveType(index: number) {
		const value = typeDrafts[index];
		confirmDelete({
			title: text.definitions.removeTypeTitle,
			description: text.definitions.removeTypeDescription.replace('{value}', value),
			confirm: { text: text.definitions.removeAction },
			cancel: { text: text.definitions.cancel },
			onConfirm: async () => {
				removeType(index);
			}
		});
	}

	function setParticipantNames(names: string[]) {
		if (!taskDraft) return;
		const owner = members().find((member) => member.id === taskDraft?.ownerID);
		const memberByName = new Map(members().map((member) => [member.name, member]));
		const ordered: FlowMember[] = [];
		if (owner) ordered.push(owner);
		for (const name of names) {
			const candidate = memberByName.get(name);
			if (candidate && !ordered.some((member) => member.id === candidate.id)) {
				ordered.push(candidate);
			}
		}
		taskDraft.participantIDs = ordered.map((member) => member.id);
		taskDraft.participantNames = ordered.map((member) => member.name);
	}


	function statusBadgeClass(status: string) {
		switch (status) {
			case '완료':
				return 'bg-[#d4edbc] text-[#1f3826] border-transparent';
			case '진행':
				return 'bg-[#bfe1f6] text-[#0b3d63] border-transparent';
			case '예정':
				return 'bg-[#ffe5a0] text-[#473821] border-transparent';
			case '요청':
				return 'bg-[#e6cff2] text-[#3d1c52] border-transparent';
			case '일시정지':
				return 'bg-[#ffcfc9] text-[#5b1c14] border-transparent';
			case '기각':
			case '중단':
				return 'bg-[#f6c1bd] text-[#5b1c14] border-transparent';
			default:
				return 'bg-muted text-muted-foreground border-transparent';
		}
	}

	function sizeBadgeClass(size: string) {
		switch (size) {
			case 'XS':
				return 'bg-[#f1f3f4] text-[#3c4043] border-transparent';
			case 'S':
				return 'bg-[#d4edbc] text-[#1f3826] border-transparent';
			case 'M':
				return 'bg-[#bfe1f6] text-[#0b3d63] border-transparent';
			case 'L':
				return 'bg-[#ffe5a0] text-[#473821] border-transparent';
			case 'XL':
				return 'bg-[#ffcfc9] text-[#5b1c14] border-transparent';
			case 'XXL':
				return 'bg-[#f6c1bd] text-[#5b1c14] border-transparent';
			default:
				return 'bg-muted text-muted-foreground border-transparent';
		}
	}

	function sortedEntries(values: Record<string, number>) {
		return Object.entries(values).sort((left, right) => right[1] - left[1]);
	}

	function maxValue(values: Record<string, number>) {
		return Math.max(1, ...Object.values(values));
	}

	function statusLabel(status: string) {
		const labels = text.status as Record<string, string>;
		return labels[status] ?? status;
	}

	function compareDate(left: string | undefined, right: string | undefined) {
		const leftValue = left ?? '';
		const rightValue = right ?? '';
		if (leftValue === rightValue) return 0;
		if (!leftValue) return 1;
		if (!rightValue) return -1;
		return leftValue < rightValue ? -1 : 1;
	}

	const taskColumns: ColumnDef<FlowTask>[] = [
		{
			accessorKey: 'ownerName',
			header: () => renderSnippet(taskHeader, { label: text.table.owner, id: 'ownerName' }),
			cell: (info) => renderSnippet(taskOwnerCell, { task: info.row.original })
		},
		{
			accessorKey: 'business',
			header: () => renderSnippet(taskHeader, { label: text.table.business, id: 'business' }),
			cell: (info) => renderSnippet(taskTextCell, { value: info.row.original.business || '-', muted: true })
		},
		{
			accessorKey: 'type',
			header: () => renderSnippet(taskHeader, { label: text.table.type, id: 'type' }),
			cell: (info) => renderSnippet(taskTextCell, { value: info.row.original.type, muted: false })
		},
		{
			accessorKey: 'content',
			enableSorting: false,
			header: () => renderSnippet(taskHeader, { label: text.table.content, id: 'content' }),
			cell: (info) => renderSnippet(taskContentCell, { value: info.row.original.content })
		},
		{
			id: 'participants',
			enableSorting: false,
			header: () => renderSnippet(taskHeader, { label: text.table.participants, id: 'participants' }),
			cell: (info) => renderSnippet(taskParticipantsCell, { task: info.row.original })
		},
		{
			accessorKey: 'size',
			header: () => renderSnippet(taskHeader, { label: text.table.size, id: 'size' }),
			cell: (info) => renderSnippet(taskSizeCell, { task: info.row.original })
		},
		{
			accessorKey: 'status',
			header: () => renderSnippet(taskHeader, { label: text.table.status, id: 'status' }),
			cell: (info) => renderSnippet(taskStatusCell, { task: info.row.original })
		},
		{
			accessorKey: 'startDate',
			header: () => renderSnippet(taskHeader, { label: text.table.startDate, id: 'startDate' }),
			cell: (info) => renderSnippet(taskTextCell, { value: info.row.original.startDate || '-', muted: true }),
			sortingFn: (left, right) => compareDate(left.original.startDate, right.original.startDate)
		},
		{
			accessorKey: 'endDate',
			header: () => renderSnippet(taskHeader, { label: text.table.endDate, id: 'endDate' }),
			cell: (info) => renderSnippet(taskTextCell, { value: info.row.original.endDate || '-', muted: true }),
			sortingFn: (left, right) => compareDate(left.original.endDate, right.original.endDate)
		},
		{
			accessorKey: 'flag',
			header: () => renderSnippet(taskHeader, { label: text.table.flag, id: 'flag', align: 'right' }),
			cell: (info) => renderSnippet(taskNumberCell, { value: info.row.original.flag })
		}
	];

	const taskTable = createSvelteTable<FlowTask>({
		get data() {
			return filteredTasks();
		},
		columns: taskColumns,
		getCoreRowModel: getCoreRowModel(),
		getSortedRowModel: getSortedRowModel(),
		getPaginationRowModel: getPaginationRowModel(),
		state: {
			get sorting() {
				return taskSorting;
			},
			get pagination() {
				return taskPagination;
			}
		},
		onSortingChange: (updater) => {
			taskSorting = typeof updater === 'function' ? updater(taskSorting) : updater;
		},
		onPaginationChange: (updater) => {
			taskPagination = typeof updater === 'function' ? updater(taskPagination) : updater;
		}
	});

	async function responseErrorMessage(response: Response, fallback: string) {
		const message = (await response.text()).trim();
		if (!message || message.startsWith('<!doctype html>') || message.startsWith('<html')) return fallback;
		return message;
	}
</script>

<svelte:head>
	<title>{text.pageTitle}</title>
</svelte:head>

<main class="min-h-screen bg-background text-foreground">
	<div class="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-6 md:px-8">
		<header class="flex flex-col gap-4 border-b pb-5 md:flex-row md:items-end md:justify-between">
			<div class="space-y-1">
				<p class="text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.product}</p>
				<h1 class="text-2xl font-semibold">{text.title}</h1>
				<p class="text-sm text-muted-foreground">{text.description}</p>
			</div>
			<div class="flex flex-wrap items-center gap-2">
				<Button variant="outline" size="sm" onclick={() => selectWeek(summary?.week.previous ?? '')} disabled={!summary || isLoading}>
					<ChevronLeftIcon />
					{text.previousWeek}
				</Button>
				<div class="rounded-lg border bg-card px-3 py-1.5 text-sm font-medium tabular-nums">
					{summary?.week.code || (summary?.week.startISO ? `${summary.week.startISO} – ${summary.week.endISO}` : '...')}
				</div>
				<Button variant="outline" size="sm" onclick={() => selectWeek(summary?.week.next ?? '')} disabled={!summary || isLoading}>
					{text.nextWeek}
					<ChevronRightIcon />
				</Button>
				<Button variant="secondary" size="sm" onclick={selectCurrentWeek} disabled={isLoading}>
					{text.currentWeek}
				</Button>
				<Button variant="ghost" size="icon-sm" aria-label={text.refresh} onclick={() => loadFlow(currentWeek())} disabled={isLoading}>
					<RefreshCwIcon class={isLoading ? 'animate-spin' : ''} />
				</Button>
			</div>
		</header>

		{#if errorMessage}
			<div class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
				{errorMessage}
			</div>
		{/if}

		<section class="grid gap-3 md:grid-cols-4">
			{@render MetricCard(text.metrics.total, metrics().totalTasks, `${summary?.week.startISO ?? ''} – ${summary?.week.endISO ?? ''}`)}
			{@render MetricCard(text.metrics.completed, metrics().completedTasks, `${metrics().totalScore} km`)}
			{@render MetricCard(text.metrics.requested, metrics().requestedTasks, text.metrics.requestedDescription)}
			{@render MetricCard(text.metrics.blocked, metrics().pausedTasks + metrics().stoppedTasks, text.metrics.blockedDescription)}
		</section>

		<div class="flex w-full items-center gap-1 overflow-x-auto border-b">
			{@render TabButton('report', text.tabs.report)}
			{@render TabButton('tasks', text.tabs.tasks)}
			{#each members() as member}
				{@render MemberTabButton(member)}
			{/each}
			{@render TabButton('definitions', text.tabs.definitions, !canEditDefinitions())}
			{@render TabButton('members', text.tabs.members)}
		</div>

		{#if activeTab === 'report'}
			<section class="grid gap-4 lg:grid-cols-2">
				{@render ReportCard(text.report.weeklyStatus, text.report.weeklyStatusDescription, sortedEntries(metrics().statusCounts).map(([key, value]) => [statusLabel(key), value] as [string, number]), maxValue(metrics().statusCounts))}
				{@render ReportCard(text.report.memberScores, text.report.memberScoresDescription, sortedEntries(metrics().memberScores), maxValue(metrics().memberScores))}
				{@render ReportCard(text.report.businessDistance, '', sortedEntries(metrics().businessCounts), maxValue(metrics().businessCounts))}
				{@render ReportCard(text.report.typeBreakdown, '', sortedEntries(metrics().typeCounts), maxValue(metrics().typeCounts))}
			</section>
		{:else if activeTab === 'definitions'}
			{#if canEditDefinitions()}
				<section class="grid gap-4">
					{@render SizeDefinitionCard()}
					<div class="grid gap-4 lg:grid-cols-2">
						{@render EditableListCard(
							text.definitions.category,
							text.definitions.categoryDescription,
							categoryDrafts,
							newCategoryText,
							updateCategory,
							confirmRemoveCategory,
							addCategory,
							(value: string) => (newCategoryText = value)
						)}
						{@render EditableListCard(
							text.definitions.type,
							text.definitions.typeDescription,
							typeDrafts,
							newTypeText,
							updateType,
							confirmRemoveType,
							addType,
							(value: string) => (newTypeText = value)
						)}
					</div>
					{#if summary?.isAdmin}
						<div class="flex flex-wrap items-center justify-between gap-3 rounded-lg border bg-muted/30 p-3">
							<p class="text-sm text-muted-foreground">{definitionErrorMessage || text.definitions.adminOnly}</p>
							<Button onclick={saveDefinitions} disabled={isSavingDefinitions || typeDrafts.filter((value) => value.trim()).length === 0}>
								{isSavingDefinitions ? text.definitions.saving : text.definitions.save}
							</Button>
						</div>
					{:else}
						<p class="text-sm text-muted-foreground">{text.definitions.adminOnly}</p>
					{/if}
				</section>
			{:else}
				<div class="rounded-lg border bg-muted/30 p-4 text-sm text-muted-foreground">
					{errorMessage || text.loadError}
				</div>
			{/if}
		{:else if activeTab === 'members'}
			{@render MembersCard()}
		{:else}
			{@render TasksSection()}
		{/if}
	</div>
</main>

<Sheet.Root open={selectedTask !== null} onOpenChange={(open) => {
	if (!open) {
		selectedTask = null;
		taskDraft = null;
	}
}}>
	<Sheet.Content class="w-full overflow-y-auto sm:max-w-xl">
		<Sheet.Header>
			<Sheet.Title>{taskDraft?.id ? text.task.editTitle : text.task.createTitle}</Sheet.Title>
			<Sheet.Description>{taskDraft?.weekCode} · {text.title}</Sheet.Description>
		</Sheet.Header>
		{#if taskDraft}
			<div class="space-y-4 px-4 pb-6">
				<div class="flex flex-wrap gap-2">
					<Badge class={statusBadgeClass(taskDraft.status)}>{statusLabel(taskDraft.status)}</Badge>
					<Badge class={sizeBadgeClass(taskDraft.size)}>{taskDraft.size}</Badge>
					{#if taskDraft.business}
						<Badge variant="outline">{taskDraft.business}</Badge>
					{/if}
					<Badge variant="outline">{taskDraft.type}</Badge>
				</div>
				<label class="grid gap-1 text-xs font-medium text-muted-foreground">
					{text.task.content}
					<Input bind:value={taskDraft.content} placeholder={text.task.contentPlaceholder} />
				</label>
				<label class="grid gap-1 text-xs font-medium text-muted-foreground">
					{text.task.goal}
					<Input bind:value={taskDraft.goal} placeholder={text.task.goalPlaceholder} />
				</label>
				<div class="grid gap-3 md:grid-cols-2">
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.task.owner}
						<Select.Root type="single" bind:value={taskDraft.ownerID}>
							<Select.Trigger class="w-full">
								{memberSelectOptions().find((option) => option.value === taskDraft?.ownerID)?.label ?? '-'}
							</Select.Trigger>
							<Select.Content>
								{#each memberSelectOptions() as option (option.value)}
									<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.task.status}
						<Select.Root type="single" bind:value={taskDraft.status}>
							<Select.Trigger class="w-full">
								{statusLabel(taskDraft.status)}
							</Select.Trigger>
							<Select.Content>
								{#each statusSelectOptions() as option (option.value)}
									<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</label>
					{#if definitions().categories.length > 1}
						<label class="grid gap-1 text-xs font-medium text-muted-foreground">
							{text.task.category}
							<Select.Root type="single" bind:value={taskDraft.business}>
								<Select.Trigger class="w-full">
									{taskDraft.business || '-'}
								</Select.Trigger>
								<Select.Content>
									{#each categoryOptions() as option (option.value)}
										<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</label>
					{/if}
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.task.type}
						<Select.Root type="single" bind:value={taskDraft.type}>
							<Select.Trigger class="w-full">
								{taskDraft.type || '-'}
							</Select.Trigger>
							<Select.Content>
								{#each typeOptions() as option (option.value)}
									<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.task.size}
						<Select.Root type="single" bind:value={taskDraft.size}>
							<Select.Trigger class="w-full">
								{sizeOptions().find((option) => option.value === taskDraft?.size)?.label ?? '-'}
							</Select.Trigger>
							<Select.Content>
								{#each sizeOptions() as option (option.value)}
									<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.task.flag}
						<Input type="number" min={0} step={1} bind:value={taskDraft.flag} />
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.task.startDate}
						<Input type="date" bind:value={taskDraft.startDate} />
					</label>
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.task.endDate}
						<Input type="date" bind:value={taskDraft.endDate} />
					</label>
				</div>
				<div class="space-y-2">
					<div class="text-xs font-medium text-muted-foreground">{text.task.participants}</div>
					<TagsInput
						value={taskDraft.participantNames}
						suggestions={members().map((member) => member.name)}
						restrictToSuggestions
						placeholder={text.task.participantsPlaceholder}
						onValueChange={setParticipantNames}
					/>
				</div>
				{#if taskDraft.status === '요청'}
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.task.requestReason}
						<Input bind:value={taskDraft.requestReason} />
					</label>
				{/if}
				{#if taskDraft.status === '기각' || taskDraft.status === '중단'}
					<label class="grid gap-1 text-xs font-medium text-muted-foreground">
						{text.task.reason}
						<Input bind:value={taskDraft.decisionReason} />
					</label>
				{/if}
				<Separator />
				<div class="rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">
					{text.task.dateRule}
				</div>
				{#if taskErrorMessage}
					<p class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{taskErrorMessage}</p>
				{/if}
				<Sheet.Footer>
					<Button onclick={saveTask} disabled={isSavingTask || !taskDraft.content.trim()}>
						{isSavingTask ? text.task.saving : text.task.save}
					</Button>
				</Sheet.Footer>
			</div>
		{/if}
	</Sheet.Content>
</Sheet.Root>

<ConfirmDeleteDialog />

{#snippet TabButton(value: string, label: string, disabled = false)}
	<button
		type="button"
		class={cn(
			'relative h-9 whitespace-nowrap px-3 text-sm font-medium transition-colors',
			activeTab === value
				? 'text-foreground after:absolute after:inset-x-2 after:-bottom-px after:h-0.5 after:bg-primary'
				: 'text-muted-foreground hover:text-foreground',
			disabled && 'cursor-not-allowed opacity-50 hover:text-muted-foreground'
		)}
		{disabled}
		onclick={() => {
			if (disabled) return;
			activeTab = value;
		}}
	>
		{label}
	</button>
{/snippet}

{#snippet MemberTabButton(member: FlowMember)}
	<button
		type="button"
		class={cn(
			'relative inline-flex h-9 items-center gap-2 whitespace-nowrap px-3 text-sm font-medium transition-colors',
			activeTab === `member:${member.id}`
				? 'text-foreground after:absolute after:inset-x-2 after:-bottom-px after:h-0.5 after:bg-primary'
				: 'text-muted-foreground hover:text-foreground'
		)}
		onclick={() => (activeTab = `member:${member.id}`)}
	>
		<Identicon seed={member.email || member.name} class="size-5" />
		{member.name}
	</button>
{/snippet}

{#snippet MetricCard(label: string, value: number, subvalue: string)}
	<Card.Root size="sm">
		<Card.Header>
			<Card.Description class="text-xs uppercase tracking-wide">{label}</Card.Description>
		</Card.Header>
		<Card.Content>
			<div class="text-3xl font-semibold leading-none">{value}</div>
			<div class="mt-2 text-xs text-muted-foreground">{subvalue}</div>
		</Card.Content>
	</Card.Root>
{/snippet}

{#snippet ReportCard(title: string, description: string, entries: [string, number][], max: number)}
	<Card.Root>
		<Card.Header>
			<Card.Title>{title}</Card.Title>
			{#if description}
				<Card.Description>{description}</Card.Description>
			{/if}
		</Card.Header>
		<Card.Content class="space-y-3">
			{#each entries as [label, value]}
				<div class="grid grid-cols-[8rem_1fr_3rem] items-center gap-3 text-sm">
					<div class="truncate text-muted-foreground">{label}</div>
					<Meter {value} max={max} />
					<div class="text-right font-medium tabular-nums">{value}</div>
				</div>
			{/each}
			{#if entries.length === 0}
				<p class="text-sm text-muted-foreground">—</p>
			{/if}
		</Card.Content>
	</Card.Root>
{/snippet}

{#snippet SelectControl(label: string, options: { value: string; label: string }[], value: string, onchange: (value: string) => void)}
	<label class="grid gap-1 text-xs font-medium text-muted-foreground">
		{label}
		<Select.Root type="single" {value} onValueChange={onchange}>
			<Select.Trigger class="w-full">
				{options.find((option) => option.value === value)?.label ?? '-'}
			</Select.Trigger>
			<Select.Content>
				{#each options as option (option.value)}
					<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</label>
{/snippet}

{#snippet TasksSection()}
	<div class="space-y-4">
		<Card.Root size="sm">
			<Card.Content class="space-y-3">
				<div class="grid gap-2 md:grid-cols-[1fr_auto] md:items-stretch">
					<div>
						<Textarea
							class="min-h-16 resize-none"
							placeholder={text.task.quickAddPlaceholder}
							bind:value={quickTaskText}
						/>
						<p class="mt-1 text-xs text-muted-foreground">{text.task.quickAddHint}</p>
					</div>
					<Button
						class="md:h-auto md:min-h-16 md:px-5"
						onclick={createQuickTask}
						disabled={isCreatingQuickTask || !quickTaskText.trim() || members().length === 0}
					>
						<SparklesIcon />
						{isCreatingQuickTask ? text.task.saving : text.task.quickAdd}
					</Button>
				</div>
				{#if taskErrorMessage}
					<p class="rounded-lg border border-destructive/30 bg-destructive/10 p-2 text-sm text-destructive">{taskErrorMessage}</p>
				{/if}
			</Card.Content>
		</Card.Root>

		<div class="flex flex-wrap items-end gap-2 rounded-lg border bg-card p-3">
			<div class="relative grow basis-56">
				<SearchIcon class="absolute left-2 top-2.5 size-4 text-muted-foreground" />
				<Input class="pl-8" placeholder={text.filters.searchPlaceholder} bind:value={searchText} />
			</div>
			<div class="grow basis-36">
				{@render SelectControl(text.filters.status, statusFilterOptions(), statusFilter, (value: string) => (statusFilter = value))}
			</div>
			<div class="grow basis-36">
				{@render SelectControl(text.filters.owner, memberFilterOptions(), ownerFilter, (value: string) => (ownerFilter = value))}
			</div>
			{#if definitions().categories.length > 1}
				<div class="grow basis-36">
					{@render SelectControl(text.filters.business, categoryFilterOptions(), businessFilter, (value: string) => (businessFilter = value))}
				</div>
			{/if}
			<div class="grow basis-36">
				{@render SelectControl(text.filters.type, typeFilterOptions(), typeFilter, (value: string) => (typeFilter = value))}
			</div>
			<Button variant="ghost" size="sm" onclick={resetFilters}>{text.filters.reset}</Button>
			<Button size="sm" onclick={createTask} disabled={members().length === 0}>
				<PlusIcon />
				{text.filters.addTask}
			</Button>
		</div>

		{@render TaskTable()}
	</div>
{/snippet}

{#snippet taskHeader({ label, id, align }: { label: string; id: string; align?: 'left' | 'right' })}
	{@const column = taskTable.getColumn(id)}
	{@const sortDirection = column?.getIsSorted()}
	{@const canSort = column?.getCanSort()}
	<div class={cn('flex items-center gap-1', align === 'right' && 'justify-end')}>
		{#if canSort}
			<button
				type="button"
				class="-mx-1 inline-flex items-center gap-1 rounded px-1 py-0.5 text-xs font-medium uppercase tracking-wide text-muted-foreground hover:bg-muted hover:text-foreground"
				onclick={() => column?.toggleSorting(sortDirection === 'asc')}
			>
				{label}
				{#if sortDirection === 'asc'}
					<ArrowUpIcon class="size-3" />
				{:else if sortDirection === 'desc'}
					<ArrowDownIcon class="size-3" />
				{:else}
					<ArrowUpDownIcon class="size-3 opacity-40" />
				{/if}
			</button>
		{:else}
			<span class="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</span>
		{/if}
	</div>
{/snippet}

{#snippet taskOwnerCell({ task }: { task: FlowTask })}
	<div class="flex items-center gap-2 font-medium">
		<Identicon seed={task.ownerID || task.ownerName} class="size-6" />
		{task.ownerName}
	</div>
{/snippet}

{#snippet taskTextCell({ value, muted }: { value: string; muted: boolean })}
	<span class={muted ? 'text-muted-foreground' : ''}>{value}</span>
{/snippet}

{#snippet taskContentCell({ value }: { value: string })}
	<span class="block max-w-[26rem] truncate">{value}</span>
{/snippet}

{#snippet taskParticipantsCell({ task }: { task: FlowTask })}
	<div class="flex max-w-56 flex-wrap gap-1">
		{#each task.participantNames as name}
			<Badge variant="outline">{name}</Badge>
		{/each}
	</div>
{/snippet}

{#snippet taskSizeCell({ task }: { task: FlowTask })}
	<Badge class={sizeBadgeClass(task.size)}>{task.size}</Badge>
{/snippet}

{#snippet taskStatusCell({ task }: { task: FlowTask })}
	<div onclick={(event) => event.stopPropagation()} role="presentation">
		<Select.Root
			type="single"
			value={task.status}
			disabled={pendingStatusTaskID === task.id}
			onValueChange={(next) => updateTaskStatus(task, next)}
		>
			<Select.Trigger
				size="sm"
				class={cn('w-28 justify-between border-transparent font-medium', statusBadgeClass(task.status))}
			>
				{statusLabel(task.status)}
			</Select.Trigger>
			<Select.Content>
				{#each statusSelectOptions() as option (option.value)}
					<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>
{/snippet}

{#snippet taskNumberCell({ value }: { value: number })}
	<span class="block text-right tabular-nums">{value}</span>
{/snippet}

{#snippet TaskTable()}
	{@const rowModel = taskTable.getRowModel()}
	{@const totalRows = taskTable.getFilteredRowModel().rows.length}
	{@const pageCount = taskTable.getPageCount()}
	{@const pageIndex = taskTable.getState().pagination.pageIndex}
	<div class="space-y-3">
		<div class="overflow-hidden rounded-lg border bg-card">
			<Table.Root class="min-w-[1080px]">
				<Table.Header class="bg-muted/40">
					{#each taskTable.getHeaderGroups() as headerGroup (headerGroup.id)}
						<Table.Row class="hover:bg-transparent">
							{#each headerGroup.headers as header (header.id)}
								<Table.Head class="h-10">
									{#if !header.isPlaceholder}
										<FlexRender content={header.column.columnDef.header} context={header.getContext()} />
									{/if}
								</Table.Head>
							{/each}
						</Table.Row>
					{/each}
				</Table.Header>
				<Table.Body>
					{#each rowModel.rows as row (row.id)}
						<Table.Row class="cursor-pointer hover:bg-muted/40" onclick={() => openTask(row.original)}>
							{#each row.getVisibleCells() as cell (cell.id)}
								<Table.Cell>
									<FlexRender content={cell.column.columnDef.cell} context={cell.getContext()} />
								</Table.Cell>
							{/each}
						</Table.Row>
					{/each}
					{#if rowModel.rows.length === 0}
						<Table.Row class="hover:bg-transparent">
							<Table.Cell colspan={taskColumns.length} class="py-10 text-center text-muted-foreground">{text.task.empty}</Table.Cell>
						</Table.Row>
					{/if}
				</Table.Body>
			</Table.Root>
		</div>
		{#if totalRows > 0}
			<div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
				<div>
					{text.table.pagination.summary
						.replace('{from}', String(totalRows === 0 ? 0 : pageIndex * taskPagination.pageSize + 1))
						.replace('{to}', String(Math.min(totalRows, (pageIndex + 1) * taskPagination.pageSize)))
						.replace('{total}', String(totalRows))}
				</div>
				<div class="flex items-center gap-1">
					<Button variant="outline" size="sm" onclick={() => taskTable.previousPage()} disabled={!taskTable.getCanPreviousPage()}>
						{text.table.pagination.previous}
					</Button>
					<span class="px-2 tabular-nums">{pageIndex + 1} / {Math.max(1, pageCount)}</span>
					<Button variant="outline" size="sm" onclick={() => taskTable.nextPage()} disabled={!taskTable.getCanNextPage()}>
						{text.table.pagination.next}
					</Button>
				</div>
			</div>
		{/if}
	</div>
{/snippet}

{#snippet SizeDefinitionCard()}
	<Card.Root>
		<Card.Header>
			<Card.Title>{text.definitions.size}</Card.Title>
			<Card.Description>{text.definitions.sizeDescription}</Card.Description>
		</Card.Header>
		<Card.Content>
			<div class="overflow-hidden rounded-lg border">
				<Table.Root class="min-w-[980px]">
					<Table.Header class="bg-muted/40">
						<Table.Row class="hover:bg-transparent">
							<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.definitions.sizeName}</Table.Head>
							<Table.Head class="h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.definitions.distance}</Table.Head>
							<Table.Head class="h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.definitions.maxHours}</Table.Head>
							<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.definitions.developmentExample}</Table.Head>
							<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.definitions.otherExample}</Table.Head>
							<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.definitions.note}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each definitions().sizes as size (size.name)}
							<Table.Row>
								<Table.Cell><Badge class={sizeBadgeClass(size.name)}>{size.name}</Badge></Table.Cell>
								<Table.Cell class="text-right tabular-nums">{size.distanceKm}</Table.Cell>
								<Table.Cell class="text-right tabular-nums">{size.maxHours}</Table.Cell>
								<Table.Cell>{size.developmentExample}</Table.Cell>
								<Table.Cell>{size.otherExample}</Table.Cell>
								<Table.Cell class="text-muted-foreground">{size.note}</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</div>
		</Card.Content>
	</Card.Root>
{/snippet}

{#snippet EditableListCard(
	title: string,
	description: string,
	items: string[],
	newValue: string,
	update: (index: number, value: string) => void,
	remove: (index: number) => void,
	add: () => void,
	setNewValue: (value: string) => void
)}
	<Card.Root size="sm">
		<Card.Header>
			<Card.Title>{title}</Card.Title>
			<Card.Description>{description}</Card.Description>
		</Card.Header>
		<Card.Content class="space-y-2">
			{#each items as item, index}
				<div class="grid grid-cols-[1fr_auto] gap-2">
					<Input
						value={item}
						disabled={!summary?.isAdmin}
						oninput={(event) => update(index, event.currentTarget.value)}
					/>
					<Button
						variant="ghost"
						size="icon"
						disabled={!summary?.isAdmin}
						onclick={() => remove(index)}
						aria-label={text.definitions.removeAction}
					>
						<Trash2Icon class="size-4" />
					</Button>
				</div>
			{/each}
			{#if summary?.isAdmin}
				<div class="grid grid-cols-[1fr_auto] gap-2">
					<Input value={newValue} placeholder={title} oninput={(event) => setNewValue(event.currentTarget.value)} />
					<Button variant="outline" size="icon" onclick={add} aria-label={text.definitions.add}>
						<PlusIcon class="size-4" />
					</Button>
				</div>
			{/if}
			{#if items.length === 0 && !summary?.isAdmin}
				<p class="rounded-lg bg-muted/40 p-3 text-sm text-muted-foreground">—</p>
			{/if}
		</Card.Content>
	</Card.Root>
{/snippet}

{#snippet MembersCard()}
	<Card.Root>
		<Card.Header>
			<Card.Title>{text.members.title}</Card.Title>
		</Card.Header>
		<Card.Content>
			<div class="overflow-hidden rounded-lg border">
				<Table.Root class="min-w-[720px]">
					<Table.Header class="bg-muted/40">
						<Table.Row class="hover:bg-transparent">
							<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.members.name}</Table.Head>
							<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.members.email}</Table.Head>
							<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.members.role}</Table.Head>
							<Table.Head class="h-10 text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.members.mattermost}</Table.Head>
							<Table.Head class="h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.members.active}</Table.Head>
							<Table.Head class="h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.members.completed}</Table.Head>
							<Table.Head class="h-10 text-right text-xs font-medium uppercase tracking-wide text-muted-foreground">{text.members.score}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each members() as member}
							<Table.Row>
								<Table.Cell class="font-medium">
									<div class="flex items-center gap-2">
										<Identicon seed={member.email || member.name} class="size-7" />
										{member.name}
									</div>
								</Table.Cell>
								<Table.Cell class="text-muted-foreground">{member.email || '-'}</Table.Cell>
								<Table.Cell>
									<Badge variant={member.role === 'admin' ? 'secondary' : 'outline'}>{member.role}</Badge>
								</Table.Cell>
								<Table.Cell>{member.mattermostStatus}</Table.Cell>
								<Table.Cell class="text-right tabular-nums">{member.activeTaskCount}</Table.Cell>
								<Table.Cell class="text-right tabular-nums">{member.completeTaskCount}</Table.Cell>
								<Table.Cell class="text-right font-medium tabular-nums">{member.score}</Table.Cell>
							</Table.Row>
						{/each}
						{#if members().length === 0}
							<Table.Row class="hover:bg-transparent">
								<Table.Cell colspan={7} class="py-10 text-center text-muted-foreground">{text.members.empty}</Table.Cell>
							</Table.Row>
						{/if}
					</Table.Body>
				</Table.Root>
			</div>
		</Card.Content>
	</Card.Root>
{/snippet}
