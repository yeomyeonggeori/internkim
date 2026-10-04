import { describe, expect, test } from 'bun:test';
import { localDatabaseSelected } from '../../../supabase/scripts/local-database-selection';

const ports = { api: 56421, database: 56422 };
const sql = 'postgresql://fixture:fixture@127.0.0.1:56422/postgres';

describe('owned local SQL selection before a connection', () => {
	test('refuses an isolated API without its explicit SQL URL', () => {
		expect(() => localDatabaseSelected({ SUPABASE_URL: 'http://127.0.0.1:56421' }, ports)).toThrow('SUPABASE_DB_URL is required');
	});
	test('refuses the canonical SQL port beside an isolated API', () => {
		expect(() => localDatabaseSelected({ SUPABASE_URL: 'http://127.0.0.1:56421', SUPABASE_DB_URL: 'postgresql://fixture:fixture@127.0.0.1:54322/postgres' }, ports)).toThrow('configured ports');
	});
	test('accepts API and SQL selected from this checkout’s local status', () => {
		expect(localDatabaseSelected({ SUPABASE_URL: 'http://127.0.0.1:56421', SUPABASE_DB_URL: sql }, ports)).toBe(sql);
	});
	test('keeps the documented direct local workflow on its configured port', () => {
		expect(localDatabaseSelected({}, { api: 54321, database: 54322 })).toBe('postgresql://supabase_admin:postgres@127.0.0.1:54322/postgres');
	});
	test('refuses remote SQL before credentials or a connection can escape', () => {
		expect(() => localDatabaseSelected({ SUPABASE_DB_URL: 'postgresql://fixture:fixture@database.example.com:56422/postgres' }, ports)).toThrow('configured ports');
	});
});
