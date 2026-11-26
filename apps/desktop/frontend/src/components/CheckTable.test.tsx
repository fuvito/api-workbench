import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'

vi.mock('@/lib/bridge', () => ({ getCheckTable: vi.fn() }))

import * as bridge from '@/lib/bridge'
import { CheckTable } from './CheckTable'

const b = vi.mocked(bridge)

beforeEach(() => {
  vi.resetAllMocks()
  b.getCheckTable.mockResolvedValue([
    { id: 'CS-RESP-005', title: 'Required body field missing', section: '1.1 Failures', severity: 'failure', cells: [
      { label: 'What it means', text: 'The body lacks a property listed as `required`.' },
    ] },
    { id: 'CS-GAP-001', title: 'Formats outside the supported list', section: '3. Known gaps', cells: [
      { label: 'What is missed', text: '`format: duration` is not checked.' },
    ] },
  ])
})

const search = () => screen.getByRole('searchbox', { name: 'Search the checks' })

describe('CheckTable', () => {
  it('lists every row with its severity, which a gap has none of', async () => {
    render(<CheckTable />)
    expect(await screen.findByText('CS-RESP-005')).toBeInTheDocument()
    expect(screen.getByText('CS-GAP-001')).toBeInTheDocument()
    expect(screen.getByText('failure')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('2 of 2')
  })

  it('searches every word in any column, in any case, through the Markdown marks', async () => {
    render(<CheckTable />)
    await screen.findByText('CS-RESP-005')
    fireEvent.change(search(), { target: { value: 'FORMAT: duration' } })
    expect(screen.queryByText('CS-RESP-005')).not.toBeInTheDocument()
    expect(screen.getByText('CS-GAP-001')).toBeInTheDocument()
    fireEvent.change(search(), { target: { value: 'cs-resp-005' } })
    expect(screen.getByRole('status')).toHaveTextContent('1 of 2')
    fireEvent.change(search(), { target: { value: 'required duration' } })
    expect(screen.getByText(/Nothing in the list matches/)).toBeInTheDocument()
  })

  it('shows one section', async () => {
    render(<CheckTable />)
    await screen.findByText('CS-RESP-005')
    fireEvent.change(screen.getByRole('combobox', { name: 'Section' }), { target: { value: '3. Known gaps' } })
    expect(screen.queryByText('CS-RESP-005')).not.toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('1 of 2')
  })

  it('opens a row to show its other columns, and closes it', async () => {
    render(<CheckTable />)
    const id = await screen.findByRole('button', { name: 'CS-RESP-005' })
    expect(screen.queryByText('What it means')).not.toBeInTheDocument()
    fireEvent.click(id)
    expect(id).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('What it means')).toBeInTheDocument()
    expect(screen.getByText('required').tagName).toBe('CODE')
    fireEvent.click(id)
    expect(screen.queryByText('What it means')).not.toBeInTheDocument()
  })

  it('says when the table cannot be loaded', async () => {
    b.getCheckTable.mockRejectedValue(new Error('boom'))
    render(<CheckTable />)
    expect(await screen.findByText('Failed to load the table of checks.')).toBeInTheDocument()
  })
})
