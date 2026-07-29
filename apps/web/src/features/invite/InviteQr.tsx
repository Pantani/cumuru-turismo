import QRCode from "qrcode";
import { useEffect, useRef } from "react";

interface InviteQrProps {
  onDiscard: () => void;
  url: string;
}

export function InviteQr({ onDiscard, url }: InviteQrProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);

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
      <h3 id="qr-title">Convite pronto para leitura local</h3>
      <canvas ref={canvasRef} aria-label="Código QR do convite" role="img" />
      <p>
        Mostre o código diretamente ao visitante. A URL sensível não é exibida
        nem copiada para o histórico.
      </p>
      <button type="button" onClick={onDiscard}>
        Ocultar convite
      </button>
    </div>
  );
}
