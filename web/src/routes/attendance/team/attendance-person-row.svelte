<script lang="ts">
 import TeamStatusDayProgress from './team-status-day-progress.svelte';
 import type { TeamStatusPersonDay } from './team-status-table-model';
 import type { Snippet } from 'svelte';
 import PersonAvatar from '$lib/components/person-avatar.svelte';
 import ColoredOutlineBadge from '$lib/components/colored-outline-badge.svelte';
 import { IsMobile } from '$lib/hooks/is-mobile.svelte';
 import LocationLabel from '../shared/location-label.svelte';
 import * as Card from '$lib/components/ui/card';
 import { Skeleton } from '$lib/components/ui/skeleton';
 let {name, email, status, statusLabel, location, onclick, action, day, progressLoading = false}: {
  name: string; email: string; status: string; statusLabel: string;
  location?: string | null; onclick: () => void; action?: Snippet; day?: TeamStatusPersonDay; progressLoading?: boolean;
 } = $props();
 const isPhone = new IsMobile();
 const workLocation = $derived(status === 'working' || status === 'needs_checkout' ? location : null);
</script>
{#snippet metadata()}
 {#if isPhone.current}
  {#if status !== 'not_started'}<span class="text-xs text-muted-foreground">{statusLabel}</span>{/if}
  {#if workLocation}<LocationLabel name={workLocation} form="marker" />{/if}
 {:else}
  {#if workLocation}<LocationLabel name={workLocation} />{/if}
  {#if status !== 'not_started'}<ColoredOutlineBadge>{statusLabel}</ColoredOutlineBadge>{/if}
 {/if}
{/snippet}
<Card.Content class="py-3">
 <div class="grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3">
  <button type="button" class={"flex min-w-0 items-center gap-3 text-left " + (!action ? "col-span-2" : "")} {onclick}>
   <PersonAvatar {name} {email} class="size-9" />
   <span class="min-w-0 flex-1">
    <span class="block truncate font-medium">{name}</span>
    {#if action}<span class="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-1 sm:mt-1 sm:gap-2">{@render metadata()}</span>
    {:else}<span class="block truncate text-xs text-muted-foreground">{email}</span>{/if}
   </span>
   {#if !action}<span class="flex shrink-0 items-center gap-3 sm:gap-2">{@render metadata()}</span>{/if}
  </button>
  {#if action}<div class="order-3 col-span-2 mt-3 sm:order-none sm:col-span-1 sm:mt-0">{@render action()}</div>{/if}
 {#if day || progressLoading}<div aria-busy={progressLoading} class={action ? "order-2 col-span-2 mt-3 sm:order-3" : "col-span-2 mt-2 pl-12"}>{#if day}<TeamStatusDayProgress {day} />{:else}<Skeleton class="h-1.5 w-full rounded-full" data-testid="employee-today-progress-loading" aria-hidden="true" />{/if}</div>{/if}
 </div>
</Card.Content>
