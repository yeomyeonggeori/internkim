// DayFlow 내부 mini calendar의 월/연도 picker DOM 동작을 관리합니다.
type MiniCalendarPickerLabels = {
	pickMonthAndYear: string;
	previousYear: string;
	nextYear: string;
	previousTwelveYears: string;
	nextTwelveYears: string;
};

export type DayFlowMiniCalendarPickerContext = {
	getStageElement: () => HTMLElement | null;
	getCurrentDate: () => Date;
	localeCode: () => string;
	labels: () => MiniCalendarPickerLabels;
	selectDate: (date: Date) => void;
};

type MiniCalendarPickerMode = 'month' | 'year';

export function installDayFlowMiniCalendarMonthPicker(context: DayFlowMiniCalendarPickerContext): () => void {
	let pickerElement: HTMLElement | null = null;
	let pickerMode: MiniCalendarPickerMode = 'month';
	let pickerYear = context.getCurrentDate().getFullYear();
	let pickerYearWindowStart = Math.floor(pickerYear / 12) * 12;
	const pickerControlActions = new WeakMap<HTMLElement, () => void>();

	const handleClick = (event: MouseEvent): void => {
		const target = event.target instanceof Element ? event.target : null;
		const stageElement = context.getStageElement();
		if (!target || !stageElement) {
			closePicker();
			return;
		}
		if (runPickerControlAction(target, event)) return;
		const label = target.closest<HTMLElement>('.df-mini-calendar-month-label[role="button"]');
		if (label && stageElement.contains(label)) {
			event.preventDefault();
			event.stopImmediatePropagation();
			openPicker(label);
			return;
		}
		if (target.closest('.calendar-mini-month-picker')) return;
		closePicker();
	};

	const handleKeydown = (event: KeyboardEvent): void => {
		const target = event.target instanceof Element ? event.target : null;
		const stageElement = context.getStageElement();
		if (event.key === 'Escape') {
			closePicker();
			return;
		}
		if (target && (event.key === 'Enter' || event.key === ' ') && runPickerControlAction(target, event)) return;
		if (!target || !stageElement || (event.key !== 'Enter' && event.key !== ' ')) return;
		const label = target.closest<HTMLElement>('.df-mini-calendar-month-label[role="button"]');
		if (!label || !stageElement.contains(label)) return;
		event.preventDefault();
		openPicker(label);
	};

	function openPicker(label: HTMLElement): void {
		const visibleMonth = miniCalendarVisibleMonth(label) ?? currentMonth();
		pickerMode = 'month';
		pickerYear = visibleMonth.getFullYear();
		pickerYearWindowStart = Math.floor(pickerYear / 12) * 12;
		renderPicker(label);
	}

	function closePicker(): void {
		pickerElement?.remove();
		pickerElement = null;
	}

	function renderPicker(label: HTMLElement): void {
		const stageElement = context.getStageElement();
		if (!stageElement) return;
		if (!pickerElement) {
			pickerElement = document.createElement('div');
			pickerElement.className = 'calendar-mini-month-picker';
			pickerElement.setAttribute('role', 'dialog');
			pickerElement.setAttribute('aria-label', context.labels().pickMonthAndYear);
			stageElement.appendChild(pickerElement);
		}
		pickerElement.style.left = `${pickerLeft(stageElement, label)}px`;
		pickerElement.style.top = `${pickerTop(stageElement, label)}px`;
		pickerElement.replaceChildren(pickerHeader(label), pickerGrid(label));
	}

	function pickerHeader(label: HTMLElement): HTMLElement {
		const header = document.createElement('header');
		header.className = 'calendar-mini-month-picker-header';
		header.appendChild(pickerNavButton(previousPickerLabel(), () => shiftPickerWindow(label, -1), '‹'));
		header.appendChild(pickerTitleButton(label));
		header.appendChild(pickerNavButton(nextPickerLabel(), () => shiftPickerWindow(label, 1), '›'));
		return header;
	}

	function pickerTitleButton(label: HTMLElement): HTMLButtonElement {
		const button = document.createElement('button');
		button.type = 'button';
		button.className = 'calendar-mini-month-picker-title';
		button.textContent = pickerMode === 'month' ? String(pickerYear) : `${pickerYearWindowStart}-${pickerYearWindowStart + 11}`;
		const action = () => {
			pickerMode = pickerMode === 'month' ? 'year' : 'month';
			if (pickerMode === 'year') pickerYearWindowStart = Math.floor(pickerYear / 12) * 12;
			renderPicker(label);
		};
		pickerControlActions.set(button, action);
		button.addEventListener('click', action);
		return button;
	}

	function pickerNavButton(label: string, action: () => void, text: string): HTMLButtonElement {
		const button = document.createElement('button');
		button.type = 'button';
		button.className = 'calendar-mini-month-picker-nav';
		button.setAttribute('aria-label', label);
		button.textContent = text;
		pickerControlActions.set(button, action);
		button.addEventListener('click', action);
		return button;
	}

	function pickerGrid(label: HTMLElement): HTMLElement {
		const grid = document.createElement('div');
		grid.className = 'calendar-mini-month-picker-grid';
		if (pickerMode === 'month') {
			monthLabels().forEach((monthLabel, monthIndex) => {
				grid.appendChild(
					pickerOption(monthLabel, isActiveMonth(monthIndex), isTodayMonth(monthIndex), () => selectMonth(label, monthIndex))
				);
			});
			return grid;
		}
		for (let index = 0; index < 12; index += 1) {
			const year = pickerYearWindowStart + index;
			grid.appendChild(pickerOption(String(year), year === pickerYear, year === new Date().getFullYear(), () => selectYear(label, year)));
		}
		return grid;
	}

	function pickerOption(text: string, isActive: boolean, isToday: boolean, action: () => void): HTMLButtonElement {
		const button = document.createElement('button');
		button.type = 'button';
		button.className = 'calendar-mini-month-picker-option';
		if (isActive) button.classList.add('active-picker-option');
		if (isToday) button.classList.add('today-picker-option');
		button.textContent = text;
		pickerControlActions.set(button, action);
		button.addEventListener('click', action);
		return button;
	}

	function runPickerControlAction(target: Element, event: Event): boolean {
		const control = target.closest<HTMLElement>('.calendar-mini-month-picker button');
		if (!control || !pickerElement?.contains(control)) return false;
		const action = pickerControlActions.get(control);
		if (!action) return false;
		event.preventDefault();
		event.stopImmediatePropagation();
		action();
		return true;
	}

	function shiftPickerWindow(label: HTMLElement, delta: -1 | 1): void {
		if (pickerMode === 'month') pickerYear += delta;
		else pickerYearWindowStart += delta * 12;
		renderPicker(label);
	}

	function selectMonth(label: HTMLElement, monthIndex: number): void {
		const currentDate = context.getCurrentDate();
		const daysInMonth = new Date(pickerYear, monthIndex + 1, 0).getDate();
		const day = Math.min(currentDate.getDate(), daysInMonth);
		const selectedDate = new Date(pickerYear, monthIndex, day, 12, 0, 0, 0);
		const stageElement = context.getStageElement();
		closePicker();
		updateMiniCalendarMonthLabel(label, selectedDate, context.localeCode());
		context.selectDate(selectedDate);
		requestAnimationFrame(() => updateCurrentMiniCalendarMonthLabel(stageElement, selectedDate, context.localeCode()));
		requestAnimationFrame(() =>
			requestAnimationFrame(() => updateCurrentMiniCalendarMonthLabel(stageElement, selectedDate, context.localeCode()))
		);
	}

	function selectYear(label: HTMLElement, year: number): void {
		pickerYear = year;
		pickerMode = 'month';
		renderPicker(label);
	}

	function previousPickerLabel(): string {
		return pickerMode === 'month' ? context.labels().previousYear : context.labels().previousTwelveYears;
	}

	function nextPickerLabel(): string {
		return pickerMode === 'month' ? context.labels().nextYear : context.labels().nextTwelveYears;
	}

	function monthLabels(): string[] {
		return Array.from({ length: 12 }, (_, monthIndex) =>
			new Date(2000, monthIndex, 1).toLocaleDateString(context.localeCode(), { month: 'short' })
		);
	}

	function isActiveMonth(monthIndex: number): boolean {
		const currentDate = context.getCurrentDate();
		return pickerYear === currentDate.getFullYear() && monthIndex === currentDate.getMonth();
	}

	function isTodayMonth(monthIndex: number): boolean {
		const today = new Date();
		return pickerYear === today.getFullYear() && monthIndex === today.getMonth();
	}

	function currentMonth(): Date {
		const currentDate = context.getCurrentDate();
		return new Date(currentDate.getFullYear(), currentDate.getMonth(), 1);
	}

	document.addEventListener('click', handleClick, { capture: true });
	document.addEventListener('keydown', handleKeydown, { capture: true });
	return () => {
		document.removeEventListener('click', handleClick, { capture: true });
		document.removeEventListener('keydown', handleKeydown, { capture: true });
		closePicker();
	};
}

function pickerLeft(stageElement: HTMLElement, label: HTMLElement): number {
	const stageRectangle = stageElement.getBoundingClientRect();
	const labelRectangle = label.getBoundingClientRect();
	return Math.max(8, labelRectangle.left - stageRectangle.left + labelRectangle.width / 2 - 110);
}

function pickerTop(stageElement: HTMLElement, label: HTMLElement): number {
	const stageRectangle = stageElement.getBoundingClientRect();
	const labelRectangle = label.getBoundingClientRect();
	return labelRectangle.bottom - stageRectangle.top + 8;
}

function miniCalendarVisibleMonth(label: HTMLElement): Date | null {
	const text = label.textContent?.trim() ?? '';
	const koreanMatch = /(\d{4})년\s*(\d{1,2})월/.exec(text);
	if (koreanMatch) return new Date(Number(koreanMatch[1]), Number(koreanMatch[2]) - 1, 1);
	const parsedDate = new Date(`${text} 1`);
	if (Number.isNaN(parsedDate.getTime())) return null;
	return new Date(parsedDate.getFullYear(), parsedDate.getMonth(), 1);
}

function updateCurrentMiniCalendarMonthLabel(stageElement: HTMLElement | null, date: Date, localeCode: string): void {
	const label = stageElement?.querySelector<HTMLElement>('.df-mini-calendar-month-label[role="button"]');
	if (!label) return;
	updateMiniCalendarMonthLabel(label, date, localeCode);
}

function updateMiniCalendarMonthLabel(label: HTMLElement, date: Date, localeCode: string): void {
	label.textContent = date.toLocaleDateString(localeCode, {
		year: 'numeric',
		month: 'long'
	});
}
