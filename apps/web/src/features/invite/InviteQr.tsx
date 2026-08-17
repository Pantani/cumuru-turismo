import QRCode from "qrcode";
import { useEffect, useRef, type ReactNode } from "react";

export interface QrPresentation {
  description?: ReactNode;
  discardLabel?: string;
  label?: string;
  title?: string;
}

interface InviteQrProps {
  onDiscard: () => void;
  onOpenHere?: () => void;
  presentation?: QrPresentation;
  url: string;
}

const defaultPresentation = {
  description: (
    <p>
      Mostre o código diretamente ao visitante. A URL sensível não é exibida nem
      copiada para o histórico.
    </p>
  ),
  discardLabel: "Ocultar convite",
  label: "Código QR do convite",
  title: "Convite pronto para leitura local",
} satisfies Required<QrPresentation>;

function presentationOf(presentation: QrPresentation | undefined) {
  return { ...defaultPresentation, ...presentation };
}

/**
 * The code is drawn locally, on a canvas, with no request to any third party
 * and under the existing CSP. The URL itself is never rendered as text and
 * never copied to the address bar, so it reaches neither history nor a
 * screenshot of the browser chrome.
 */
export function InviteQr({
  onDiscard,
  onOpenHere,
  presentation,
  url,
}: InviteQrProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const view = presentationOf(presentation);

  useEffect(() => {
    if (canvasRef.current !== null) {
      void QRCode.toCanvas(canvasRef.current, url, {
        errorCorrectionLevel: "M",
        margin: 2,
        width: 256,
      });
    }
  }, [url]);

  return (
    <div className="qr-panel" role="group" aria-labelledby="qr-title">
      <h3 id="qr-title">{view.title}</h3>
      <canvas ref={canvasRef} aria-label={view.label} role="img" />
      {view.description}
      {onOpenHere === undefined ? null : (
        <button type="button" onClick={onOpenHere}>
          Abrir registro neste navegador
        </button>
      )}
      <button type="button" onClick={onDiscard}>
        {view.discardLabel}
      </button>
    </div>
  );
}
