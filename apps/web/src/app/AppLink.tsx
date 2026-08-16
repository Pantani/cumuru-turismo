import type { AnchorHTMLAttributes, MouseEvent, ReactNode } from "react";

import type { AppPath } from "./routes";

interface AppLinkProps
  extends Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href"> {
  children: ReactNode;
  href: AppPath;
  navigate: (path: AppPath) => void;
}

/**
 * A modified or non-primary click must keep the browser's own behaviour, so
 * "open in a new tab" and friends keep working on in-app links.
 */
function hasModifier(event: MouseEvent<HTMLAnchorElement>) {
  return event.metaKey || event.ctrlKey || event.shiftKey || event.altKey;
}

function isPlainLeftClick(event: MouseEvent<HTMLAnchorElement>) {
  if (event.defaultPrevented || event.button !== 0) {
    return false;
  }
  return !hasModifier(event);
}

export function AppLink({
  children,
  href,
  navigate,
  onClick,
  ...anchorProps
}: AppLinkProps) {
  const handleClick = (event: MouseEvent<HTMLAnchorElement>) => {
    onClick?.(event);
    if (!isPlainLeftClick(event)) {
      return;
    }
    event.preventDefault();
    navigate(href);
  };

  return (
    <a {...anchorProps} href={href} onClick={handleClick}>
      {children}
    </a>
  );
}
