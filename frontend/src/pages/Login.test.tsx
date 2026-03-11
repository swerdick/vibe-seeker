import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'
import Login from './Login'

function renderLogin(initialEntries: string[] = ['/']) {
  return render(
    <MemoryRouter initialEntries={initialEntries}>
      <Login />
    </MemoryRouter>,
  )
}

describe('Login', () => {
  it('renders the login page heading', () => {
    renderLogin()
    expect(screen.getByRole('heading', { name: /vibe seeker/i })).toBeInTheDocument()
  })

  it('renders a Spotify login link pointing to the API', () => {
    renderLogin()
    const link = screen.getByRole('link', { name: /log in with spotify/i })
    expect(link).toHaveAttribute('href', expect.stringContaining('/api/auth/login'))
  })

  it('shows an error message when error param is present', () => {
    renderLogin(['/?error=access_denied'])
    expect(screen.getByText(/login failed: access_denied/i)).toBeInTheDocument()
  })

  it('does not show an error message when no error param', () => {
    renderLogin()
    expect(screen.queryByText(/login failed/i)).not.toBeInTheDocument()
  })
})
