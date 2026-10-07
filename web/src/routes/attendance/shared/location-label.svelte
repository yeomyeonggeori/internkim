<script lang="ts">
 import ColoredOutlineBadge from '$lib/components/colored-outline-badge.svelte';
 import ColorMarker from '$lib/components/color-marker.svelte';
 import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
 import { cn } from '$lib/utils';
 let { name, color, count, form = 'tag', class: className = '' }: {name: string; color?: string; count?: number; form?: 'tag' | 'marker'; class?: string} = $props();
 const registeredColor = $derived(color || myAttendanceToday.locations.find(location => location.name === name)?.color);
</script>
{#if form === 'marker'}
 <span class={cn('inline-flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground', className)} title={name}>
  <ColorMarker color={registeredColor} /><span class="max-w-32 truncate">{name}</span>{#if count !== undefined}<span class="tabular-nums text-foreground">{count}</span>{/if}
 </span>
{:else}
 <ColoredOutlineBadge color={registeredColor} class={className} title={name}>
  <span class="max-w-32 truncate">{name}</span>{#if count !== undefined}<span class="ml-1 tabular-nums">{count}</span>{/if}
 </ColoredOutlineBadge>
{/if}
