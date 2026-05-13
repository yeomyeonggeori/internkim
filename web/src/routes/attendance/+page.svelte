<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Table from '$lib/components/ui/table';
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import CalendarDaysIcon from '@lucide/svelte/icons/calendar-days';
	import Clock3Icon from '@lucide/svelte/icons/clock-3';
	import LogInIcon from '@lucide/svelte/icons/log-in';
	import LogOutIcon from '@lucide/svelte/icons/log-out';
	import MapPinIcon from '@lucide/svelte/icons/map-pin';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import TimerIcon from '@lucide/svelte/icons/timer';
	import TrendingUpIcon from '@lucide/svelte/icons/trending-up';
	import UsersIcon from '@lucide/svelte/icons/users';
	import { onMount } from 'svelte';

	type AttendanceKind = 'clock_in' | 'clock_out';
	type ChartMode = 'day' | 'week' | 'month';

	type AttendanceLocation = {
		id: string;
		name: string;
		color: string;
		isDefault: boolean;
	};

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
		locationID?: string;
		locationName?: string;
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
		locations: AttendanceLocation[];
	};

	type WorkSegment = {
		email: string;
		localDate: string;
		startTime: string;
		endTime: string;
		minutes: number;
		locationName: string;
	};

	type ChartPoint = {
		label: string;
		minutes: number;
		x: number;
		y: number;
	};

	let summary = $state<AttendanceSummary | null>(null);
	let selectedMonth = $state(new Date().toISOString().slice(0, 7));
	let selectedEmail = $state('');
	let selectedLocationID = $state('');
	let chartMode = $state<ChartMode>('day');
	let isLoading = $state(false);
	let errorMessage = $state('');

	const events = () => summary?.events ?? [];
	const locations = () => summary?.locations ?? [];
	const activeEvents = () => events().filter((event) => !event.canceledAt);
	const visibleEvents = () =>
		activeAndCanceledVisibleEvents().filter((event) => {
			if (!selectedLocationID) return true;
			return event.kind === 'clock_in' && event.locationID === selectedLocationID;
		});
	const activeAndCanceledVisibleEvents = () => (selectedEmail ? events().filter((event) => event.email === selectedEmail) : events());
	const activeVisibleEvents = () => visibleEvents().filter((event) => !event.canceledAt);
	const userOptions = () =>
		Array.from(new Map(events().map((event) => [event.email, event])).values())
			.filter((event) => event.email)
			.sort((left, right) => displayName(left).localeCompare(displayName(right)));
	const currentUserEvents = () => activeEvents().filter((event) => event.email === summary?.currentUserEmail);
	const currentUserSegments = () => workSegments(currentUserEvents());
	const visibleSegments = () => workSegments(activeVisibleEvents());
	const todayDate = () => new Date().toISOString().slice(0, 10);
	const todayMinutes = () => sumMinutes(currentUserSegments().filter((segment) => segment.localDate === todayDate()));
	const monthMinutes = () => sumMinutes(currentUserSegments());
	const weekMinutes = () => {
		const today = new Date();
		const weekStart = new Date(today);
		weekStart.setDate(today.getDate() - today.getDay());
		const start = weekStart.toISOString().slice(0, 10);
		return sumMinutes(currentUserSegments().filter((segment) => segment.localDate >= start));
	};
	const workedDayCount = () => new Set(currentUserSegments().map((segment) => segment.localDate)).size;
	const isWorking = () => summary?.todayStatus === 'clocked_in';
	const chartData = () => chartPoints(chartBuckets());

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

	function displayName(event: AttendanceEvent) {
		return event.displayName || event.mattermostUsername || event.email;
	}

	function eventIcon(kind: AttendanceKind) {
		return kind === 'clock_in' ? LogInIcon : LogOutIcon;
	}

	function eventTone(kind: AttendanceKind) {
		return kind === 'clock_in' ? 'text-emerald-600 bg-emerald-500/10' : 'text-rose-600 bg-rose-500/10';
	}

	function eventLabel(kind: AttendanceKind) {
		return kind === 'clock_in' ? '출근' : '퇴근';
	}

	function formatDuration(minutes: number) {
		const hours = Math.floor(minutes / 60);
		const remainingMinutes = Math.round(minutes % 60);
		if (hours === 0) return `${remainingMinutes}분`;
		if (remainingMinutes === 0) return `${hours}시간`;
		return `${hours}시간 ${remainingMinutes}분`;
	}

	function minutesBetween(start: AttendanceEvent, end: AttendanceEvent) {
		const startTime = new Date(start.occurredAt).getTime();
		const endTime = new Date(end.occurredAt).getTime();
		return Math.max(0, Math.round((endTime - startTime) / 60000));
	}

	function workSegments(sourceEvents: AttendanceEvent[]) {
		const grouped = new Map<string, AttendanceEvent[]>();
		for (const event of sourceEvents) {
			const key = `${event.email}:${event.localDate}`;
			grouped.set(key, [...(grouped.get(key) ?? []), event]);
		}
		const segments: WorkSegment[] = [];
		for (const dayEvents of grouped.values()) {
			const sortedEvents = [...dayEvents].sort((left, right) => left.occurredAt.localeCompare(right.occurredAt));
			let openEvent: AttendanceEvent | null = null;
			for (const event of sortedEvents) {
				if (event.kind === 'clock_in') {
					openEvent = event;
					continue;
				}
				if (!openEvent) continue;
				segments.push({
					email: event.email,
					localDate: event.localDate,
					startTime: openEvent.localTime,
					endTime: event.localTime,
					minutes: minutesBetween(openEvent, event),
					locationName: openEvent.locationName || defaultLocationName()
				});
				openEvent = null;
			}
		}
		return segments;
	}

	function sumMinutes(segments: WorkSegment[]) {
		return segments.reduce((total, segment) => total + segment.minutes, 0);
	}

	function defaultLocationName() {
		return locations().find((location) => location.isDefault)?.name ?? locations()[0]?.name ?? '사무실';
	}

	function monthDays() {
		const [year, month] = selectedMonth.split('-').map((value) => Number(value));
		if (!year || !month) return [];
		const lastDay = new Date(year, month, 0).getDate();
		return Array.from({ length: lastDay }, (_, index) => `${selectedMonth}-${String(index + 1).padStart(2, '0')}`);
	}

	function daySummary(localDate: string) {
		const dayEvents = activeVisibleEvents()
			.filter((event) => event.localDate === localDate)
			.sort((left, right) => left.localTime.localeCompare(right.localTime));
		return {
			clockIn: dayEvents.find((event) => event.kind === 'clock_in'),
			clockOut: [...dayEvents].reverse().find((event) => event.kind === 'clock_out'),
			minutes: sumMinutes(visibleSegments().filter((segment) => segment.localDate === localDate))
		};
	}

	function chartBuckets() {
		if (chartMode === 'day') {
			return monthDays().map((localDate) => ({
				label: localDate.slice(-2),
				minutes: sumMinutes(currentUserSegments().filter((segment) => segment.localDate === localDate))
			}));
		}
		if (chartMode === 'week') {
			const buckets = new Map<string, number>();
			for (const segment of currentUserSegments()) {
				const week = `${Math.ceil(Number(segment.localDate.slice(-2)) / 7)}주`;
				buckets.set(week, (buckets.get(week) ?? 0) + segment.minutes);
			}
			return ['1주', '2주', '3주', '4주', '5주'].map((label) => ({ label, minutes: buckets.get(label) ?? 0 }));
		}
		return [{ label: selectedMonth, minutes: monthMinutes() }];
	}

	function chartPoints(buckets: { label: string; minutes: number }[]) {
		const maximumMinutes = Math.max(60, ...buckets.map((bucket) => bucket.minutes));
		const width = 640;
		const height = 180;
		const horizontalStep = buckets.length > 1 ? width / (buckets.length - 1) : width;
		return buckets.map((bucket, index) => ({
			label: bucket.label,
			minutes: bucket.minutes,
			x: buckets.length > 1 ? index * horizontalStep : width / 2,
			y: height - (bucket.minutes / maximumMinutes) * 150 - 15
		}));
	}

	function areaPath(points: ChartPoint[]) {
		if (points.length === 0) return '';
		const line = points.map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x} ${point.y}`).join(' ');
		return `${line} L ${points[points.length - 1].x} 180 L ${points[0].x} 180 Z`;
	}

	function linePath(points: ChartPoint[]) {
		return points.map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x} ${point.y}`).join(' ');
	}
</script>

<svelte:head>
	<title>출결 · intern kim</title>
</svelte:head>

<main class="flex min-h-[calc(100svh-48px)] flex-col bg-background text-foreground">
	<div class="flex shrink-0 flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
		<div class="min-w-0">
			<h1 class="truncate text-sm font-medium">출결</h1>
			<p class="truncate text-xs text-muted-foreground">팀 전체 근무 기록과 개인 근무 시간</p>
		</div>
		<div class="flex flex-wrap items-center gap-2">
			<Input class="w-36" type="month" bind:value={selectedMonth} onchange={loadAttendance} />
			<select class="h-9 rounded-md border bg-background px-3 text-sm" bind:value={selectedEmail} onchange={loadAttendance}>
				<option value="">전체 사용자</option>
				{#each userOptions() as event (event.email)}
					<option value={event.email}>{displayName(event)}</option>
				{/each}
			</select>
			{#if locations().length > 1}
				<select class="h-9 rounded-md border bg-background px-3 text-sm" bind:value={selectedLocationID}>
					<option value="">전체 장소</option>
					{#each locations() as location (location.id)}
						<option value={location.id}>{location.name}</option>
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

		<section class="grid gap-3 md:grid-cols-4">
			<div class="rounded-lg border p-4">
				<div class="flex items-center gap-2 text-xs text-muted-foreground">
					<ActivityIcon class={isWorking() ? 'size-4 text-emerald-600' : 'size-4 text-muted-foreground'} />
					<span>현재 상태</span>
				</div>
				<p class="mt-3 text-2xl font-semibold">{isWorking() ? '근무 중' : '퇴근'}</p>
			</div>
			<div class="rounded-lg border p-4">
				<div class="flex items-center gap-2 text-xs text-muted-foreground">
					<TimerIcon class="size-4 text-sky-600" />
					<span>오늘</span>
				</div>
				<p class="mt-3 text-2xl font-semibold">{formatDuration(todayMinutes())}</p>
			</div>
			<div class="rounded-lg border p-4">
				<div class="flex items-center gap-2 text-xs text-muted-foreground">
					<TrendingUpIcon class="size-4 text-indigo-600" />
					<span>이번 주</span>
				</div>
				<p class="mt-3 text-2xl font-semibold">{formatDuration(weekMinutes())}</p>
			</div>
			<div class="rounded-lg border p-4">
				<div class="flex items-center gap-2 text-xs text-muted-foreground">
					<CalendarDaysIcon class="size-4 text-amber-600" />
					<span>이번 달</span>
				</div>
				<p class="mt-3 text-2xl font-semibold">{formatDuration(monthMinutes())}</p>
				<p class="mt-1 text-xs text-muted-foreground">{workedDayCount()}일 근무</p>
			</div>
		</section>

		<section class="rounded-lg border p-4">
			<div class="mb-3 flex flex-wrap items-center justify-between gap-3">
				<div class="flex items-center gap-2">
					<Clock3Icon class="size-4 text-sky-600" />
					<h2 class="text-sm font-medium">내 근무 시간</h2>
				</div>
				<div class="flex rounded-md border p-1">
					{#each ['day', 'week', 'month'] as mode}
						<Button variant={chartMode === mode ? 'secondary' : 'ghost'} size="sm" onclick={() => (chartMode = mode as ChartMode)}>
							{mode === 'day' ? '일별' : mode === 'week' ? '주별' : '월별'}
						</Button>
					{/each}
				</div>
			</div>
			<div class="h-56 overflow-hidden rounded-md bg-muted/30 px-3 py-4">
				<svg viewBox="0 0 640 210" class="h-full w-full" role="img" aria-label="근무 시간 추세">
					<defs>
						<linearGradient id="attendance-area" x1="0" x2="0" y1="0" y2="1">
							<stop offset="0%" stop-color="#0ea5e9" stop-opacity="0.34" />
							<stop offset="100%" stop-color="#0ea5e9" stop-opacity="0.02" />
						</linearGradient>
					</defs>
					<path d={areaPath(chartData())} fill="url(#attendance-area)" />
					<path d={linePath(chartData())} fill="none" stroke="#0284c7" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />
					{#each chartData() as point}
						<circle cx={point.x} cy={point.y} r="3.5" fill="#0284c7" />
						<text x={point.x} y="204" text-anchor="middle" class="fill-muted-foreground text-[10px]">{point.label}</text>
					{/each}
				</svg>
			</div>
		</section>

		<section class="rounded-lg border">
			<div class="flex items-center gap-2 border-b px-4 py-3">
				<CalendarDaysIcon class="size-4 text-muted-foreground" />
				<h2 class="text-sm font-medium">월간 캘린더</h2>
			</div>
			<div class="grid grid-cols-2 gap-px bg-border sm:grid-cols-4 lg:grid-cols-7">
				{#each monthDays() as localDate (localDate)}
					{@const day = daySummary(localDate)}
					<div class="min-h-24 bg-background p-3">
						<div class="flex items-center justify-between gap-2">
							<p class="text-xs font-medium">{Number(localDate.slice(-2))}</p>
							{#if day.minutes > 0}
								<span class="text-xs text-sky-700">{formatDuration(day.minutes)}</span>
							{/if}
						</div>
						<div class="mt-3 grid gap-1 text-xs">
							<p class="flex items-center gap-1 {day.clockIn ? 'text-emerald-700' : 'text-muted-foreground'}">
								<LogInIcon class="size-3.5" />
								<span>{day.clockIn?.localTime.slice(0, 5) ?? '-'}</span>
								{#if day.clockIn?.locationName}
									<span class="truncate text-muted-foreground">· {day.clockIn.locationName}</span>
								{/if}
							</p>
							<p class="flex items-center gap-1 {day.clockOut ? 'text-rose-700' : 'text-muted-foreground'}">
								<LogOutIcon class="size-3.5" />
								<span>{day.clockOut?.localTime.slice(0, 5) ?? '-'}</span>
							</p>
						</div>
					</div>
				{/each}
			</div>
		</section>

		<section class="rounded-lg border">
			<div class="flex items-center justify-between gap-3 border-b px-4 py-3">
				<div class="flex items-center gap-2">
					<UsersIcon class="size-4 text-muted-foreground" />
					<h2 class="text-sm font-medium">팀 로그</h2>
				</div>
				<Badge variant="outline">{summary?.timeZone ?? '-'}</Badge>
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
							<Table.Head>사용자</Table.Head>
							<Table.Head>이벤트</Table.Head>
							<Table.Head>장소</Table.Head>
							<Table.Head>상태</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each visibleEvents() as event (event.id)}
							{@const Icon = eventIcon(event.kind)}
							<Table.Row class={event.canceledAt ? 'opacity-55' : ''}>
								<Table.Cell>
									<div class="min-w-0">
										<p class="text-sm">{event.localDate}</p>
										<p class="text-xs text-muted-foreground">{event.localTime.slice(0, 5)}</p>
									</div>
								</Table.Cell>
								<Table.Cell>
									<div class="min-w-0">
										<p class="truncate text-sm">{displayName(event)}</p>
										<p class="truncate text-xs text-muted-foreground">{event.email}</p>
									</div>
								</Table.Cell>
								<Table.Cell>
									<span class="inline-flex items-center gap-1 rounded-full px-2 py-1 text-xs {eventTone(event.kind)}">
										<Icon class="size-3.5" />
										{eventLabel(event.kind)}
									</span>
								</Table.Cell>
								<Table.Cell>
									{#if event.locationName}
										<span class="inline-flex items-center gap-1 text-sm">
											<MapPinIcon class="size-3.5 text-muted-foreground" />
											{event.locationName}
										</span>
									{:else}
										<span class="text-sm text-muted-foreground">-</span>
									{/if}
								</Table.Cell>
								<Table.Cell>
									<Badge variant={event.canceledAt ? 'outline' : 'secondary'}>{event.canceledAt ? '취소됨' : '기록'}</Badge>
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			{/if}
		</section>
	</div>
</main>
