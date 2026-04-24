import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

export function cn(...inputs: ClassValue[]) {
	return twMerge(clsx(inputs));
}

export type WithElementRef<T, E extends HTMLElement = HTMLElement> = T & {
	ref?: E | null;
};

export type WithoutChildren<T> = T extends object ? Omit<T, 'children'> : T;

export type WithoutChildrenOrChild<T> = T extends object ? Omit<T, 'child' | 'children'> : T;
