import { error, json } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import {
	attendanceWorkModeFromDevice,
	attendanceWorkPolicyFromDevice,
	companyHolidaysFromDevice,
	InvalidAttendanceWorkModeError,
	InvalidAttendanceWorkPolicyError,
	InvalidCompanyHolidayError,
	mergeCompanyHolidays,
	saveAttendanceReconciliationSettings
} from '$lib/server/attendance-work-policy-reconcile';
import type { RequestHandler } from './$types';

type ReconcileRequest = {
	platform?: unknown;
	workMode?: unknown;
	workCalendar?: unknown;
	workPolicy?: unknown;
	companyHolidays?: unknown;
	from?: unknown;
	to?: unknown;
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const asked = (await request.json().catch(() => ({}))) as ReconcileRequest;
	const from = moment(asked.from, 'from');
	const to = moment(asked.to, 'to');
	if (from >= to) error(400, 'the window ends before it begins');
	askedWorkMode(asked.workMode);
	const workPolicy = askedWorkPolicy(asked.workPolicy);
	const companyHolidays = askedCompanyHolidays(asked.companyHolidays);
	if (workPolicy !== undefined) {
		await saveAttendanceReconciliationSettings(client, companyID, workPolicy);
	}
	if (companyHolidays !== undefined) {
		await mergeCompanyHolidays(client, companyID, companyHolidays);
	}

	// A device still sends its own clocks here and they are ignored. Attendance
	// is kept in one place; this route used to make that place match a device,
	// which deleted every clock made in a browser. The work policy stays because
	// that request is still the only way it reaches the record.
	return json({ added: 0, removed: 0, refused: [], rejected: [] });
};

function askedWorkPolicy(offered: unknown) {
	try {
		return attendanceWorkPolicyFromDevice(offered);
	} catch (thrown) {
		if (!(thrown instanceof InvalidAttendanceWorkPolicyError)) throw thrown;
		error(400, thrown.message);
	}
}

function askedCompanyHolidays(offered: unknown) {
	try {
		return companyHolidaysFromDevice(offered);
	} catch (thrown) {
		if (!(thrown instanceof InvalidCompanyHolidayError)) throw thrown;
		error(400, thrown.message);
	}
}

function askedWorkMode(offered: unknown) {
	try {
		return attendanceWorkModeFromDevice(offered);
	} catch (thrown) {
		if (!(thrown instanceof InvalidAttendanceWorkModeError)) throw thrown;
		error(400, thrown.message);
	}
}

function moment(offered: unknown, named: string): string {
	if (typeof offered !== 'string' || Number.isNaN(Date.parse(offered))) error(400, `${named} must be a time`);
	return new Date(offered).toISOString();
}
