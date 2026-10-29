import { z } from 'zod';

export const listPageSize = 50;
export const listPageSizeLimit = 100;

export function listLimitSchema(noun: string) {
  return z.number().int().min(1).max(listPageSizeLimit).describe(`Maximum ${noun} to return. Defaults to ${listPageSize}.`);
}

export function listCursorSchema(toolName: string) {
  return z.string().min(1).describe(`Pagination cursor from a previous ${toolName} result. Omit on the first call.`);
}

export function nextCursorSchema(noun: string) {
  return z.string().describe(`Cursor for the next page. Absent when no more ${noun} match.`);
}

export function moreRemainSentence(noun: string, absentOne: string): string {
  return `A nextCursor in the result means more ${noun} match than this page holds: read the next page before concluding that ${absentOne}.`;
}
