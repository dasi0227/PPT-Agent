export const RESOURCE_EDIT_TOOLS = [
  'edit_manifest', 'edit_design', 'edit_spec',
  'edit_outline', 'edit_html',
] as const;

export function isResourceEditTool(name: string): boolean {
  return (RESOURCE_EDIT_TOOLS as readonly string[]).includes(name);
}
