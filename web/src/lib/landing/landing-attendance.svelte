<script lang="ts">
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import AttendanceCompanyMetrics from '../../routes/attendance/team/attendance-company-metrics.svelte';
	import AttendanceTeamCard from '../../routes/attendance/team/attendance-team-card.svelte';
	import { sampleAttendanceSummary, sampleTeams } from './landing-samples';

	const teams = $derived(sampleTeams(currentLocale.value));
	const summary = $derived(sampleAttendanceSummary(teams));
</script>

<div class="grid gap-4 p-4 sm:p-6">
	<AttendanceCompanyMetrics {summary} />
	<div class="grid gap-4 sm:grid-cols-2">
		{#each teams as team (team.name)}
			<AttendanceTeamCard {team} name={team.name}>
				{#snippet face(person)}<PersonAvatar name={person.name} class="size-6" />{/snippet}
			</AttendanceTeamCard>
		{/each}
	</div>
</div>
