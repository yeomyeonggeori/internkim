import { describe, expect, test } from 'bun:test';

import {
	defaultEventReminderLead,
	eventReminderLeadLabel,
	eventReminderLeadOf,
	eventReminderLeads,
	isEventReminderLead
} from '../../../src/lib/calendar/event-reminder-lead';

describe('event reminder leads', () => {
	test('offers a lead the default belongs to, in rising order', () => {
		expect(eventReminderLeads).toEqual([10, 30, 60, 120, 180, 360, 720, 1440, 2880]);
		expect(isEventReminderLead(defaultEventReminderLead)).toBe(true);
		expect([...eventReminderLeads].sort((left, right) => left - right)).toEqual([...eventReminderLeads]);
	});

	test('spells a lead in the largest unit that divides it', () => {
		expect(eventReminderLeadLabel(30, 'ko', '{amount} 전')).toBe('30분 전');
		expect(eventReminderLeadLabel(60, 'ko', '{amount} 전')).toBe('1시간 전');
		expect(eventReminderLeadLabel(2880, 'ko', '{amount} 전')).toBe('2일 전');
		expect(eventReminderLeadLabel(60, 'en', '{amount} before')).toBe('1 hour before');
		expect(eventReminderLeadLabel(120, 'en', '{amount} before')).toBe('2 hours before');
		expect(eventReminderLeadLabel(1440, 'en', '{amount} before')).toBe('1 day before');
	});

	test('reads a stored lead and refuses what the column cannot hold', () => {
		expect(eventReminderLeadOf(30)).toBe(30);
		expect(eventReminderLeadOf(null)).toBeNull();
		expect(eventReminderLeadOf(0)).toBeNull();
		expect(eventReminderLeadOf(-30)).toBeNull();
		expect(eventReminderLeadOf(12.5)).toBeNull();
	});
});
