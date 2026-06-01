<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import ChevronLeftIcon from '@lucide/svelte/icons/chevron-left';
	import ChevronRightIcon from '@lucide/svelte/icons/chevron-right';
	import ClipboardCheckIcon from '@lucide/svelte/icons/clipboard-check';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { Popover } from 'bits-ui';
	import { onMount } from 'svelte';
	import { AttendanceState, setAttendanceState } from './attendance-context.svelte';
	import { attendanceText } from './text';
	import QuickActions from './quick-actions.svelte';

	let { children } = $props();

	const text = createPageText(attendanceText);
	const attendance = new AttendanceState(text.loadFailed);
	setAttendanceState(attendance);

	onMount(() => attendance.load());

	$effect(() => {
		attendance.selectedMonth;
		attendance.selectedEmail;
		attendance.chartMode;
		attendance.persistFilters();
	});

	const userOptions = () =>
		Array.from(
			new Map((attendance.summary?.events ?? []).map((event) => [event.email, event])).values()
		)
			.filter((event) => event.email)
			.sort((a, b) => displayName(a).localeCompare(displayName(b)));

	function displayName(event: { displayName?: string; mattermostUsername?: string; email?: string }) {
		return event.displayName || event.mattermostUsername || event.email || '';
	}

	const today = new Date();
	let isMonthPickerOpen = $state(false);
	let pickerMode = $state<'month' | 'year'>('month');
	let pickerYear = $state(today.getFullYear());
	let pickerYearWindowStart = $state(Math.floor(today.getFullYear() / 12) * 12);

	const selectedYearMonth = $derived(parseYearMonth(attendance.selectedMonth));

	function parseYearMonth(value: string): { year: number; month: number } {
		const [yearString, monthString] = value.split('-');
		const year = Number(yearString);
		const month = Number(monthString);
		if (!year || !month) return { year: today.getFullYear(), month: today.getMonth() + 1 };
		return { year, month };
	}

	function formatYearMonth(year: number, monthIndex: number): string {
		return `${year}-${String(monthIndex + 1).padStart(2, '0')}`;
	}

	const monthTriggerLabel = $derived(
		currentLocale.value === 'en'
			? new Date(selectedYearMonth.year, selectedYearMonth.month - 1, 1).toLocaleDateString('en-US', {
					year: 'numeric',
					month: 'long'
				})
			: `${selectedYearMonth.year}년 ${selectedYearMonth.month}월`
	);

	const monthShortLabels = $derived(
		Array.from({ length: 12 }, (_, i) =>
			currentLocale.value === 'en'
				? new Date(2000, i, 1).toLocaleDateString('en-US', { month: 'short' })
				: `${i + 1}월`
		)
	);

	function handlePickerOpenChange(open: boolean) {
		isMonthPickerOpen = open;
		if (open) {
			pickerMode = 'month';
			pickerYear = selectedYearMonth.year;
			pickerYearWindowStart = Math.floor(pickerYear / 12) * 12;
		}
	}

	function selectPickerMonth(monthIndex: number) {
		attendance.selectedMonth = formatYearMonth(pickerYear, monthIndex);
		isMonthPickerOpen = false;
		attendance.load();
	}

	function selectPickerYear(year: number) {
		pickerYear = year;
		pickerMode = 'month';
	}
</script>

<div class="flex min-h-0 flex-1">
	<aside class="flex w-60 shrink-0 flex-col border-r bg-background max-md:hidden">
		<div class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
			<ClipboardCheckIcon class="size-4 text-muted-foreground" />
			<div class="min-w-0 flex-1">
				<p class="truncate text-sm font-medium">{text.title}</p>
				<p class="truncate text-xs text-muted-foreground">{attendance.summary?.timeZone ?? '-'}</p>
			</div>
			<Button variant="ghost" size="icon-sm" aria-label={text.refresh} onclick={() => attendance.load()}>
				<RefreshCwIcon class={attendance.isLoading ? 'animate-spin' : ''} />
			</Button>
		</div>

		<div class="min-h-0 flex-1 space-y-4 overflow-auto p-4">
			<div class="space-y-2">
				<Label class="text-xs font-medium text-muted-foreground">{text.month}</Label>
				<Popover.Root open={isMonthPickerOpen} onOpenChange={handlePickerOpenChange}>
					<Popover.Trigger
						class="border-input bg-background hover:bg-accent flex h-9 w-full items-center justify-between rounded-md border px-3 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
					>
						{monthTriggerLabel}
					</Popover.Trigger>
					<Popover.Portal>
						<Popover.Content
							side="bottom"
							align="start"
							sideOffset={6}
							class="z-50 w-56 rounded-md border border-border/50 bg-popover p-2 text-popover-foreground shadow-sm outline-none data-open:animate-in data-closed:animate-out data-open:fade-in-0 data-closed:fade-out-0 data-open:zoom-in-95 data-closed:zoom-out-95"
						>
							<header class="mb-1.5 flex items-center gap-1">
								<button
									type="button"
									aria-label={pickerMode === 'month' ? 'Previous year' : 'Previous 12 years'}
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
									aria-label={pickerMode === 'month' ? 'Next year' : 'Next 12 years'}
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
											class="flex h-8 items-center justify-center rounded text-xs tabular-nums transition-colors {pickerYear === selectedYearMonth.year && i === selectedYearMonth.month - 1
												? 'bg-primary font-semibold text-primary-foreground'
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
			</div>

			<div class="space-y-2">
				<Label for="attendance-user" class="text-xs font-medium text-muted-foreground">{text.user}</Label>
				<select
					id="attendance-user"
					class="border-input bg-background h-9 w-full rounded-md border px-2 text-sm"
					bind:value={attendance.selectedEmail}
					onchange={() => {
						attendance.setTab(attendance.selectedEmail ? 'personal' : 'team');
						attendance.load();
					}}
					disabled={!attendance.summary?.isAdmin}
				>
					<option value="">{text.allUsers}</option>
					{#each userOptions() as event (event.email)}
						<option value={event.email}>{displayName(event)}</option>
					{/each}
				</select>
			</div>

			{#if attendance.summary?.isAdmin}
				<div class="space-y-1 rounded-md border p-3">
					<Label for="attendance-team-visible" class="text-xs font-medium text-muted-foreground">
						팀 탭 공개
					</Label>
					<div class="flex items-center justify-between">
						<span class="text-xs text-muted-foreground">
							{attendance.summary.teamViewVisibleToAll ? '전원 공개' : '관리자만'}
						</span>
						<Switch
							id="attendance-team-visible"
							checked={attendance.summary.teamViewVisibleToAll}
							onCheckedChange={(value) => attendance.updateTeamViewVisibility(value)}
						/>
					</div>
				</div>
			{/if}

			<QuickActions />
		</div>
	</aside>

	<div class="min-w-0 flex-1 overflow-y-auto p-6">
		{@render children()}
	</div>
</div>
