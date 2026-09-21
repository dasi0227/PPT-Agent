const RESIZE_HANDLE_PX = 3;
const CENTER_MIN_PX = 600;
const SIDEBAR_OPEN_BUTTON_PX = 36;
const RIGHT_MIN_PX = 360;

export function workspacePanelLayout(width: number, leftHidden: boolean, rightHidden: boolean) {
  const workspaceWidth = Number.isFinite(width) && width > 0 ? width : 1440;
  const sizes = () => {
    const availableWidth = Math.max(1, workspaceWidth - RESIZE_HANDLE_PX * (Number(!leftHidden) + Number(!rightHidden)));
    const centerMinPx = CENTER_MIN_PX + SIDEBAR_OPEN_BUTTON_PX * (Number(leftHidden) + Number(rightHidden));
    const leftMin = leftHidden ? 0 : 16;
    const rightMin = rightHidden ? 0 : Math.max(20, RIGHT_MIN_PX / availableWidth * 100);
    const centerMin = centerMinPx / availableWidth * 100;
    return { leftMin, rightMin, centerMin };
  };

  let bounds = sizes();
  // Keep the conversation available first; automatic hiding never changes user preferences.
  if (bounds.leftMin + bounds.centerMin + bounds.rightMin > 100 && !leftHidden) {
    leftHidden = true;
    bounds = sizes();
  }
  if (bounds.centerMin + bounds.rightMin > 100 && !rightHidden) {
    rightHidden = true;
    bounds = sizes();
  }

  const centerMin = Math.min(100, bounds.centerMin);
  const rightDefault = rightHidden ? 0 : Math.min(Math.max(28, bounds.rightMin), 100 - centerMin - bounds.leftMin);
  const leftDefault = leftHidden ? 0 : Math.min(20, 100 - centerMin - rightDefault);
  return {
    leftHidden,
    rightHidden,
    leftMin: bounds.leftMin,
    rightMin: bounds.rightMin,
    centerMin,
    leftDefault,
    rightDefault,
    centerDefault: 100 - leftDefault - rightDefault,
  };
}
