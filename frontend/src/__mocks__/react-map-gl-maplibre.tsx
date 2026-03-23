import React from "react";

export default function Map({
  children,
}: {
  children?: React.ReactNode;
  [key: string]: unknown;
}) {
  return <div data-testid="map">{children}</div>;
}

export function Marker({
  children,
  onClick,
}: {
  children?: React.ReactNode;
  onClick?: (e: { originalEvent: { stopPropagation: () => void } }) => void;
  [key: string]: unknown;
}) {
  return (
    <div
      data-testid="marker"
      onClick={() =>
        onClick?.({ originalEvent: { stopPropagation: () => {} } })
      }
    >
      {children}
    </div>
  );
}

export function Popup({
  children,
}: {
  children?: React.ReactNode;
  [key: string]: unknown;
}) {
  return <div data-testid="popup">{children}</div>;
}
