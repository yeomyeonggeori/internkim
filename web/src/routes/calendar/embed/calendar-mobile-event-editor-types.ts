import type { Event as DayFlowEvent } from '@dayflow/core';
import type { CalendarLocaleText } from '../text';

export type MobileEventEditorPersistenceContext = {
	saveEvent: (event: DayFlowEvent) => void | Promise<void>;
};

export type MobileEventEditorLocaleContext = {
	getText: () => CalendarLocaleText;
};

export type MobileEventEditorCalendar = {
	id: string;
	name: string;
};

export const mobileEventEditorPersistenceContextKey = Symbol.for('internkim.calendar.mobileEventEditorPersistence');
export const mobileEventEditorLocaleContextKey = Symbol.for('internkim.calendar.mobileEventEditorLocale');
