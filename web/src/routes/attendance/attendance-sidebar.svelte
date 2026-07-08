<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ClipboardCheckIcon from '@lucide/svelte/icons/clipboard-check';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { getAttendanceState } from './attendance-context.svelte';
	import PersonalToolsPanel from './personal/personal-tools-panel.svelte';
	import { timeZoneDisplayLabel } from './shared/attendance-date';
	import { attendanceText } from './text';

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
</script>

<aside class="flex w-60 shrink-0 flex-col border-r bg-background max-md:hidden">
	<div class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
		<ClipboardCheckIcon class="size-4 text-muted-foreground" />
		<div class="min-w-0 flex-1">
			<p class="truncate text-sm font-medium">{text.title}</p>
			<p class="truncate text-xs text-muted-foreground">
				{attendance.summary ? timeZoneDisplayLabel(attendance.summary.timeZone) : '-'}
			</p>
		</div>
		<Button variant="ghost" size="icon-sm" aria-label={text.refresh} onclick={() => attendance.load()}>
			<RefreshCwIcon class={attendance.isLoading ? 'animate-spin' : ''} />
		</Button>
	</div>

	<PersonalToolsPanel containerClass="flex-1 p-4" />
</aside>
