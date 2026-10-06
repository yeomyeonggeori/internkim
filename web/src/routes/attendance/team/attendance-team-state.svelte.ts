import { getContext, setContext } from 'svelte';
import { invokeTool, ToolRefused } from '$lib/public-api-call';
import {
	attendanceTeamPageSchema,
	type AttendanceTeamPage,
	type AttendanceTeamPageInput
} from '$lib/attendance/team-page';

type Team = AttendanceTeamPage['teams'][number];
type Member = AttendanceTeamPage['members'][number];

export class AttendanceTeamState {
	teams = $state<Team[]>([]);
	members = $state<Member[]>([]);
	teamOffset = $state(0);
	teamTotal = $state(0);
	memberOffset = $state(0);
	memberTotal = $state(0);
	selectedTeamKey = $state('');
	searchText = $state('');
	locationFilter = $state('');
	isLoadingTeams = $state(false);
	isLoadingMembers = $state(false);
	error = $state('');
	memberError = $state('');
	companyName = $state('');
	companySummary = $state<AttendanceTeamPage['companySummary']>();
	timeZone = $state('');
	serverTime = $state('');
	revision = $state(0);
	private started = false;
	private disposed = false;
	private teamsSequence = 0;
	private membersSequence = 0;

	start(): void {
		if (this.started || this.disposed) return;
		this.started = true;
		void this.loadTeams();
	}

	dispose(): void {
		this.disposed = true;
		this.teamsSequence += 1;
		this.membersSequence += 1;
	}

	async refresh(resetPages = false): Promise<void> {
		if (!this.started || this.disposed) return;
		await Promise.all([
			this.loadTeams(resetPages ? 0 : this.teamOffset),
			this.selectedTeamKey ? this.loadMembers(resetPages ? 0 : this.memberOffset) : Promise.resolve()
		]);
		if (!this.disposed) this.revision += 1;
	}

	async loadTeams(offset = this.teamOffset): Promise<void> {
		if (this.disposed) return;
		const sequence = ++this.teamsSequence;
		if (offset !== this.teamOffset) this.teams = [];
		this.teamOffset = offset;
		this.isLoadingTeams = true;
		this.error = '';
		try {
			const page = await this.read({ pageKind: 'teams', teamOffset: offset, teamLimit: 6 });
			if (this.disposed || sequence !== this.teamsSequence) return;
			if (page.teamTotal > 0 && offset >= page.teamTotal) {
				void this.loadTeams(Math.floor((page.teamTotal - 1) / 6) * 6);
				return;
			}
			this.companyName = page.companyName ?? '';
			this.companySummary = page.companySummary;
			this.teams = page.teams;
			this.teamTotal = page.teamTotal;
			this.timeZone = page.timeZone;
			this.serverTime = page.serverTime;
			if (this.selectedTeamKey && !page.teams.some((team) => team.teamKey === this.selectedTeamKey)) {
				this.clearSelection();
			}
		} catch (error) {
			if (this.disposed || sequence !== this.teamsSequence) return;
			if (error instanceof ToolRefused && [401, 403].includes(error.status)) {
				this.teams = [];
				this.teamTotal = 0;
				this.companySummary = undefined;
				this.clearSelection();
			}
			this.error = error instanceof Error ? error.message : String(error);
		} finally {
			if (!this.disposed && sequence === this.teamsSequence) this.isLoadingTeams = false;
		}
	}

	selectTeam(teamKey: string): void {
		this.selectedTeamKey = teamKey;
		this.memberOffset = 0;
		this.searchText = '';
		this.locationFilter = '';
		this.members = [];
		void this.loadMembers(0);
	}

	clearSelection(): void {
		this.membersSequence += 1;
		this.isLoadingMembers = false;
		this.selectedTeamKey = '';
		this.members = [];
		this.memberTotal = 0;
		this.memberOffset = 0;
		this.memberError = '';
	}

	filter(search: string, location: string): void {
		this.members = [];
		this.memberTotal = 0;
		this.searchText = search;
		this.locationFilter = location;
		this.memberOffset = 0;
		void this.loadMembers(0);
	}

	async loadMembers(offset = this.memberOffset): Promise<void> {
		if (this.disposed || !this.selectedTeamKey) return;
		const sequence = ++this.membersSequence;
		if (offset !== this.memberOffset) this.members = [];
		this.memberOffset = offset;
		this.isLoadingMembers = true;
		this.memberError = '';
		try {
			const page = await this.read({
				pageKind: 'members', selectedTeamKey: this.selectedTeamKey,
				memberOffset: offset, memberLimit: 24,
				searchText: this.searchText.trim(), locationFilter: this.locationFilter
			});
			if (this.disposed || sequence !== this.membersSequence) return;
			if (page.memberTotal > 0 && offset >= page.memberTotal) {
				void this.loadMembers(Math.floor((page.memberTotal - 1) / 24) * 24);
				return;
			}
			this.members = page.members;
			this.memberTotal = page.memberTotal;
			this.timeZone = page.timeZone;
			this.serverTime = page.serverTime;
		} catch (error) {
			if (this.disposed || sequence !== this.membersSequence) return;
			if (error instanceof ToolRefused && [401, 403].includes(error.status)) {
				this.members = [];
				this.memberTotal = 0;
			}
			this.memberError = error instanceof Error ? error.message : String(error);
		} finally {
			if (!this.disposed && sequence === this.membersSequence) this.isLoadingMembers = false;
		}
	}

	private async read(input: AttendanceTeamPageInput): Promise<AttendanceTeamPage> {
		return attendanceTeamPageSchema.parse(await invokeTool('attendance_team_dashboard_get', input));
	}
}

const teamStateKey = Symbol('attendance-team-state');

export function setAttendanceTeamState(state: AttendanceTeamState): void {
	setContext(teamStateKey, state);
}

export function getAttendanceTeamState(): AttendanceTeamState {
	const state = getContext<AttendanceTeamState | undefined>(teamStateKey);
	if (!state) throw new Error('AttendanceTeamState not provided');
	return state;
}
