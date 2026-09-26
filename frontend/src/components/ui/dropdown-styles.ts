// Shared by value selectors and action menus; dialog surfaces stay independent.
export const dropdownSurfaceClassName =
  'z-50 max-w-[calc(100vw-24px)] rounded-lg border border-border bg-surface p-1 text-text-800 shadow-overlay outline-none';

export const dropdownItemClassName =
  'relative flex w-full min-w-0 cursor-pointer select-none items-center gap-2 rounded-md px-2.5 py-2 text-xs outline-none transition-colors data-[disabled]:pointer-events-none data-[disabled]:opacity-45';

export const dropdownItemHighlightClassName =
  'ui-interactive text-text-600';
