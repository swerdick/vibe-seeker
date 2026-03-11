import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it } from 'vitest'
import Home from './Home'

afterEach(() => {
  localStorage.clear()
})

function renderHome() {
  return render(
    <MemoryRouter initialEntries={['/home']}>
      <Routes>
        <Route path="/" element={<p>login page</p>} />
        <Route path="/home" element={<Home />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('Home', () => {
  it('renders the home page when token is present', () => {
    localStorage.setItem('token', 'test-jwt')
    renderHome()
    expect(screen.getByText(/you are logged in/i)).toBeInTheDocument()
  })

  it('does not render content when no token is present', () => {
    renderHome()
    expect(screen.queryByText(/you are logged in/i)).not.toBeInTheDocument()
  })

  it('clears the token on logout', async () => {
    localStorage.setItem('token', 'test-jwt')
    renderHome()

    await userEvent.click(screen.getByRole('button', { name: /log out/i }))
    expect(localStorage.getItem('token')).toBeNull()
    expect(screen.getByText('login page')).toBeInTheDocument()
  })
})
