import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { ImportReportDialog } from './ImportReportDialog'

const request = { operationId: '', summary: 'a', method: 'GET', url: 'http://x', savedPath: 'a.yaml' }

describe('ImportReportDialog', () => {
  it('lists every warning and closes', () => {
    const onClose = vi.fn()
    render(<ImportReportDialog onClose={onClose} report={{
      collectionName: 'Shop', imported: [request] as never, skipped: 0,
      warnings: ['Create user: a Postman test script was not imported', 'Login: something else'],
    }} />)
    expect(screen.getByRole('dialog', { name: 'Imported 1 request from Shop' })).toBeInTheDocument()
    expect(screen.getAllByRole('listitem').map(li => li.textContent)).toEqual([
      'Create user: a Postman test script was not imported', 'Login: something else',
    ])
    expect(screen.queryByText(/could not be written/)).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(onClose).toHaveBeenCalled()
  })

  it('says what could not be written, and Escape closes it', () => {
    const onClose = vi.fn()
    render(<ImportReportDialog onClose={onClose} report={{ collectionName: 'Shop', imported: [], skipped: 2 }} />)
    expect(screen.getByText('2 requests could not be written.')).toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })
})
