import { apiErrorMessage } from './admin-api';
import { ToolRefused } from '$lib/public-api-call';
import {
	createCompanyHoliday,
	deleteCompanyHoliday,
	fetchCompanyHolidays,
	updateCompanyHoliday
} from './company-holidays-api';
import type { AdminPageText, CompanyHoliday, CompanyHolidayInput } from './admin-types';

export type CompanyHolidayDraft = CompanyHolidayInput & {
	id?: string;
};

function todayDate(): string {
	const now = new Date();
	const year = now.getFullYear();
	const month = String(now.getMonth() + 1).padStart(2, '0');
	const day = String(now.getDate()).padStart(2, '0');
	return `${year}-${month}-${day}`;
}

function sortCompanyHolidays(holidays: CompanyHoliday[]): CompanyHoliday[] {
	return [...holidays].sort((left, right) =>
		left.date.localeCompare(right.date) || left.title.localeCompare(right.title)
	);
}

export class CompanyHolidaySettingsState {
	holidays = $state<CompanyHoliday[]>([]);
	draft = $state<CompanyHolidayDraft | null>(null);
	message = $state('');
	loadError = $state('');
	hasLoadedHolidays = $state(false);
	isLoading = $state(false);
	isSaving = $state(false);
	validationAttempted = $state(false);

	private adminBaseURL = '';
	private text: AdminPageText['companyHolidays'] | null = null;
	private hasLoaded = false;

	sync(adminBaseURL: string, text: AdminPageText['companyHolidays']): void {
		this.adminBaseURL = adminBaseURL;
		this.text = text;
		if (this.hasLoaded) return;
		this.hasLoaded = true;
		void this.load();
	}

	startCreate(): void {
		this.draft = { title: '', date: todayDate(), recursAnnually: false };
		this.validationAttempted = false;
		this.message = '';
	}

	startEdit(holiday: CompanyHoliday): void {
		this.draft = {
			id: holiday.id,
			title: holiday.title,
			date: holiday.date,
			recursAnnually: holiday.recursAnnually
		};
		this.validationAttempted = false;
		this.message = '';
	}

	updateDraft(change: Partial<CompanyHolidayInput>): void {
		if (!this.draft) return;
		this.draft = { ...this.draft, ...change };
	}

	cancel(): void {
		this.draft = null;
		this.validationAttempted = false;
		this.message = '';
	}

	async save(): Promise<boolean> {
		if (!this.draft || !this.text) return false;
		this.validationAttempted = true;
		if (!this.draft.title.trim() || !this.draft.date) return false;
		this.isSaving = true;
		this.message = '';
		const input: CompanyHolidayInput = {
			title: this.draft.title.trim(),
			date: this.draft.date,
			recursAnnually: this.draft.recursAnnually
		};
		try {
			const saved = this.draft.id
				? await updateCompanyHoliday(this.draft.id, input)
				: await createCompanyHoliday(input);
			this.holidays = sortCompanyHolidays([
				...this.holidays.filter((holiday) => holiday.id !== saved.id),
				saved
			]);
			this.draft = null;
			this.validationAttempted = false;
			this.message = this.text.saveSuccess;
			return true;
		} catch (error) {
			this.message = apiErrorMessage(error, this.text.saveError);
			return false;
		} finally {
			this.isSaving = false;
		}
	}

	async remove(): Promise<boolean> {
		if (!this.draft?.id || !this.text) return false;
		this.isSaving = true;
		this.message = '';
		try {
			await deleteCompanyHoliday(this.draft.id);
			this.holidays = this.holidays.filter((holiday) => holiday.id !== this.draft?.id);
			this.draft = null;
			this.validationAttempted = false;
			this.message = this.text.removeSuccess;
			return true;
		} catch (error) {
			this.message = apiErrorMessage(error, this.text.removeError);
			return false;
		} finally {
			this.isSaving = false;
		}
	}

	private async load(): Promise<void> {
		if (!this.text) return;
		this.isLoading = true;
		this.loadError = '';
		try {
			const response = await fetchCompanyHolidays();
			this.holidays = sortCompanyHolidays(response.holidays ?? []);
			this.hasLoadedHolidays = true;
		} catch (error) {
			if (error instanceof ToolRefused && (error.status === 401 || error.status === 403)) {
				this.holidays = [];
				this.draft = null;
				this.hasLoadedHolidays = false;
				this.validationAttempted = false;
				this.message = '';
			}
			this.loadError = apiErrorMessage(error, this.text.loadError);
		} finally {
			this.isLoading = false;
		}
	}
}
