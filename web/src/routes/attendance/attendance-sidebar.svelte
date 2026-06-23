<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import ClipboardCheckIcon from '@lucide/svelte/icons/clipboard-check';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { getAttendanceState } from './attendance-context.svelte';
	import QuickActions from './quick-actions.svelte';
	import { attendanceText } from './text';

	const text = createPageText(attendanceText);
	const attendance = getAttendanceState();
</script>

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
		{#if attendance.summary?.isAdmin}
			<div class="space-y-1 rounded-md border p-3">
				<Label for="attendance-team-visible" class="text-xs font-medium text-muted-foreground">
					{text.teamTabVisible}
				</Label>
				<div class="flex items-center justify-between">
					<span class="text-xs text-muted-foreground">
						{attendance.summary.teamViewVisibleToAll ? text.visibleToAll : text.adminOnly}
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
