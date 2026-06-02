<script lang="ts">
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Separator } from '$lib/components/ui/separator';
	import * as Sheet from '$lib/components/ui/sheet';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { breadcrumbMeta } from '$lib/stores/breadcrumb-meta.svelte';
	import CalendarIcon from '@lucide/svelte/icons/calendar-days';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import RotateCwIcon from '@lucide/svelte/icons/rotate-cw';
	import { Popover } from 'bits-ui';
	import { onMount } from 'svelte';
	import { bumpCalendarRefresh, calendarVisibility } from './refresh-signal.svelte';
	import { calendarText } from './text';

	type CalendarSyncResponse = {
		caldavURL: string;
		caldavUsername: string;
		caldavPassword: string;
		icsURL: string;
	};

	type CalendarAccountStatusResponse = {
		connected: boolean;
		provider?: string;
		accountEmail?: string;
		lastAuthError?: string;
		needsReauth: boolean;
	};

	type CalendarEvent = {
		id?: string;
		title?: string;
		startISO: string;
		endISO: string;
		isAllDay: boolean;
	};

	type CalendarEventsResponse = {
		events?: CalendarEvent[];
	};

	type CalendarSource = {
		id: string;
		label: string;
		count: string;
		isChecked: boolean;
		colorClass: string;
	};

	let { children } = $props();

	const text = createPageText(calendarText);
	const isEmbed = $derived(page.url.pathname.startsWith('/calendar/embed'));
	const localeCode = $derived(currentLocale.value === 'ko' ? 'ko-KR' : 'en-US');

	let syncInformation = $state<CalendarSyncResponse | null>(null);
	let accountStatus = $state<CalendarAccountStatusResponse | null>(null);
	let accountStatusError = $state(false);
	let isLoadingAccountStatus = $state(false);
	let isSyncSheetOpen = $state(false);
	let isRotatingSync = $state(false);
	let syncError = $state('');
	let miniMonthEventDates = $state<Set<string>>(new Set());
	let miniMonthEventCount = $state(0);

	const today = new Date();
	let miniMonth = $state(new Date(today.getFullYear(), today.getMonth(), 1));

	const todayMonthLabel = $derived(
		today.toLocaleDateString(localeCode, {
			year: 'numeric',
			month: 'long'
		})
	);

	const miniMonthLabel = $derived(miniMonth.toLocaleDateString(localeCode, { year: 'numeric', month: 'long' }));

	function shiftMiniMonth(delta: number) {
		miniMonth = new Date(miniMonth.getFullYear(), miniMonth.getMonth() + delta, 1);
	}

	function isSameDay(a: Date, b: Date) {
		return a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
	}

	let isMonthPickerOpen = $state(false);
	let pickerMode = $state<'month' | 'year'>('month');
	let pickerYear = $state(today.getFullYear());
	let pickerYearWindowStart = $state(Math.floor(today.getFullYear() / 12) * 12);

	function handlePickerOpenChange(open: boolean) {
		isMonthPickerOpen = open;
		if (open) {
			pickerMode = 'month';
			pickerYear = miniMonth.getFullYear();
			pickerYearWindowStart = Math.floor(pickerYear / 12) * 12;
		}
	}

	function selectPickerMonth(monthIndex: number) {
		miniMonth = new Date(pickerYear, monthIndex, 1);
		isMonthPickerOpen = false;
	}

	function selectPickerYear(year: number) {
		pickerYear = year;
		pickerMode = 'month';
	}

	const monthShortLabels = $derived(
		Array.from({ length: 12 }, (_, monthIndex) => new Date(2000, monthIndex, 1).toLocaleDateString(localeCode, { month: 'short' }))
	);

	$effect(() => {
		if (isEmbed) return;
		breadcrumbMeta.value = todayMonthLabel;
		return () => {
			breadcrumbMeta.value = '';
		};
	});

	$effect(() => {
		if (isEmbed) return;
		miniMonth;
		loadMiniMonthEvents();
	});

	const calendarSources = $derived<CalendarSource[]>([
		{
			id: 'work',
			label: text.work,
			count: String(miniMonthEventCount),
			isChecked: calendarVisibility.work,
			colorClass: 'border-[#4a8dde] bg-[#4a8dde] text-white'
		}
	]);

	onMount(() => {
		if (isEmbed) return;
		loadSyncInformation();
		loadAccountStatus();
		const savedVisibility = window.localStorage.getItem('internkim.calendar.workVisible');
		if (savedVisibility) calendarVisibility.work = savedVisibility === 'true';
	});

	function toggleCalendarSource(sourceID: string) {
		if (sourceID !== 'work') return;
		calendarVisibility.work = !calendarVisibility.work;
		window.localStorage.setItem('internkim.calendar.workVisible', String(calendarVisibility.work));
		const channel = new BroadcastChannel('internkim-calendar');
		channel.postMessage({ type: 'calendar-visibility', work: calendarVisibility.work });
		channel.close();
	}

	async function loadSyncInformation() {
		try {
			const response = await fetch('/calendar/api/sync', { credentials: 'include' });
			if (!response.ok) return;
			syncInformation = (await response.json()) as CalendarSyncResponse;
		} catch {
			syncInformation = null;
		}
	}

	async function loadAccountStatus() {
		isLoadingAccountStatus = true;
		accountStatusError = false;
		try {
			const response = await fetch('/calendar/api/account-status', { credentials: 'include' });
			if (!response.ok) throw new Error('account status failed');
			accountStatus = (await response.json()) as CalendarAccountStatusResponse;
		} catch {
			accountStatus = null;
			accountStatusError = true;
		} finally {
			isLoadingAccountStatus = false;
		}
	}

	async function loadMiniMonthEvents() {
		try {
			const startDate = new Date(miniMonth.getFullYear(), miniMonth.getMonth(), 1);
			const endDate = new Date(miniMonth.getFullYear(), miniMonth.getMonth() + 1, 1);
			const query = new URLSearchParams({
				startISO: startDate.toISOString(),
				endISO: endDate.toISOString()
			});
			const response = await fetch(`/calendar/api/events?${query}`, { credentials: 'include' });
			if (!response.ok) {
				miniMonthEventDates = new Set();
				miniMonthEventCount = 0;
				return;
			}
			const payload = (await response.json()) as CalendarEventsResponse;
			const events = payload.events ?? [];
			miniMonthEventDates = eventDateKeysForMonth(events, miniMonth);
			miniMonthEventCount = events.length;
		} catch {
			miniMonthEventDates = new Set();
			miniMonthEventCount = 0;
		}
	}

	async function rotateSubscriptionURL() {
		isRotatingSync = true;
		syncError = '';
		try {
			const response = await fetch('/calendar/api/ics-token', {
				method: 'POST',
				credentials: 'include'
			});
			if (!response.ok) throw new Error(text.saveError);
			syncInformation = (await response.json()) as CalendarSyncResponse;
		} catch (error) {
			syncError = error instanceof Error ? error.message : text.saveError;
		} finally {
			isRotatingSync = false;
		}
	}

	function openSyncSheet() {
		syncError = '';
		isSyncSheetOpen = true;
		loadSyncInformation();
		loadAccountStatus();
	}

	function accountStatusLabel() {
		if (isLoadingAccountStatus) return text.accountStatusLoading;
		if (accountStatusError) return text.accountStatusLoadFailed;
		if (!accountStatus?.connected) return text.googleCalendarDisconnected;
		if (accountStatus.needsReauth) return text.googleCalendarReauthRequired;
		if (accountStatus.accountEmail) {
			return text.googleCalendarConnectedTemplate.replace('{email}', accountStatus.accountEmail);
		}
		return text.googleCalendarConnected;
	}

	function accountStatusDotClass() {
		if (accountStatusError || accountStatus?.needsReauth) return 'bg-warning';
		if (accountStatus?.connected) return 'bg-success';
		return 'bg-muted-foreground/50';
	}

	const weekdayLabels = $derived(
		Array.from({ length: 7 }, (_, weekdayIndex) => new Date(2026, 4, 24 + weekdayIndex).toLocaleDateString(localeCode, { weekday: 'narrow' }))
	);

	type MiniMonthCell = { date: Date; isOther: boolean; isToday: boolean };

	function eventDateKeysForMonth(events: CalendarEvent[], monthDate: Date): Set<string> {
		const monthStart = new Date(monthDate.getFullYear(), monthDate.getMonth(), 1);
		const monthEnd = new Date(monthDate.getFullYear(), monthDate.getMonth() + 1, 0);
		const keys = new Set<string>();
		for (const event of events) {
			const startDate = localDateFromISO(event.startISO);
			const rawEndDate = localDateFromISO(event.endISO);
			const endDate = event.isAllDay
				? new Date(rawEndDate.getFullYear(), rawEndDate.getMonth(), rawEndDate.getDate() - 1)
				: rawEndDate;
			const cursor = maxDate(startDate, monthStart);
			const lastDate = minDate(endDate, monthEnd);
			while (cursor <= lastDate) {
				keys.add(dateKey(cursor));
				cursor.setDate(cursor.getDate() + 1);
			}
		}
		return keys;
	}

	function localDateFromISO(value: string): Date {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return new Date(0);
		return new Date(date.getFullYear(), date.getMonth(), date.getDate());
	}

	function maxDate(left: Date, right: Date): Date {
		return new Date(Math.max(left.getTime(), right.getTime()));
	}

	function minDate(left: Date, right: Date): Date {
		return new Date(Math.min(left.getTime(), right.getTime()));
	}

	function dateKey(date: Date): string {
		return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
	}

	function miniMonthCells(): MiniMonthCell[] {
		const year = miniMonth.getFullYear();
		const month = miniMonth.getMonth();
		const firstWeekday = new Date(year, month, 1).getDay();
		const cells: MiniMonthCell[] = [];
		for (let i = firstWeekday; i > 0; i -= 1) {
			const date = new Date(year, month, 1 - i);
			cells.push({ date, isOther: true, isToday: isSameDay(date, today) });
		}
		const daysInMonth = new Date(year, month + 1, 0).getDate();
		for (let d = 1; d <= daysInMonth; d += 1) {
			const date = new Date(year, month, d);
			cells.push({ date, isOther: false, isToday: isSameDay(date, today) });
		}
		let trailing = 1;
		while (cells.length < 42) {
			const date = new Date(year, month + 1, trailing);
			cells.push({ date, isOther: true, isToday: isSameDay(date, today) });
			trailing += 1;
		}
		return cells;
	}
</script>

{#if isEmbed}
	{@render children()}
{:else}
	<aside class="flex w-60 shrink-0 flex-col border-r bg-background max-md:hidden">
		<div class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
			<CalendarIcon class="size-5 text-foreground" />
			<div class="min-w-0 flex-1">
				<p class="truncate text-[15px] font-semibold tracking-tight">{text.title}</p>
			</div>
			<Button variant="ghost" size="icon-sm" aria-label={text.moreOptions} class="size-8">•••</Button>
		</div>

		<div class="min-h-0 flex-1 overflow-auto">
			<section class="space-y-3 px-5 py-5">
				<header class="flex items-center justify-between px-1">
					<p class="text-[12px] font-bold uppercase tracking-[0.14em] text-muted-foreground">{text.myCalendars}</p>
					<button type="button" aria-label={text.addCalendar} class="flex size-5 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-accent-foreground">
						<PlusIcon class="size-3.5" />
					</button>
				</header>
				<div class="space-y-2">
					{#each calendarSources as source (source.id)}
						<div class="flex h-9 w-full items-center gap-3 rounded-md text-left text-[15px]">
							<button
								type="button"
								aria-pressed={source.isChecked}
								aria-label={`${source.label} ${text.calendarVisibility}`}
								class="flex size-5 shrink-0 items-center justify-center rounded-md border-2 {source.isChecked
									? source.colorClass
									: 'border-[#a9c89e] bg-transparent text-transparent'}"
								onclick={() => toggleCalendarSource(source.id)}
							>
								<span class="text-[15px] font-bold leading-none {source.isChecked ? '' : 'opacity-0'}">✓</span>
							</button>
							<span class="min-w-0 flex-1 truncate text-foreground">{source.label}</span>
							{#if source.count}
								<span class="text-sm tabular-nums text-muted-foreground">{source.count}</span>
							{/if}
						</div>
					{/each}
				</div>
			</section>
		</div>

		<section class="shrink-0 border-t px-5 pb-3 pt-4">
			<header class="mb-2 flex items-center gap-1">
				<button
					type="button"
					aria-label={text.previousMonth}
					class="flex size-7 items-center justify-center rounded text-foreground hover:bg-accent"
					onclick={() => shiftMiniMonth(-1)}
				>
					<ChevronLeftIcon class="size-3.5" />
				</button>
				<Popover.Root open={isMonthPickerOpen} onOpenChange={handlePickerOpenChange}>
					<Popover.Trigger
						class="flex h-7 flex-1 items-center justify-center rounded text-center text-[14px] font-bold transition-colors hover:bg-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
						aria-label={text.pickMonthAndYear}
					>
						{miniMonthLabel}
					</Popover.Trigger>
					<Popover.Portal>
						<Popover.Content
							side="bottom"
							align="center"
							sideOffset={6}
							class="z-50 w-56 rounded-md border border-border/50 bg-popover p-2 text-popover-foreground shadow-sm outline-none data-open:animate-in data-closed:animate-out data-open:fade-in-0 data-closed:fade-out-0 data-open:zoom-in-95 data-closed:zoom-out-95"
						>
							<header class="mb-1.5 flex items-center gap-1">
								<button
									type="button"
									aria-label={pickerMode === 'month' ? text.previousYear : text.previousTwelveYears}
									class="flex size-6 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground"
									onclick={() => {
										if (pickerMode === 'month') pickerYear -= 1;
										else pickerYearWindowStart -= 12;
									}}
								>
									<ChevronLeftIcon class="size-3.5" />
								</button>
								<button
									type="button"
									class="flex-1 rounded py-1 text-xs font-semibold tabular-nums hover:bg-accent"
									onclick={() => {
										if (pickerMode === 'month') {
											pickerYearWindowStart = Math.floor(pickerYear / 12) * 12;
											pickerMode = 'year';
										} else {
											pickerMode = 'month';
										}
									}}
								>
									{#if pickerMode === 'month'}
										{pickerYear}
									{:else}
										{pickerYearWindowStart}–{pickerYearWindowStart + 11}
									{/if}
								</button>
								<button
									type="button"
									aria-label={pickerMode === 'month' ? text.nextYear : text.nextTwelveYears}
									class="flex size-6 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground"
									onclick={() => {
										if (pickerMode === 'month') pickerYear += 1;
										else pickerYearWindowStart += 12;
									}}
								>
									<ChevronRightIcon class="size-3.5" />
								</button>
							</header>

							{#if pickerMode === 'month'}
								<div class="grid grid-cols-3 gap-1">
									{#each monthShortLabels as label, i (i)}
										<button
											type="button"
											class="flex h-8 items-center justify-center rounded text-xs tabular-nums transition-colors {pickerYear === miniMonth.getFullYear() && i === miniMonth.getMonth()
												? 'bg-primary font-semibold text-primary-foreground'
												: pickerYear === today.getFullYear() && i === today.getMonth()
													? 'font-semibold text-primary hover:bg-accent'
													: 'text-foreground hover:bg-accent'}"
											onclick={() => selectPickerMonth(i)}
										>
											{label}
										</button>
									{/each}
								</div>
							{:else}
								<div class="grid grid-cols-3 gap-1">
									{#each Array.from({ length: 12 }, (_, i) => pickerYearWindowStart + i) as year (year)}
										<button
											type="button"
											class="flex h-8 items-center justify-center rounded text-xs tabular-nums transition-colors {year === pickerYear
												? 'bg-primary font-semibold text-primary-foreground'
												: year === today.getFullYear()
													? 'font-semibold text-primary hover:bg-accent'
													: 'text-foreground hover:bg-accent'}"
											onclick={() => selectPickerYear(year)}
										>
											{year}
										</button>
									{/each}
								</div>
							{/if}
						</Popover.Content>
					</Popover.Portal>
				</Popover.Root>
				<button
					type="button"
					aria-label={text.nextMonth}
					class="flex size-7 items-center justify-center rounded text-foreground hover:bg-accent"
					onclick={() => shiftMiniMonth(1)}
				>
					<ChevronRightIcon class="size-3.5" />
				</button>
			</header>
			<div class="grid grid-cols-7 gap-y-0.5">
				{#each weekdayLabels as wd, index (index)}
					<span class="py-0.5 text-center text-[11px] font-bold text-muted-foreground">
						{wd}
					</span>
				{/each}
				{#each miniMonthCells() as cell, index (index)}
					{@const isWeekend = cell.date.getDay() === 0 || cell.date.getDay() === 6}
					{@const hasEvent = miniMonthEventDates.has(dateKey(cell.date))}
					<span
						class="relative mx-auto flex size-7 items-start justify-center rounded-md pt-0.5 text-[12px] tabular-nums {cell.isToday
							? 'bg-primary font-bold text-primary-foreground'
							: cell.isOther
								? 'text-muted-foreground opacity-45'
								: isWeekend
									? 'text-foreground hover:bg-accent'
									: 'text-foreground hover:bg-accent'}"
					>
						{cell.date.getDate()}
						{#if hasEvent}
							<span
								class="absolute bottom-0.5 left-1/2 size-1.5 -translate-x-1/2 rounded-full {cell.isToday ? 'bg-primary-foreground' : 'bg-primary'}"
								aria-hidden="true"
							></span>
						{/if}
					</span>
				{/each}
			</div>
		</section>

		<div class="flex shrink-0 items-center gap-1 border-t px-2 py-2 text-xs">
			<button
				type="button"
				class="flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-accent"
				onclick={openSyncSheet}
			>
				<span class="size-1.5 rounded-full bg-muted-foreground/50" aria-hidden="true"></span>
				<span class="min-w-0 flex-1 truncate text-muted-foreground">{text.subscriptionSettings}</span>
			</button>
			<Button variant="ghost" size="icon-sm" aria-label={text.refresh} class="size-7" onclick={bumpCalendarRefresh}>
				<RefreshCwIcon class="size-3.5" />
			</Button>
		</div>
	</aside>

	<div class="flex min-w-0 flex-1 flex-col">
		{@render children()}
	</div>

	<Sheet.Root bind:open={isSyncSheetOpen}>
		<Sheet.Content class="w-full overflow-y-auto sm:max-w-md">
			<Sheet.Header>
				<Sheet.Title>{text.syncTitle}</Sheet.Title>
				<Sheet.Description>{text.syncDescription}</Sheet.Description>
			</Sheet.Header>

			<div class="grid gap-5 px-4 pb-4">
				{#if syncError}
					<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{syncError}</p>
				{/if}

				<div class="space-y-2 rounded-md border p-3">
					<p class="text-xs font-medium uppercase text-muted-foreground">{text.externalCalendarAccount}</p>
					<div class="flex items-center gap-2 text-sm">
						<span class={`size-2 rounded-full ${accountStatusDotClass()}`} aria-hidden="true"></span>
						<span class="min-w-0 flex-1 truncate">{accountStatusLabel()}</span>
					</div>
				</div>

				<div class="space-y-3">
					<p class="text-xs font-medium text-muted-foreground">{text.subscriptionReady}</p>
					<p class="text-xs font-medium uppercase text-muted-foreground">{text.caldav}</p>
					<div class="flex min-w-0 items-start gap-2">
						<code class="block min-w-0 flex-1 rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">
							{syncInformation?.caldavURL ?? ''}
						</code>
						<CopyButton text={syncInformation?.caldavURL ?? ''} variant="outline" size="icon" class="shrink-0" disabled={!syncInformation?.caldavURL} />
					</div>
					<div class="space-y-1">
						<p class="text-[11px] font-medium text-muted-foreground">{text.username}</p>
						<div class="flex min-w-0 items-start gap-2">
							<code class="block min-w-0 flex-1 rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">
								{syncInformation?.caldavUsername ?? ''}
							</code>
							<CopyButton
								text={syncInformation?.caldavUsername ?? ''}
								variant="outline"
								size="icon"
								class="shrink-0"
								disabled={!syncInformation?.caldavUsername}
							/>
						</div>
					</div>
					<div class="space-y-1">
						<p class="text-[11px] font-medium text-muted-foreground">{text.password}</p>
						<div class="flex min-w-0 items-start gap-2">
							<code class="block min-w-0 flex-1 rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">
								{syncInformation?.caldavPassword ?? ''}
							</code>
							<CopyButton
								text={syncInformation?.caldavPassword ?? ''}
								variant="outline"
								size="icon"
								class="shrink-0"
								disabled={!syncInformation?.caldavPassword}
							/>
						</div>
					</div>
				</div>

				<Separator />

				<div class="space-y-2">
					<p class="text-xs font-medium uppercase text-muted-foreground">{text.ics}</p>
					<div class="flex min-w-0 items-start gap-2">
						<code class="block min-w-0 flex-1 rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">
							{syncInformation?.icsURL ?? ''}
						</code>
						<CopyButton text={syncInformation?.icsURL ?? ''} variant="outline" size="icon" class="shrink-0" disabled={!syncInformation?.icsURL} />
					</div>
				</div>

				<Button variant="outline" class="w-full justify-center gap-2" onclick={rotateSubscriptionURL} disabled={isRotatingSync}>
					<RotateCwIcon class={isRotatingSync ? 'size-4 animate-spin' : 'size-4'} />
					<span>{text.rotate}</span>
				</Button>
			</div>
		</Sheet.Content>
	</Sheet.Root>
{/if}
