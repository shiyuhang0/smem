import '@testing-library/jest-dom'

class TestIntersectionObserver {
  observe() {}
  disconnect() {}
  unobserve() {}
}

Object.defineProperty(globalThis, 'IntersectionObserver', {
  writable: true,
  configurable: true,
  value: TestIntersectionObserver,
})

Object.defineProperty(Element.prototype, 'hasPointerCapture', {
  writable: true,
  configurable: true,
  value: () => false,
})

Object.defineProperty(Element.prototype, 'setPointerCapture', {
  writable: true,
  configurable: true,
  value: () => {},
})

Object.defineProperty(Element.prototype, 'releasePointerCapture', {
  writable: true,
  configurable: true,
  value: () => {},
})

Object.defineProperty(Element.prototype, 'scrollIntoView', {
  writable: true,
  configurable: true,
  value: () => {},
})
