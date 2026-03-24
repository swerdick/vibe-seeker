// eslint-disable-next-line @typescript-eslint/no-explicit-any
export default function MapGL(props: any) {
  return <div data-testid="map">{props?.children}</div>;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function Marker(props: any) {
  return (
    <div
      data-testid="marker"
      onClick={() =>
        props?.onClick?.({ originalEvent: { stopPropagation: () => {} } })
      }
    >
      {props?.children}
    </div>
  );
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function Popup(props: any) {
  return <div data-testid="popup">{props?.children}</div>;
}
