import { render, screen } from '@testing-library/react'

import { App } from './app'

test('renders dashboard shell title', () => {
  render(<App />)
  expect(screen.getByText('Memory Console')).toBeInTheDocument()
})
