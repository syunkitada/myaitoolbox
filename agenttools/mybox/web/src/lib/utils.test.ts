import { describe, it, expect } from 'vitest'
import { hasCRLF, normalizeLineEndings } from './utils'

describe('normalizeLineEndings', () => {
  it('converts CRLF to LF', () => {
    expect(normalizeLineEndings('a\r\nb\r\nc')).toBe('a\nb\nc')
  })

  it('converts lone CR to LF', () => {
    expect(normalizeLineEndings('a\rb\rc')).toBe('a\nb\nc')
  })

  it('leaves LF-only text unchanged', () => {
    expect(normalizeLineEndings('a\nb\nc')).toBe('a\nb\nc')
  })

  it('normalizes the shebang line', () => {
    expect(normalizeLineEndings('#!/bin/bash\r\necho hi\r\n')).toBe('#!/bin/bash\necho hi\n')
  })
})

describe('hasCRLF', () => {
  it('detects CRLF', () => {
    expect(hasCRLF('a\r\nb')).toBe(true)
  })

  it('returns false for LF-only or empty text', () => {
    expect(hasCRLF('a\nb')).toBe(false)
    expect(hasCRLF('a\rb')).toBe(false)
    expect(hasCRLF('')).toBe(false)
  })
})