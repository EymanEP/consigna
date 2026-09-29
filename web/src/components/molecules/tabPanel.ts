/** Props for the panel shown by Tabs; ids match the tab ids. */
export function tabPanelProps(idPrefix: string, key: string) {
  return {
    role: 'tabpanel',
    id: `${idPrefix}-panel-${key}`,
    'aria-labelledby': `${idPrefix}-tab-${key}`,
    tabIndex: 0,
  } as const;
}
