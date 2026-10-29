import { z } from 'zod';

export const listPageSize = 50;
export const listPageSizeLimit = 100;

export function listLimitSchema(noun: string) {
  return z.number().int().min(1).max(listPageSizeLimit).describe(`Maximum ${noun} to return. Defaults to ${listPageSize}.`);
}

export function listCursorSchema(toolName: string) {
  return z.string().min(1).describe(`Pagination cursor from a previous ${toolName} result. Omit on the first call.`);
}

export function listTotalCountSchema(noun: string) {
  return z.number().int().describe(`How many ${noun} match the filters across every page.`);
}

export function listNextCursorSchema(noun: string) {
  return z.string().describe(`Cursor for the next page. Absent when no more ${noun} match.`);
}
