<script lang="ts">
	import * as Tooltip from '$lib/components/ui/tooltip';
	import AttendanceCompanyMetrics from '../../routes/attendance/team/attendance-company-metrics.svelte';
	import AttendanceTeamCard from '../../routes/attendance/team/attendance-team-card.svelte';
	import ActivityAvatar from './activity-avatar.svelte';
	import ActivityTaskList from './activity-task-list.svelte';
	import ActivityContributions from './activity-contributions.svelte';
	import { dataRoomCompanyText } from './company-text';
	import { contributionTotal, type CompanyActivity } from './activity';

	type RecentActor = CompanyActivity['attendance']['teams'][number]['recentClockIns'][number];

	let { activity, locale }: { activity: CompanyActivity; locale: 'ko' | 'en' } = $props();

	const shownTeams = 4;
	const text = $derived(dataRoomCompanyText[locale].activity);
	const allTeams = $derived(activity.attendance.teams.length);
	const total = $derived(contributionTotal(activity));
	const teams = $derived(
		activity.attendance.teams.toSorted((left, right) => right.memberCount - left.memberCount).slice(0, shownTeams)
	);
	const isAttendanceToday = $derived(activity.attendance.date === activity.today);
	const attendanceDay = $derived(new Intl.DateTimeFormat(locale, { month: 'long', day: 'numeric', weekday: 'short', timeZone: 'UTC' })
		.format(new Date(`${activity.attendance.date}T00:00:00Z`)));
	const asOf = $derived(new Intl.DateTimeFormat(locale, { month: 'long', day: 'numeric', weekday: 'short', timeZone: 'UTC' })
		.format(new Date(`${activity.today}T00:00:00Z`)));
</script>

{#snippet recentActors(actors: RecentActor[], label: string)}
	<div class="flex min-w-0 items-center justify-between gap-2 max-sm:flex-col max-sm:items-start max-sm:justify-start max-sm:gap-1.5">
		<span class="text-muted-foreground text-xs">{label}</span>
		<span class="flex -space-x-2">
			{#each actors.slice(0, 4) as actor (actor.person)}
				<ActivityAvatar person={activity.people[actor.person]} seed={actor.person} class="ring-card size-8 ring-2" />
			{/each}
		</span>
	</div>
{/snippet}

<Tooltip.Provider>
	<section class="grid gap-6" aria-labelledby="company-activity">
		<div class="grid gap-1">
			<div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
				<h3 id="company-activity" class="text-lg font-semibold">{text.title}</h3>
				<p class="text-muted-foreground text-xs">{text.asOf(asOf)}</p>
			</div>
			<p class="text-muted-foreground text-sm">{text.description}</p>
		</div>

		<div class="bg-card grid gap-4 rounded-lg border p-5">
			<div class="grid gap-1">
				<h4 class="text-muted-foreground text-xs">{text.contributions}</h4>
				<p class="text-2xl font-semibold tracking-tight tabular-nums">{text.contributionsLabel(total)}</p>
			</div>
			<ActivityContributions {activity} {locale} />
		</div>

		<div class="grid gap-3">
			<div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
				<h4 class="text-sm font-semibold">{text.attendance(attendanceDay, isAttendanceToday)}</h4>
				<p class="text-muted-foreground text-xs">{text.tracked(activity.attendance.summary.memberCount)}{#if allTeams > teams.length} · {text.teamsShown(teams.length, allTeams)}{/if}</p>
			</div>
			<AttendanceCompanyMetrics summary={activity.attendance.summary} />
			{#if teams.length > 0}
				<div class="grid gap-3 min-[761px]:grid-cols-2 lg:grid-cols-4">
					{#each teams as team (team.teamKey)}
						<AttendanceTeamCard {team} name={team.name} actorStack={recentActors} />
					{/each}
				</div>
			{/if}
		</div>

		<div class="grid gap-3">
			<h4 class="text-sm font-semibold">{text.board}</h4>
			<ActivityTaskList {activity} {locale} />
		</div>
	</section>
</Tooltip.Provider>
