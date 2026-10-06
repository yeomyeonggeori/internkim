<script lang="ts">
 import { ABSENCE_TONE } from '../shared/color-tokens';
 import type { TeamStatusPersonDay } from './team-status-table-model';
 let {day}: {day: TeamStatusPersonDay} = $props();
 const total = $derived(Math.min(100, day.timelineSegments.reduce((sum, segment) => sum + segment.widthPercent, 0)));
</script>
<span class={`block h-1.5 rounded-full ${day.tone === 'absence' ? ABSENCE_TONE[day.absenceTone ?? 'other'].meter : 'bg-muted'}`} data-testid="employee-today-progress" aria-hidden="true">
 {#if day.tone !== 'absence' && day.timelineSegments.length}
  <span class="flex h-full overflow-hidden rounded-full" style:width={`${total}%`}>
   {#each day.timelineSegments as segment (segment.id)}
    <span class="h-full" style:flex-grow={segment.widthPercent} style:background-color={segment.kind === 'leave' ? ABSENCE_TONE.leave.bar : segment.locationColor ?? ABSENCE_TONE.other.bar}></span>
   {/each}
  </span>
 {/if}
</span>
