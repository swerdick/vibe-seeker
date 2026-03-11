import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it } from 'vitest'
import Callback from './Callback'

afterEach(() => {
  localStorage.clear()
})

function renderCallback(initialEntries: string[]) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <Callback />
    </MemoryRouter>,
  )
}

describe('Callback', () => {
  it('stores the token in localStorage when present', () => {
    renderCallback(['/callback?token=test-jwt-token'])
    expect(localStorage.getItem('token')).toBe('test-jwt-token')
  })

  it('does not store anything when token is missing', () => {
    renderCallback(['/callback'])
    expect(localStorage.getItem('token')).toBeNull()
  })
})
