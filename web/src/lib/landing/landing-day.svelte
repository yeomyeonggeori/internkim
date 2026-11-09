<script lang="ts">
	import type { Component } from 'svelte';
	import { isAppSection, type AppSection } from '$lib/components/app-section-icons';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import LandingAttendance from './landing-attendance.svelte';
	import LandingCalendar from './landing-calendar.svelte';
	import LandingDataRoom from './landing-data-room.svelte';
	import LandingMessage from './landing-message.svelte';
	import LandingMessenger from './landing-messenger.svelte';
	import LandingTasks from './landing-tasks.svelte';
	import LandingWindow from './landing-window.svelte';
	import { landingText } from './text';

	const text = createPageText(landingText);
	const scenes: Partial<Record<AppSection, Component>> = {
		attendance: LandingAttendance,
		calendar: LandingCalendar,
		task: LandingTasks,
		messenger: LandingMessenger,
		dataRoom: LandingDataRoom
	};
</script>

<ol class="grid">
	{#each text.day.moments as moment (moment.time)}
		{@const Scene = isAppSection(moment.section) ? scenes[moment.section] : undefined}
		<li class="grid gap-8 border-t py-14 first:border-t-0 first:pt-4 lg:grid-cols-[minmax(0,21rem)_minmax(0,1fr)] lg:gap-12">
			<div class="grid content-start gap-5 lg:sticky lg:top-24 lg:self-start">
				<div class="grid gap-1.5">
					<time class="text-muted-foreground font-mono text-sm tabular-nums">{moment.time}</time>
					<p class="text-lg leading-7 font-semibold break-keep">{moment.caption}</p>
					{#if moment.note}<p class="text-muted-foreground mt-1 leading-7 break-keep">{moment.note}</p>{/if}
				</div>
				{#if moment.turns.length > 0}
					<div class="grid gap-3">
						{#each moment.turns as turn, index (index)}
							<LandingMessage from={turn.from} text={turn.text} agentName={text.demo.conversation} />
						{/each}
					</div>
				{/if}
			</div>
			{#if Scene && isAppSection(moment.section)}
				<LandingWindow section={moment.section}><Scene /></LandingWindow>
			{/if}
		</li>
	{/each}
</ol>
