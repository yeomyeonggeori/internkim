<script lang="ts">
	import { pageActions } from '$lib/components/app-page-actions.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { onMount } from 'svelte';
	import { AttendanceState, setAttendanceState } from './attendance-context.svelte';
	import AttendanceSidebar from './attendance-sidebar.svelte';
	import { attendanceText } from './text';

	let { children } = $props();

	const text = createPageText(attendanceText);
	const attendance = new AttendanceState(text.loadFailed);
	setAttendanceState(attendance);

	function refreshServerClock(): void {
		void attendance.refreshServerClock();
	}

	function refreshVisibleServerClock(): void {
		if (document.visibilityState !== 'visible') return;
		refreshServerClock();
	}

	onMount(() => {
		void attendance.load();
		const releaseRefresh = pageActions.setRefresh(() => attendance.load());
		window.addEventListener('focus', refreshServerClock);
		window.addEventListener('pageshow', refreshServerClock);
		document.addEventListener('visibilitychange', refreshVisibleServerClock);
		return () => {
			window.removeEventListener('focus', refreshServerClock);
			window.removeEventListener('pageshow', refreshServerClock);
			document.removeEventListener('visibilitychange', refreshVisibleServerClock);
			releaseRefresh();
		};
	});

	$effect(() => {
		attendance.selectedMonth;
		attendance.chartMode;
		attendance.persistFilters();
	});
</script>

<div class="flex min-h-0 min-w-0 flex-1">
	<AttendanceSidebar />

	<div class="min-w-0 flex-1 overflow-y-auto p-3 md:p-6">
		{@render children()}
	</div>
</div>
