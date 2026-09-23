/** Vector version of the D-shaped smile mark, tinted to match its button. */
export function HomeLogo() {
  return (
    <svg
      aria-hidden="true"
      focusable="false"
      viewBox="0 0 24 24"
      className="h-4 w-4 shrink-0"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M6 2h7c5.5 0 9 4.2 9 10s-3.5 10-9 10H6a4 4 0 0 1-4-4V6a4 4 0 0 1 4-4Z" />
      <circle cx="8.6" cy="10.6" r="1.2" fill="currentColor" stroke="none" />
      <circle cx="15.4" cy="10.6" r="1.2" fill="currentColor" stroke="none" />
      <path d="M10 14.9c.9 1.6 3.1 1.6 4 0" strokeWidth="1.25" />
    </svg>
  );
}
