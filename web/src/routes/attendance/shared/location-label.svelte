<script lang="ts">
 import ColoredOutlineBadge from '$lib/components/colored-outline-badge.svelte';
 import { myAttendanceToday } from '$lib/attendance/my-attendance-today.svelte';
 let { name, color, count, class: className = '' }: {name: string; color?: string; count?: number; class?: string} = $props();
 const registeredColor = $derived(color || myAttendanceToday.locations.find(location => location.name === name)?.color);
</script>
<ColoredOutlineBadge color={registeredColor} class={className} title={name}>
 <span class="max-w-32 truncate">{name}</span>{#if count !== undefined}<span class="ml-1 tabular-nums">{count}</span>{/if}
</ColoredOutlineBadge>
