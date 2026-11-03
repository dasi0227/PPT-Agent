import '@testing-library/jest-dom';

// Mock scrollIntoView
window.HTMLElement.prototype.scrollIntoView = function() {};

// Mock localStorage
const localStorageMock = (function() {
  let store: Record<string, string> = {};
  return {
    getItem: function(key: string) { return store[key] || null; },
    setItem: function(key: string, value: string) { store[key] = value.toString(); },
    removeItem: function(key: string) { delete store[key]; },
    clear: function() { store = {}; }
  };
})();
Object.defineProperty(window, 'localStorage', { value: localStorageMock });
Object.defineProperty(window, 'sessionStorage', { value: localStorageMock });

// Retained tests mock their API boundaries; never reach a running service.
globalThis.fetch = async (input: RequestInfo | URL) => {
  throw new Error(`Unexpected test fetch: ${input.toString()}`);
};
