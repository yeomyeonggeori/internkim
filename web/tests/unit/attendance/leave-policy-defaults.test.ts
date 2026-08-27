import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import {
	defaultLeaveTypes,
	systemLeaveTypeKind
} from '../../../src/lib/attendance/leave-policy-defaults';

const goSource = readFileSync('../internal/admind/attendance_leave_policy_defaults.go', 'utf8');

type GoLeaveType = {
	id: string;
	systemKind: string;
	name: string;
	paid: boolean;
	balanceMode: string;
	grantCadence: string;
	grantAmountMilliDays: number;
	expiryMode: string;
	allowedUnits: string[];
	includeInSummary: boolean;
	sortOrder: number;
};

function topLevelArguments(call: string): string[] {
	const found: string[] = [];
	let depth = 0;
	let quoted = false;
	let current = '';
	for (let index = 0; index < call.length; index += 1) {
		const character = call[index];
		if (quoted) {
			current += character;
			if (character === '"' && call[index - 1] !== '\\') quoted = false;
			continue;
		}
		if (character === '"') {
			quoted = true;
			current += character;
			continue;
		}
		if (character === '(') depth += 1;
		if (character === ')') depth -= 1;
		if (character === ',' && depth === 0) {
			found.push(current.trim());
			current = '';
			continue;
		}
		current += character;
	}
	if (current.trim()) found.push(current.trim());
	return found;
}

function goConstant(name: string): number {
	const declaration = new RegExp(`const\\s+${name}\\s*=\\s*(-?\\d+)`).exec(goSource);
	if (!declaration) throw new Error(`${name} is not declared in the Go defaults`);
	return Number(declaration[1]);
}

function goUnitHelper(name: string): string[] {
	const body = new RegExp(`func\\s+${name}\\(\\)\\s*\\[\\]string\\s*\\{[^}]*return\\s*\\[\\]string\\{([^}]*)\\}`, 's').exec(goSource);
	if (!body) throw new Error(`${name} is not declared in the Go defaults`);
	return [...body[1].matchAll(/"([^"]*)"/g)].map((unit) => unit[1]);
}

function goString(argument: string): string {
	const quoted = /^"(.*)"$/s.exec(argument);
	if (!quoted) throw new Error(`expected a Go string literal, read ${argument}`);
	return quoted[1];
}

function goNumber(argument: string): number {
	if (/^-?\d+$/.test(argument)) return Number(argument);
	return goConstant(argument);
}

function goBoolean(argument: string): boolean {
	if (argument !== 'true' && argument !== 'false') {
		throw new Error(`expected a Go boolean, read ${argument}`);
	}
	return argument === 'true';
}

function goUnits(argument: string): string[] {
	const call = /^([A-Za-z0-9_]+)\(\)$/.exec(argument);
	if (!call) throw new Error(`expected a Go unit helper call, read ${argument}`);
	return goUnitHelper(call[1]);
}

function goLeaveTypes(): GoLeaveType[] {
	const calls = [...goSource.matchAll(/defaultAttendanceLeaveType\(([^\n]*)\),\n/g)];
	return calls.map((call) => {
		const argumentValues = topLevelArguments(call[1]);
		if (argumentValues.length !== 11) {
			throw new Error(`expected 11 arguments, read ${argumentValues.length} in ${call[1]}`);
		}
		return {
			id: goString(argumentValues[0]),
			systemKind: goString(argumentValues[1]),
			name: goString(argumentValues[2]),
			paid: goBoolean(argumentValues[3]),
			balanceMode: goString(argumentValues[4]),
			grantCadence: goString(argumentValues[5]),
			grantAmountMilliDays: goNumber(argumentValues[6]),
			expiryMode: goString(argumentValues[7]),
			allowedUnits: goUnits(argumentValues[8]),
			includeInSummary: goBoolean(argumentValues[9]),
			sortOrder: goNumber(argumentValues[10])
		};
	});
}

describe('the central plane answers the device catalogue of leave types', () => {
	test('the Go defaults are readable and are the list this test compares against', () => {
		const fromGo = goLeaveTypes();
		expect(fromGo.length).not.toBe(0);
		expect(fromGo[0].id).toBe('annual');
	});

	test('every default leave type matches the one internal/admind declares', () => {
		expect(defaultLeaveTypes()).toEqual(
			goLeaveTypes().map((leaveType) => ({
				...leaveType,
				carryoverEnabled: false,
				isActive: true,
				isSystem: true
			}))
		);
	});

	test('a system kind is the one the device gives that id', () => {
		for (const leaveType of goLeaveTypes()) {
			expect(systemLeaveTypeKind(leaveType.id)).toBe(leaveType.systemKind);
		}
		expect(systemLeaveTypeKind('custom-whatever')).toBeUndefined();
	});
});
