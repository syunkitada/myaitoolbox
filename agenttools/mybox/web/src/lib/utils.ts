import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function normalizeLineEndings(text: string): string {
  return text.replace(/\r\n?/g, '\n')
}

export function hasCRLF(text: string): boolean {
  return /\r\n/.test(text)
}
