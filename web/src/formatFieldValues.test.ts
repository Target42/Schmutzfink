import { describe, expect, it } from 'vitest'
import { formatFieldValues } from './pages/FieldInputs.tsx'
import type { CustomField } from './types.ts'

const defs: CustomField[] = [
  {
    id: '1',
    key: 'gangname',
    label: 'Gangname',
    type: 'text',
    options: [],
    required: false,
    sort_order: 0,
  },
  {
    id: '2',
    key: 'status',
    label: 'Status',
    type: 'select',
    options: ['offen'],
    required: false,
    sort_order: 1,
  },
]

describe('formatFieldValues', () => {
  it('skips empty values and unknown keys', () => {
    expect(formatFieldValues(defs, { gangname: 'Oz', status: '' })).toEqual(['Gangname: Oz'])
  })
  it('returns nothing without values', () => {
    expect(formatFieldValues(defs)).toEqual([])
  })
})
