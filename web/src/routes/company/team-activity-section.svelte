<script lang="ts">
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import FlowTaskBoardCard from '../flow/flow-task-board-card.svelte';
	import type { FlowTask } from '../flow/flow-types';
	import CompanyStaffChip from './company-staff-chip.svelte';
	import {
		companyActivityIntensity,
		companyActivityPulsePoints,
		companyWorkStatusPercentage,
		type CompanyShareMember,
		type CompanyShareTeamActivity,
		type CompanyShareWorkStatus
	} from './company-page-model';

	type TeamActivityText = {
		title: string;
		description: string;
		activeTeam: string;
		people: string;
		activityRhythm: string;
		activityGrid: string;
		attendanceSignals: string;
		workSignals: string;
		workDistribution: string;
		recentWork: string;
		teamMember: string;
		defaultJobTitle: string;
		otherBusiness: string;
		statusLabels: Record<CompanyShareWorkStatus['status'], string>;
	};

	let { activity, text, language }: { activity: CompanyShareTeamActivity; text: TeamActivityText; language: 'ko' | 'en' } = $props();
	const activityRhythmDays = 30;
	const pulsePoints = $derived(companyActivityPulsePoints(activity.days.slice(-activityRhythmDays)));

	function formatDay(date: string): string {
		return new Intl.DateTimeFormat(language, { month: 'short', day: 'numeric' }).format(new Date(`${date}T00:00:00`));
	}

	function formatActivityWindow(days: number): string {
		return language === 'ko' ? `${days}일` : `${days} days`;
	}

	function statusClass(status: CompanyShareWorkStatus['status']): string {
		switch (status) {
			case 'inProgress': return 'bg-blue-600';
			case 'completed': return 'bg-black';
			case 'paused': return 'bg-warning';
			case 'closed': return 'bg-destructive';
			default: return 'bg-muted-foreground/35';
		}
	}

	function memberLabel(surname: string | undefined, jobTitle: string | undefined): string {
		return `${surname || text.teamMember} ${jobTitle || text.defaultJobTitle}`;
	}

	function publicFlowTask(work: CompanyShareTeamActivity['recentWork'][number], member: CompanyShareMember): FlowTask {
		return {
			id: `company-${work.memberSeed}-${work.date}-${work.title}`,
			ownerID: work.memberSeed,
			ownerName: memberLabel(member.surname, member.jobTitle),
			participantIDs: [], participantNames: [], business: work.business ?? '', type: work.type ?? '',
			content: work.title, goal: '', size: work.size ?? '', status: work.status, statusRank: 0,
			startDate: work.startDate, endDate: work.endDate, weekCode: '', flag: 0
		};
	}

	function activityLabel(date: string, attendanceCount: number, workCount: number): string {
		return `${formatDay(date)} · ${text.attendanceSignals} ${attendanceCount} · ${text.workSignals} ${workCount}`;
	}
</script>

<section class="team-activity overflow-hidden rounded-xl border bg-card">
	<div class="grid lg:grid-cols-[minmax(0,1.55fr)_minmax(18rem,0.75fr)]">
		<div class="min-w-0 p-5 sm:p-8 lg:border-r">
			<div class="flex flex-wrap items-start justify-between gap-5">
				<div class="max-w-2xl">
					<div class="flex items-center gap-2">
						<ActivityIcon class="text-primary size-5" />
						<h2 class="text-2xl font-semibold text-balance">{text.title}</h2>
					</div>
					<p class="text-muted-foreground mt-3 max-w-[68ch] leading-7 text-pretty">{text.description}</p>
				</div>
				<div class="text-right">
					<p class="text-3xl font-semibold tabular-nums">{activity.members.length}</p>
					<p class="text-muted-foreground text-sm">{text.people}</p>
				</div>
			</div>

			<div class="mt-8 flex items-end justify-between gap-5">
				<div>
					<p class="text-sm font-medium">{text.activeTeam}</p>
					<div class="mt-3 flex flex-wrap gap-3" aria-label={`${text.activeTeam} ${activity.members.length}`}>
						{#each activity.members.slice(0, 12) as member}
							<CompanyStaffChip {member} fallbackName={text.teamMember} defaultJobTitle={text.defaultJobTitle} />
						{/each}
						{#if activity.members.length > 12}
							<div class="bg-muted grid size-9 place-items-center rounded-full text-xs font-medium ring-2 ring-card">+{activity.members.length - 12}</div>
						{/if}
					</div>
				</div>
				<div class="flex gap-5 text-right text-sm">
					<div><p class="font-semibold tabular-nums">{activity.attendanceTotal}</p><p class="text-muted-foreground">{text.attendanceSignals}</p></div>
					<div><p class="font-semibold tabular-nums">{activity.workTotal}</p><p class="text-muted-foreground">{text.workSignals}</p></div>
				</div>
			</div>

			<div class="mt-9">
				<div class="mb-3 flex items-center justify-between gap-4">
					<h3 class="text-sm font-medium">{text.activityRhythm}</h3>
					<span class="text-muted-foreground text-xs">{formatActivityWindow(activityRhythmDays)}</span>
				</div>
				<div class="bg-muted/30 relative h-40 overflow-hidden rounded-lg" role="img" aria-label={text.activityRhythm}>
					<svg viewBox="0 0 720 160" preserveAspectRatio="none" class="absolute inset-0 size-full" aria-hidden="true">
						<defs>
							<linearGradient id="activity-area" x1="0" y1="0" x2="0" y2="1">
								<stop offset="0" stop-color="var(--color-blue-600)" stop-opacity="0.24" />
								<stop offset="1" stop-color="var(--color-blue-600)" stop-opacity="0" />
							</linearGradient>
						</defs>
						<polyline points={`12,148 ${pulsePoints} 708,148`} fill="url(#activity-area)" stroke="none" />
						<polyline points={pulsePoints} fill="none" stroke="var(--color-blue-600)" stroke-width="3" vector-effect="non-scaling-stroke" stroke-linecap="round" stroke-linejoin="round" />
					</svg>
				</div>
			</div>

			<div class="mt-7">
				<h3 class="mb-3 text-sm font-medium">{text.activityGrid}</h3>
				<div class="activity-grid" role="list" aria-label={text.activityGrid}>
					{#each activity.days as day}
						{@const intensity = companyActivityIntensity(day, activity.days)}
						<div
							role="listitem"
							class="bg-blue-600 aspect-square w-full rounded-[2px]"
							style={`opacity:${intensity === 0 ? 0.08 : 0.2 + intensity * 0.2}`}
							aria-label={activityLabel(day.date, day.attendanceCount, day.workCount)}
							title={activityLabel(day.date, day.attendanceCount, day.workCount)}
						></div>
					{/each}
				</div>
			</div>
		</div>

		<div class="grid content-start gap-8 bg-muted/15 p-5 sm:p-8">
			<div>
				<h3 class="text-sm font-medium">{text.workDistribution}</h3>
				<div class="mt-4 flex h-3 overflow-hidden rounded-full bg-muted" aria-label={text.workDistribution}>
					{#each activity.workStatuses as status}
						<div class={statusClass(status.status)} style={`width:${companyWorkStatusPercentage(status.count, activity.workStatuses)}%`} title={`${text.statusLabels[status.status]} ${status.count}`}></div>
					{/each}
				</div>
				<div class="mt-4 grid gap-2">
					{#each activity.workStatuses as status}
						<div class="flex items-center justify-between gap-3 text-sm">
							<span class="flex items-center gap-2"><span class={`size-2 rounded-full ${statusClass(status.status)}`}></span>{text.statusLabels[status.status]}</span>
							<span class="font-medium tabular-nums">{status.count}</span>
						</div>
					{/each}
				</div>
			</div>

			<div>
				<h3 class="text-sm font-medium">{text.recentWork}</h3>
				<div class="mt-3 grid gap-2">
					{#each activity.recentWork as work, index (`${work.memberSeed}-${work.date}-${index}`)}
						{@const member = activity.members.find((candidate) => candidate.seed === work.memberSeed)}
						{@const publicMember = member ?? { seed: work.memberSeed, surname: '' }}
						{@const task = publicFlowTask(work, publicMember)}
						{#snippet ownerChip()}
							<CompanyStaffChip member={publicMember} fallbackName={text.teamMember} defaultJobTitle={text.defaultJobTitle} compact />
						{/snippet}
						<FlowTaskBoardCard
							{task}
							businessFallback={text.otherBusiness}
							openTask={() => {}}
							isReadOnly
							isDraggable={false}
							isInteractive={false}
							{ownerChip}
						/>
					{/each}
				</div>
			</div>
		</div>
	</div>
</section>

<style>
	.activity-grid {
		display: grid;
		grid-auto-flow: column;
		grid-template-rows: repeat(7, minmax(0, 1fr));
		grid-auto-columns: minmax(0, 1fr);
		gap: clamp(0.1rem, 0.35vw, 0.3rem);
		width: 100%;
	}
</style>
