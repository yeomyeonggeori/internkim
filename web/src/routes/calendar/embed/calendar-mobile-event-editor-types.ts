import type { Event as DayFlowEvent } from '@dayflow/core';
import type { CalendarLocaleText } from '../text';
import type { CalendarParticipant } from './calendar-participants';

export type MobileEventEditorPersistenceContext = {
	saveEvent: (event: DayFlowEvent) => void | Promise<void>;
};

export type MobileEventEditorLocaleContext = {
	getText: () => CalendarLocaleText;
};

export type MobileEventEditorParticipantsContext = {
	getCandidates: () => CalendarParticipant[];
};

export type MobileEventEditorActivationContext = {
	getActiveEventID: () => string | null;
	getStageElement: () => HTMLElement | null;
	clearActiveEvent: (eventID: string) => void;
};

export type MobileEventEditorCalendar = {
	id: string;
	name: string;
};

export const mobileEventEditorPersistenceContextKey = Symbol.for('internkim.calendar.mobileEventEditorPersistence');
export const mobileEventEditorLocaleContextKey = Symbol.for('internkim.calendar.mobileEventEditorLocale');
export const mobileEventEditorParticipantsContextKey = Symbol.for('internkim.calendar.mobileEventEditorParticipants');
export const mobileEventEditorActivationContextKey = Symbol.for('internkim.calendar.mobileEventEditorActivation');
