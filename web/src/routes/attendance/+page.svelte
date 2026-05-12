<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { onMount } from 'svelte';

	type AttendanceKind = 'clock_in' | 'clock_out';

	type AttendanceEvent = {
		id: string;
		mattermostUserID: string;
		mattermostUsername: string;
		email: string;
		displayName: string;
		kind: AttendanceKind;
		occurredAt: string;
		localDate: string;
		localTime: string;
		timeZoneAtEvent: string;
		source: string;
		resultPostID: string;
		canceledAt?: string;
		cancelReason?: string;
	};

	type AttendanceSummary = {
		month: string;
		currentUserEmail: string;
		isAdmin: boolean;
		timeZone: string;
		events: AttendanceEvent[];
		todayStatus: string;
	};

	let summary = $state<AttendanceSummary | null>(null);
	let selectedMonth = $state(new Date().toISOString().slice(0, 7));
	let selectedEmail = $state('');
	let isLoading = $state(false);
	let errorMessage = $state('');

	const attendanceStatusLabels: Record<string, string> = {
		clocked_in: '출근',
		clocked_out: '퇴근',
		not_clocked_in: '미출근'
	};
	const events = () => summary?.events ?? [];
	const activeEvents = () => events().filter((event) => !event.canceledAt);
	const userOptions = () => Array.from(new Set(events().map((event) => event.email).filter(Boolean))).sort();
	const visibleEvents = () => (selectedEmail ? events().filter((event) => event.email === selectedEmail) : events());
	const todayStatusLabel = () => attendanceStatusLabels[summary?.todayStatus ?? 'not_clocked_in'] ?? '미출근';
	const monthDays = () => {
		const [year, month] = selectedMonth.split('-').map((value) => Number(value));
		if (!year || !month) return [];
		const lastDay = new Date(year, month, 0).getDate();
		return Array.from({ length: lastDay }, (_, index) => `${selectedMonth}-${String(index + 1).padStart(2, '0')}`);
	};
	const daySummary = (localDate: string) => {
		const dayEvents = sortedEventsForDay(localDate);
		return {
			clockIn: dayEvents.find((event) => event.kind === 'clock_in'),
			clockOut: [...dayEvents].reverse().find((event) => event.kind === 'clock_out')
		};
	};

	onMount(loadAttendance);

	async function loadAttendance() {
		isLoading = true;
		errorMessage = '';
		try {
			const query = new URLSearchParams({ month: selectedMonth });
			if (selectedEmail) query.set('email', selectedEmail);
			const response = await fetch(`/attendance/api/summary?${query}`, { credentials: 'include' });
			if (!response.ok) throw new Error(await response.text());
			summary = (await response.json()) as AttendanceSummary;
			selectedMonth = summary.month;
		} catch (error) {
			errorMessage = error instanceof Error ? error.message : '출결 정보를 불러오지 못했습니다.';
			summary = null;
		} finally {
			isLoading = false;
		}
	}

	function kindLabel(kind: AttendanceKind) {
		return kind === 'clock_out' ? '퇴근' : '출근';
	}

	function sortedEventsForDay(localDate: string) {
		return activeEvents()
			.filter((event) => event.localDate === localDate && isVisibleEventOwner(event))
			.sort((left, right) => left.localTime.localeCompare(right.localTime));
	}

	function isVisibleEventOwner(event: AttendanceEvent) {
		return !selectedEmail || event.email === selectedEmail;
	}
</script>

<svelte:head>
	<title>출결 · intern kim</title>
</svelte:head>

<main class="flex min-h-[calc(100svh-48px)] flex-col bg-background text-foreground">
	<div class="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
		<div class="min-w-0">
			<h1 class="truncate text-sm font-medium">출결</h1>
			<p class="truncate text-xs text-muted-foreground">Attendance 채널 버튼으로 기록된 출퇴근 로그</p>
		</div>
		<div class="flex flex-wrap items-center gap-2">
			<Input class="w-36" type="month" bind:value={selectedMonth} onchange={loadAttendance} />
			{#if summary?.isAdmin && userOptions().length > 0}
				<select class="h-9 rounded-md border bg-background px-3 text-sm" bind:value={selectedEmail} onchange={loadAttendance}>
					<option value="">전체</option>
					{#each userOptions() as email (email)}
						<option value={email}>{email}</option>
					{/each}
				</select>
			{/if}
			<Button variant="ghost" size="icon-sm" aria-label="Refresh attendance" onclick={loadAttendance}>
				<RefreshCwIcon />
			</Button>
		</div>
	</div>

	<div class="grid gap-4 p-4">
		{#if errorMessage}
			<p class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">{errorMessage}</p>
		{/if}

		<section class="grid gap-3 md:grid-cols-3">
			<div class="rounded-lg border p-4">
				<p class="text-xs text-muted-foreground">오늘 상태</p>
				<p class="mt-2 text-2xl font-semibold">{todayStatusLabel()}</p>
			</div>
			<div class="rounded-lg border p-4">
				<p class="text-xs text-muted-foreground">이번 달 기록</p>
				<p class="mt-2 text-2xl font-semibold">{activeEvents().length}</p>
			</div>
			<div class="rounded-lg border p-4">
				<p class="text-xs text-muted-foreground">시간대</p>
				<p class="mt-2 truncate text-lg font-semibold">{summary?.timeZone ?? '-'}</p>
			</div>
		</section>

		<section class="rounded-lg border">
			<div class="border-b px-4 py-3">
				<h2 class="text-sm font-medium">월별 요약</h2>
			</div>
			<div class="grid grid-cols-2 gap-px bg-border sm:grid-cols-4 lg:grid-cols-7">
				{#each monthDays() as localDate (localDate)}
					{@const day = daySummary(localDate)}
					<div class="min-h-24 bg-background p-3">
						<p class="text-xs font-medium">{Number(localDate.slice(-2))}</p>
						<div class="mt-3 grid gap-1 text-xs">
							<p class={day.clockIn ? 'text-foreground' : 'text-muted-foreground'}>출근 {day.clockIn?.localTime.slice(0, 5) ?? '-'}</p>
							<p class={day.clockOut ? 'text-foreground' : 'text-muted-foreground'}>퇴근 {day.clockOut?.localTime.slice(0, 5) ?? '-'}</p>
						</div>
					</div>
				{/each}
			</div>
		</section>

		<section class="rounded-lg border">
			<div class="border-b px-4 py-3">
				<h2 class="text-sm font-medium">로그</h2>
			</div>
			{#if isLoading}
				<p class="p-4 text-sm text-muted-foreground">불러오는 중...</p>
			{:else if visibleEvents().length === 0}
				<p class="p-4 text-sm text-muted-foreground">기록이 없습니다.</p>
			{:else}
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>날짜</Table.Head>
							<Table.Head>시간</Table.Head>
							<Table.Head>사용자</Table.Head>
							<Table.Head>종류</Table.Head>
							<Table.Head>상태</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each visibleEvents() as event (event.id)}
							<Table.Row>
								<Table.Cell>{event.localDate}</Table.Cell>
								<Table.Cell>{event.localTime.slice(0, 5)}</Table.Cell>
								<Table.Cell>
									<div class="min-w-0">
										<p class="truncate text-sm">{event.displayName || event.mattermostUsername || event.email}</p>
										<p class="truncate text-xs text-muted-foreground">{event.email}</p>
									</div>
								</Table.Cell>
								<Table.Cell>{kindLabel(event.kind)}</Table.Cell>
								<Table.Cell>
									<Badge variant={event.canceledAt ? 'outline' : 'secondary'}>{event.canceledAt ? '취소' : '기록'}</Badge>
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			{/if}
		</section>
	</div>
</main>
